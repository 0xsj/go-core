// internal/lib/queue/manager.go
package queue

import (
	"context"
	"fmt"
	"sync"

	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/redis/go-redis/v9"
)

// RedisQueueManager implements QueueManager using Redis
type RedisQueueManager struct {
	redis    *redis.Client
	config   *ManagerConfig
	logger   logger.Logger
	queues   map[string]Queue
	handlers map[string]JobHandler
	workers  map[string]*WorkerPool
	mu       sync.RWMutex
}

// NewRedisQueueManager creates a new Redis-based queue manager
func NewRedisQueueManager(client *redis.Client, config *ManagerConfig, logger logger.Logger) QueueManager {
	return &RedisQueueManager{
		redis:    client,
		config:   config,
		logger:   logger,
		queues:   make(map[string]Queue),
		handlers: make(map[string]JobHandler),
		workers:  make(map[string]*WorkerPool),
	}
}

// RegisterQueue registers a new queue
func (m *RedisQueueManager) RegisterQueue(name string, config QueueConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create the Redis queue
	queue, err := NewRedisQueue(name, config, m.redis, m.logger)
	if err != nil {
		return fmt.Errorf("failed to create queue %s: %w", name, err)
	}

	m.queues[name] = queue
	m.logger.Info("Queue registered",
		logger.String("queue", name),
		logger.Int("workers", config.WorkerCount),
	)
	return nil
}

// GetQueue returns a queue by name
func (m *RedisQueueManager) GetQueue(name string) (Queue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queue, exists := m.queues[name]
	if !exists {
		return nil, fmt.Errorf("queue %s not found", name)
	}
	return queue, nil
}

// ListQueues returns all queue names
func (m *RedisQueueManager) ListQueues() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.queues))
	for name := range m.queues {
		names = append(names, name)
	}
	return names
}

// Enqueue adds a job to the appropriate queue
func (m *RedisQueueManager) Enqueue(ctx context.Context, job *Job) error {
	// Default to "default" queue if not specified
	queueName := job.Queue
	if queueName == "" {
		queueName = "default"
	}

	queue, err := m.GetQueue(queueName)
	if err != nil {
		return fmt.Errorf("failed to get queue %s: %w", queueName, err)
	}

	return queue.Enqueue(ctx, job)
}

// EnqueueWithOptions adds a job with options
func (m *RedisQueueManager) EnqueueWithOptions(ctx context.Context, queueName, jobType string, payload []byte, opts JobOptions) error {
	if queueName == "" {
		queueName = "default"
	}

	queue, err := m.GetQueue(queueName)
	if err != nil {
		return fmt.Errorf("failed to get queue %s: %w", queueName, err)
	}

	return queue.EnqueueWithOptions(ctx, jobType, payload, opts)
}

// RegisterHandler registers a job handler
func (m *RedisQueueManager) RegisterHandler(handler JobHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jobType := handler.JobType()
	if _, exists := m.handlers[jobType]; exists {
		return fmt.Errorf("handler for job type %s already registered", jobType)
	}

	m.handlers[jobType] = handler
	return nil
}

// GetHandler returns a handler by job type
func (m *RedisQueueManager) GetHandler(jobType string) (JobHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	handler, exists := m.handlers[jobType]
	return handler, exists
}

// PauseQueue pauses a queue
func (m *RedisQueueManager) PauseQueue(ctx context.Context, name string) error {
	queue, err := m.GetQueue(name)
	if err != nil {
		return err
	}
	return queue.Pause(ctx)
}

// ResumeQueue resumes a queue
func (m *RedisQueueManager) ResumeQueue(ctx context.Context, name string) error {
	queue, err := m.GetQueue(name)
	if err != nil {
		return err
	}
	return queue.Resume(ctx)
}

// DrainQueue gracefully drains a queue
func (m *RedisQueueManager) DrainQueue(ctx context.Context, name string) error {
	queue, err := m.GetQueue(name)
	if err != nil {
		return err
	}
	return queue.Clear(ctx)
}

// ScaleWorkers adjusts worker count
func (m *RedisQueueManager) ScaleWorkers(queueName string, count int) error {
	// TODO: Implement worker scaling
	return nil
}

// GetWorkerPoolHealth returns worker pool health
func (m *RedisQueueManager) GetWorkerPoolHealth(queueName string) WorkerPoolHealth {
	// TODO: Implement worker pool health
	return WorkerPoolHealth{}
}

// Start starts the queue manager
func (m *RedisQueueManager) Start(ctx context.Context) error {
	m.logger.Info("Queue manager started")
	// TODO: Start worker pools for each queue
	return nil
}

// Stop stops the queue manager
func (m *RedisQueueManager) Stop(ctx context.Context) error {
	m.logger.Info("Queue manager stopped")
	// TODO: Stop all worker pools
	return nil
}

// GetStats returns overall stats
func (m *RedisQueueManager) GetStats() QueueStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := QueueStats{
		Name: "overall",
	}

	// Aggregate stats from all queues
	for name, queue := range m.queues {
		if redisQueue, ok := queue.(*RedisQueue); ok {
			queueStats, err := redisQueue.GetStats(context.Background())
			if err != nil {
				m.logger.Error("Failed to get stats for queue",
					logger.String("queue", name),
					logger.Err(err),
				)
				continue
			}

			stats.QueueDepth += queueStats.QueueDepth
			stats.ScheduledCount += queueStats.ScheduledCount
			stats.ProcessingCount += queueStats.ProcessingCount
			stats.DLQCount += queueStats.DLQCount
			// Add other stats as needed
		}
	}

	return stats
}

// GetQueueStats returns stats for a specific queue
func (m *RedisQueueManager) GetQueueStats(queueName string) QueueStats {
	queue, err := m.GetQueue(queueName)
	if err != nil {
		return QueueStats{Name: queueName}
	}

	if redisQueue, ok := queue.(*RedisQueue); ok {
		stats, err := redisQueue.GetStats(context.Background())
		if err != nil {
			m.logger.Error("Failed to get queue stats",
				logger.String("queue", queueName),
				logger.Err(err),
			)
			return QueueStats{Name: queueName}
		}
		return stats
	}

	return QueueStats{Name: queueName}
}
