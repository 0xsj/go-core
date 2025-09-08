// internal/lib/queue/factory.go
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/0xsj/go-core/internal/config"
	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/redis/go-redis/v9"
)

func CreateManager(ctx context.Context, cfg *config.Config, appLogger logger.Logger) QueueManager {
	if !cfg.Queue.Enabled {
		appLogger.Info("Queue system disabled in configuration")
		return NewNoOpManager()
	}

	// Initialize Redis client
	redisClient, err := initializeRedis(cfg, appLogger)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize Redis for queue: %v", err))
	}

	// Create the queue manager
	managerConfig := buildManagerConfig(cfg)
	manager := NewRedisQueueManager(redisClient, managerConfig, appLogger)

	// Register default queues
	registerDefaultQueues(manager, cfg, appLogger)

	appLogger.Info("Queue system initialized",
		logger.Int("queue_count", len(cfg.Queue.Queues)),
		logger.Bool("auto_scale", cfg.Queue.Global.AutoScale),
		logger.Bool("dlq_enabled", cfg.Queue.Global.DLQEnabled),
	)

	return manager
}

func initializeRedis(cfg *config.Config, appLogger logger.Logger) (*redis.Client, error) {
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
	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(testCtx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	appLogger.Info("Redis connection established for queue system")
	return client, nil
}

func buildManagerConfig(cfg *config.Config) *ManagerConfig {
	// Parse durations with defaults if parsing fails
	defaultTimeout, _ := time.ParseDuration(cfg.Queue.Global.DefaultTimeout)
	if defaultTimeout == 0 {
		defaultTimeout = 30 * time.Minute
	}

	retryInitialDelay, _ := time.ParseDuration(cfg.Queue.Global.RetryInitialDelay)
	if retryInitialDelay == 0 {
		retryInitialDelay = 1 * time.Second
	}

	retryMaxDelay, _ := time.ParseDuration(cfg.Queue.Global.RetryMaxDelay)
	if retryMaxDelay == 0 {
		retryMaxDelay = 1 * time.Hour
	}

	shutdownTimeout, _ := time.ParseDuration(cfg.Queue.Global.ShutdownTimeout)
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	healthCheckInterval, _ := time.ParseDuration(cfg.Queue.Global.HealthCheckInterval)
	if healthCheckInterval == 0 {
		healthCheckInterval = 10 * time.Second
	}

	metricsInterval, _ := time.ParseDuration(cfg.Queue.Global.MetricsInterval)
	if metricsInterval == 0 {
		metricsInterval = 10 * time.Second
	}

	scheduledPollInterval, _ := time.ParseDuration(cfg.Queue.Global.ScheduledPollInterval)
	if scheduledPollInterval == 0 {
		scheduledPollInterval = 10 * time.Second
	}

	scaleInterval, _ := time.ParseDuration(cfg.Queue.Global.ScaleInterval)
	if scaleInterval == 0 {
		scaleInterval = 30 * time.Second
	}

	circuitBreakerTimeout, _ := time.ParseDuration(cfg.Queue.Global.CircuitBreakerTimeout)
	if circuitBreakerTimeout == 0 {
		circuitBreakerTimeout = 60 * time.Second
	}

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
		AutoScale:               cfg.Queue.Global.AutoScale,
		ScaleUpThreshold:        cfg.Queue.Global.ScaleUpThreshold,
		ScaleDownThreshold:      cfg.Queue.Global.ScaleDownThreshold,
		ScaleInterval:           scaleInterval,
		CircuitBreakerEnabled:   cfg.Queue.Global.CircuitBreakerEnabled,
		CircuitBreakerThreshold: cfg.Queue.Global.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   circuitBreakerTimeout,
	}
}

func registerDefaultQueues(manager QueueManager, cfg *config.Config, appLogger logger.Logger) {
	// If no queues configured, use defaults
	queues := cfg.Queue.Queues
	if len(queues) == 0 {
		queues = config.GetDefaultQueues()
	}

	for name, queueDef := range queues {
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
			DLQRetentionDays: cfg.Queue.Global.DLQRetentionDays,
			EnableScheduled:  queueDef.EnableScheduled,
			EnablePriority:   true,
			DefaultPriority:  parsePriority(queueDef.Priority),
			WorkerCount:      queueDef.Workers,
			MinWorkers:       queueDef.MinWorkers,
			MaxWorkers:       queueDef.MaxWorkers,
			AutoScale:        queueDef.AutoScale,
			PrefetchCount:    queueDef.PrefetchCount,
		}

		if err := manager.RegisterQueue(name, queueConfig); err != nil {
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

func parsePriority(priority string) Priority {
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
