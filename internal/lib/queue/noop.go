// internal/lib/queue/noop.go
package queue

import (
	"context"
	"time"
)

// NoOpManager is a no-operation queue manager for when queues are disabled
type NoOpManager struct{}

// NewNoOpManager creates a new no-op queue manager
func NewNoOpManager() QueueManager {
	return &NoOpManager{}
}

// RegisterQueue does nothing
func (m *NoOpManager) RegisterQueue(name string, config QueueConfig) error {
	return nil
}

// GetQueue returns a no-op queue
func (m *NoOpManager) GetQueue(name string) (Queue, error) {
	return &NoOpQueue{}, nil
}

// ListQueues returns empty list
func (m *NoOpManager) ListQueues() []string {
	return []string{}
}

// Enqueue does nothing
func (m *NoOpManager) Enqueue(ctx context.Context, job *Job) error {
	return nil
}

// EnqueueWithOptions does nothing
func (m *NoOpManager) EnqueueWithOptions(ctx context.Context, queueName, jobType string, payload []byte, opts JobOptions) error {
	return nil
}

// RegisterHandler does nothing
func (m *NoOpManager) RegisterHandler(handler JobHandler) error {
	return nil
}

// GetHandler returns nil
func (m *NoOpManager) GetHandler(jobType string) (JobHandler, bool) {
	return nil, false
}

// PauseQueue does nothing
func (m *NoOpManager) PauseQueue(ctx context.Context, name string) error {
	return nil
}

// ResumeQueue does nothing
func (m *NoOpManager) ResumeQueue(ctx context.Context, name string) error {
	return nil
}

// DrainQueue does nothing
func (m *NoOpManager) DrainQueue(ctx context.Context, name string) error {
	return nil
}

// ScaleWorkers does nothing
func (m *NoOpManager) ScaleWorkers(queueName string, count int) error {
	return nil
}

// GetWorkerPoolHealth returns empty health
func (m *NoOpManager) GetWorkerPoolHealth(queueName string) WorkerPoolHealth {
	return WorkerPoolHealth{}
}

// Start does nothing
func (m *NoOpManager) Start(ctx context.Context) error {
	return nil
}

// Stop does nothing
func (m *NoOpManager) Stop(ctx context.Context) error {
	return nil
}

// GetStats returns empty stats
func (m *NoOpManager) GetStats() QueueStats {
	return QueueStats{}
}

// GetQueueStats returns empty stats
func (m *NoOpManager) GetQueueStats(queueName string) QueueStats {
	return QueueStats{}
}

// NoOpQueue is a no-operation queue implementation
type NoOpQueue struct{}

// Enqueue does nothing
func (q *NoOpQueue) Enqueue(ctx context.Context, job *Job) error {
	return nil
}

// EnqueueWithOptions does nothing
func (q *NoOpQueue) EnqueueWithOptions(ctx context.Context, jobType string, payload []byte, opts JobOptions) error {
	return nil
}

// Dequeue returns nil
func (q *NoOpQueue) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	return nil, nil
}

// Ack does nothing
func (q *NoOpQueue) Ack(ctx context.Context, jobID string) error {
	return nil
}

// Nack does nothing
func (q *NoOpQueue) Nack(ctx context.Context, jobID string, requeue bool) error {
	return nil
}

// GetJob returns nil
func (q *NoOpQueue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	return nil, nil
}

// PeekNext returns nil
func (q *NoOpQueue) PeekNext(ctx context.Context) (*Job, error) {
	return nil, nil
}

// Len returns 0
func (q *NoOpQueue) Len(ctx context.Context) (int64, error) {
	return 0, nil
}

// Clear does nothing
func (q *NoOpQueue) Clear(ctx context.Context) error {
	return nil
}

// Pause does nothing
func (q *NoOpQueue) Pause(ctx context.Context) error {
	return nil
}

// Resume does nothing
func (q *NoOpQueue) Resume(ctx context.Context) error {
	return nil
}

// MoveToDLQ does nothing
func (q *NoOpQueue) MoveToDLQ(ctx context.Context, job *Job, reason string) error {
	return nil
}

// GetDLQ returns empty list
func (q *NoOpQueue) GetDLQ(ctx context.Context, limit int) ([]*Job, error) {
	return []*Job{}, nil
}

// ReprocessDLQ does nothing
func (q *NoOpQueue) ReprocessDLQ(ctx context.Context, jobID string) error {
	return nil
}

// ClearDLQ does nothing
func (q *NoOpQueue) ClearDLQ(ctx context.Context) error {
	return nil
}

// EnqueueScheduled does nothing
func (q *NoOpQueue) EnqueueScheduled(ctx context.Context, job *Job, scheduledAt time.Time) error {
	return nil
}

// RescheduleJob does nothing
func (q *NoOpQueue) RescheduleJob(ctx context.Context, jobID string, newTime time.Time) error {
	return nil
}

// ProcessScheduledJobs does nothing
func (q *NoOpQueue) ProcessScheduledJobs(ctx context.Context) error {
	return nil
}
