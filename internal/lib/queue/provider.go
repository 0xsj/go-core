// internal/lib/queue/provider.go
package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/container"
	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/redis/go-redis/v9"
)

// Provider creates and manages the queue system
type Provider struct {
	Container   *container.Container
	manager     QueueManager
	redisClient *redis.Client
	once        sync.Once
	startOnce   sync.Once
	stopOnce    sync.Once
}

// NewProvider creates a new queue provider
func NewProvider(c *container.Container) *Provider {
	return &Provider{
		Container: c,
	}
}

// Provide returns the queue manager (implements container.Provider interface)
func (p *Provider) Provide() QueueManager {
	p.once.Do(func() {
		// Resolve dependencies
		cfg := container.Resolve[*config.Config](p.Container)
		appLogger := container.Resolve[logger.Logger](p.Container)

		if !cfg.Queue.Enabled {
			appLogger.Info("Queue system disabled in configuration")
			// Return a no-op manager if queues are disabled
			p.manager = NewNoOpManager()
			return
		}

		// Initialize Redis client
		redisClient, err := p.initializeRedis(cfg, appLogger)
		if err != nil {
			panic(fmt.Sprintf("Failed to initialize Redis for queue: %v", err))
		}
		p.redisClient = redisClient

		// Create the queue manager
		managerConfig := p.buildManagerConfig(cfg)
		p.manager = NewRedisQueueManager(redisClient, managerConfig, appLogger)

		// Register default queues
		p.registerDefaultQueues(cfg, appLogger)

		appLogger.Info("Queue system initialized",
			logger.Int("queue_count", len(cfg.Queue.Queues)),
			logger.Bool("auto_scale", cfg.Queue.Global.AutoScale),
			logger.Bool("dlq_enabled", cfg.Queue.Global.DLQEnabled),
		)
	})

	return p.manager
}

// Start implements container.Startable
func (p *Provider) Start(ctx context.Context) error {
	p.startOnce.Do(func() {
		if p.manager == nil {
			return
		}

		// Don't start if it's a no-op manager
		if _, isNoOp := p.manager.(*NoOpManager); isNoOp {
			return
		}

		// Start the queue manager
		if err := p.manager.Start(ctx); err != nil {
			cfg := container.Resolve[*config.Config](p.Container)
			appLogger := container.Resolve[logger.Logger](p.Container)
			appLogger.Error("Failed to start queue manager", logger.Err(err))

			// Don't panic here, just log the error
			// This allows the app to start even if queues fail
			if cfg.Queue.Enabled {
				panic(fmt.Sprintf("Queue system is enabled but failed to start: %v", err))
			}
		}
	})

	return nil
}

// Stop implements container.Stoppable
func (p *Provider) Stop(ctx context.Context) error {
	var finalErr error

	p.stopOnce.Do(func() {
		if p.manager == nil {
			return
		}

		// Don't stop if it's a no-op manager
		if _, isNoOp := p.manager.(*NoOpManager); isNoOp {
			return
		}

		appLogger := container.Resolve[logger.Logger](p.Container)
		appLogger.Info("Stopping queue system...")

		// Stop the queue manager
		if err := p.manager.Stop(ctx); err != nil {
			appLogger.Error("Error stopping queue manager", logger.Err(err))
			finalErr = err
		}

		// Close Redis connection
		if p.redisClient != nil {
			if err := p.redisClient.Close(); err != nil {
				appLogger.Error("Error closing Redis connection", logger.Err(err))
				if finalErr == nil {
					finalErr = err
				}
			}
		}

		appLogger.Info("Queue system stopped")
	})

	return finalErr
}

// initializeRedis creates the Redis client for the queue system
func (p *Provider) initializeRedis(cfg *config.Config, appLogger logger.Logger) (*redis.Client, error) {
	var redisConfig config.RedisConfig

	if cfg.Queue.Redis.UseMainRedis {
		// Use the main Redis configuration
		redisConfig = cfg.Redis
		appLogger.Info("Queue using main Redis instance")
	} else {
		// Use separate Redis configuration for queues
		redisConfig = config.RedisConfig{
			Host:     cfg.Queue.Redis.Host,
			Port:     cfg.Queue.Redis.Port,
			Password: cfg.Queue.Redis.Password,
			DB:       cfg.Queue.Redis.DB,
		}
		appLogger.Info("Queue using separate Redis instance",
			logger.String("host", redisConfig.Host),
			logger.Int("port", redisConfig.Port),
			logger.Int("db", redisConfig.DB),
		)
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", redisConfig.Host, redisConfig.Port),
		Password:     redisConfig.Password,
		DB:           redisConfig.DB,
		MaxRetries:   cfg.Queue.Redis.MaxRetries,
		PoolSize:     cfg.Queue.Redis.PoolSize,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	appLogger.Info("Redis connection established for queue system")
	return client, nil
}

// buildManagerConfig converts app config to queue manager config
func (p *Provider) buildManagerConfig(cfg *config.Config) *ManagerConfig {
	// Parse durations
	defaultTimeout, _ := time.ParseDuration(cfg.Queue.Global.DefaultTimeout)
	retryInitialDelay, _ := time.ParseDuration(cfg.Queue.Global.RetryInitialDelay)
	retryMaxDelay, _ := time.ParseDuration(cfg.Queue.Global.RetryMaxDelay)
	shutdownTimeout, _ := time.ParseDuration(cfg.Queue.Global.ShutdownTimeout)
	healthCheckInterval, _ := time.ParseDuration(cfg.Queue.Global.HealthCheckInterval)
	metricsInterval, _ := time.ParseDuration(cfg.Queue.Global.MetricsInterval)
	scheduledPollInterval, _ := time.ParseDuration(cfg.Queue.Global.ScheduledPollInterval)
	scaleInterval, _ := time.ParseDuration(cfg.Queue.Global.ScaleInterval)
	circuitBreakerTimeout, _ := time.ParseDuration(cfg.Queue.Global.CircuitBreakerTimeout)

	return &ManagerConfig{
		DefaultTimeout:          defaultTimeout,
		MaxRetries:              cfg.Queue.Global.MaxRetries,
		RetryInitialDelay:       retryInitialDelay,
		RetryMaxDelay:           retryMaxDelay,
		RetryMultiplier:         cfg.Queue.Global.RetryMultiplier,
		ShutdownTimeout:         shutdownTimeout,
		HealthCheckInterval:     healthCheckInterval,
		MetricsEnabled:          cfg.Queue.Global.MetricsEnabled,
		MetricsInterval:         metricsInterval,
		DLQEnabled:              cfg.Queue.Global.DLQEnabled,
		DLQRetentionDays:        cfg.Queue.Global.DLQRetentionDays,
		ScheduledEnabled:        cfg.Queue.Global.ScheduledEnabled,
		ScheduledPollInterval:   scheduledPollInterval,
		ScaleUpThreshold:        cfg.Queue.Global.ScaleUpThreshold,
		ScaleDownThreshold:      cfg.Queue.Global.ScaleDownThreshold,
		ScaleInterval:           scaleInterval,
		CircuitBreakerEnabled:   cfg.Queue.Global.CircuitBreakerEnabled,
		CircuitBreakerThreshold: cfg.Queue.Global.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   circuitBreakerTimeout,
	}
}

// registerDefaultQueues registers the configured queues
func (p *Provider) registerDefaultQueues(cfg *config.Config, appLogger logger.Logger) {
	for name, queueDef := range cfg.Queue.Queues {
		if !queueDef.Enabled {
			appLogger.Info("Queue disabled, skipping registration",
				logger.String("queue", name),
			)
			continue
		}

		queueConfig := QueueConfig{
			Name:             name,
			StreamKey:        fmt.Sprintf("queue:stream:%s", name),
			ConsumerGroup:    "workers",
			MaxLength:        queueDef.MaxLength,
			BlockingTimeout:  5 * time.Second,
			ClaimMinIdleTime: 30 * time.Second,
			RetryPolicy:      DefaultRetryPolicy(),
			EnableDLQ:        queueDef.EnableDLQ,
			DLQMaxSize:       queueDef.DLQMaxSize,
			EnableScheduled:  queueDef.EnableScheduled,
			EnablePriority:   true,
			DefaultPriority:  p.parsePriority(queueDef.Priority),
			WorkerCount:      queueDef.Workers,
			MinWorkers:       queueDef.MinWorkers,
			MaxWorkers:       queueDef.MaxWorkers,
			AutoScale:        queueDef.AutoScale,
			PrefetchCount:    queueDef.PrefetchCount,
		}

		if err := p.manager.RegisterQueue(name, queueConfig); err != nil {
			appLogger.Error("Failed to register queue",
				logger.String("queue", name),
				logger.Err(err),
			)
		} else {
			appLogger.Info("Queue registered",
				logger.String("queue", name),
				logger.Int("workers", queueDef.Workers),
				logger.Bool("auto_scale", queueDef.AutoScale),
			)
		}
	}
}

// parsePriority converts string priority to Priority type
func (p *Provider) parsePriority(priority string) Priority {
	switch priority {
	case "critical":
		return PriorityCritical
	case "high":
		return PriorityHigh
	case "normal":
		return PriorityNormal
	case "low":
		return PriorityLow
	case "batch":
		return PriorityBatch
	default:
		return PriorityNormal
	}
}
