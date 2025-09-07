// internal/lib/queue/types.go
package queue

import (
	"context"
	"time"
)

// Priority levels for jobs with clear semantics
type Priority int

const (
	PriorityCritical Priority = 100 // System critical operations
	PriorityHigh     Priority = 75  // User-facing operations
	PriorityNormal   Priority = 50  // Standard operations
	PriorityLow      Priority = 25  // Background tasks
	PriorityBatch    Priority = 10  // Bulk operations
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusDead       JobStatus = "dead" // Moved to DLQ
	StatusScheduled  JobStatus = "scheduled"
	StatusCancelled  JobStatus = "cancelled"
)

// ErrorCode represents types of job errors
type ErrorCode string

const (
	ErrTimeout     ErrorCode = "timeout"
	ErrPanic       ErrorCode = "panic"
	ErrValidation  ErrorCode = "validation"
	ErrDependency  ErrorCode = "dependency"
	ErrRateLimit   ErrorCode = "rate_limit"
	ErrCancelled   ErrorCode = "cancelled"
	ErrExpired     ErrorCode = "expired"
)

// JobError represents an error that occurred during job processing
type JobError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Retryable  bool      `json:"retryable"`
	OccurredAt time.Time `json:"occurred_at"`
	WorkerID   string    `json:"worker_id,omitempty"`
}

// Job represents a unit of work to be processed
type Job struct {
	// Core fields
	ID       string    `json:"id"`
	Type     string    `json:"type"`     // Maps to handler
	Payload  []byte    `json:"payload"`  // JSON encoded data
	Priority Priority  `json:"priority"` // Higher = more important
	Status   JobStatus `json:"status"`
	Queue    string    `json:"queue"` // Queue name (default, high-priority, etc)

	// Scheduling & TTL
	ScheduledAt *time.Time    `json:"scheduled_at,omitempty"`
	TTL         time.Duration `json:"ttl,omitempty"` // Job expiration
	ExpiresAt   *time.Time    `json:"expires_at,omitempty"`

	// Retry management
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	LastError   string     `json:"last_error,omitempty"`
	NextRetryAt *time.Time `json:"next_retry_at,omitempty"`
	Errors      []JobError `json:"errors,omitempty"` // Error history

	// Tracking
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Execution details
	WorkerID string `json:"worker_id,omitempty"` // Which worker processed it
	Result   []byte `json:"result,omitempty"`    // Store job results

	// Grouping & Dependencies
	GroupID   string   `json:"group_id,omitempty"`   // For job ordering/grouping
	DependsOn []string `json:"depends_on,omitempty"` // Job dependencies

	// Metadata for tracing/correlation
	CorrelationID string `json:"correlation_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`

	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// JobOptions provides a cleaner API for creating jobs
type JobOptions struct {
	Priority      Priority
	ScheduledAt   *time.Time
	TTL           time.Duration
	MaxAttempts   int
	GroupID       string
	DependsOn     []string
	CorrelationID string
	UserID        string
	TenantID      string
	Metadata      map[string]string
}

// JobHandler processes a specific type of job
type JobHandler interface {
	// Handle processes the job
	Handle(ctx context.Context, job *Job) error

	// JobType returns the type of job this handler processes
	JobType() string

	// Timeout returns the max duration for job processing (0 = use default)
	Timeout() time.Duration

	// OnSuccess is called after successful processing (optional)
	OnSuccess(ctx context.Context, job *Job)

	// OnFailure is called after failed processing (optional)
	OnFailure(ctx context.Context, job *Job, err error)
}

// Queue defines the core queue operations
type Queue interface {
	// Core operations
	Enqueue(ctx context.Context, job *Job) error
	EnqueueWithOptions(ctx context.Context, jobType string, payload []byte, opts JobOptions) error
	Dequeue(ctx context.Context, timeout time.Duration) (*Job, error)

	// Job lifecycle management
	Ack(ctx context.Context, jobID string) error
	Nack(ctx context.Context, jobID string, requeue bool) error

	// Job inspection
	GetJob(ctx context.Context, jobID string) (*Job, error)
	PeekNext(ctx context.Context) (*Job, error)

	// Queue management
	Len(ctx context.Context) (int64, error)
	Clear(ctx context.Context) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error

	// Dead letter queue
	MoveToDLQ(ctx context.Context, job *Job, reason string) error
	GetDLQ(ctx context.Context, limit int) ([]*Job, error)
	ReprocessDLQ(ctx context.Context, jobID string) error
	ClearDLQ(ctx context.Context) error

	// Scheduled jobs
	EnqueueScheduled(ctx context.Context, job *Job, scheduledAt time.Time) error
	RescheduleJob(ctx context.Context, jobID string, newTime time.Time) error
	ProcessScheduledJobs(ctx context.Context) error
}

// QueueManager orchestrates queues and workers
type QueueManager interface {
	// Queue operations
	RegisterQueue(name string, config QueueConfig) error
	GetQueue(name string) (Queue, error)
	ListQueues() []string
	Enqueue(ctx context.Context, job *Job) error
	EnqueueWithOptions(ctx context.Context, queueName, jobType string, payload []byte, opts JobOptions) error

	// Handler registration
	RegisterHandler(handler JobHandler) error
	GetHandler(jobType string) (JobHandler, bool)

	// Queue control
	PauseQueue(ctx context.Context, name string) error
	ResumeQueue(ctx context.Context, name string) error
	DrainQueue(ctx context.Context, name string) error // Graceful shutdown

	// Worker pool management
	ScaleWorkers(queueName string, count int) error
	GetWorkerPoolHealth(queueName string) WorkerPoolHealth

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Monitoring
	GetStats() QueueStats
	GetQueueStats(queueName string) QueueStats
}

// WorkerPool manages a pool of workers for processing jobs
type WorkerPool interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Scale(workers int) error
	GetWorkerCount() int
	GetHealthStatus() WorkerPoolHealth
}

// WorkerPoolHealth provides health metrics for a worker pool
type WorkerPoolHealth struct {
	ActiveWorkers   int           `json:"active_workers"`
	IdleWorkers     int           `json:"idle_workers"`
	ProcessingJobs  int           `json:"processing_jobs"`
	ErrorRate       float64       `json:"error_rate"`       // Errors per minute
	AvgProcessTime  time.Duration `json:"avg_process_time"`
	LastError       *JobError     `json:"last_error,omitempty"`
	LastErrorTime   *time.Time    `json:"last_error_time,omitempty"`
	WorkerStatuses  []WorkerStatus `json:"worker_statuses"`
}

// WorkerStatus represents the status of an individual worker
type WorkerStatus struct {
	ID          string        `json:"id"`
	State       string        `json:"state"` // idle, processing, stopped
	CurrentJob  string        `json:"current_job,omitempty"`
	JobsHandled int64         `json:"jobs_handled"`
	ErrorCount  int64         `json:"error_count"`
	StartedAt   time.Time     `json:"started_at"`
	LastJobAt   *time.Time    `json:"last_job_at,omitempty"`
}

// QueueStats provides queue metrics
type QueueStats struct {
	Name            string            `json:"name"`
	QueueDepth      int64             `json:"queue_depth"`
	ScheduledCount  int64             `json:"scheduled_count"`
	ProcessingCount int64             `json:"processing_count"`
	DLQCount        int64             `json:"dlq_count"`
	WorkerCount     int               `json:"worker_count"`
	ProcessedTotal  int64             `json:"processed_total"`
	FailedTotal     int64             `json:"failed_total"`
	AverageWaitTime time.Duration     `json:"average_wait_time"`
	AverageRunTime  time.Duration     `json:"average_run_time"`
	ErrorRate       float64           `json:"error_rate"`
	Throughput      float64           `json:"throughput"` // Jobs per second
	ByPriority      map[Priority]int64 `json:"by_priority"`
	ByStatus        map[JobStatus]int64 `json:"by_status"`
}

// QueueConfig defines configuration for a queue
type QueueConfig struct {
	Name              string        `json:"name"`
	StreamKey         string        `json:"stream_key"`      // Redis stream key
	ConsumerGroup     string        `json:"consumer_group"`
	MaxLength         int64         `json:"max_length"`      // Max stream length (0 = unlimited)
	BlockingTimeout   time.Duration `json:"blocking_timeout"`
	ClaimMinIdleTime  time.Duration `json:"claim_min_idle_time"` // For stuck messages
	RetryPolicy       RetryPolicy   `json:"retry_policy"`
	
	// DLQ settings
	EnableDLQ         bool          `json:"enable_dlq"`
	DLQMaxSize        int64         `json:"dlq_max_size"`
	DLQRetentionDays  int           `json:"dlq_retention_days"` // THIS WAS MISSING
	
	// Scheduled jobs
	EnableScheduled   bool          `json:"enable_scheduled"`
	
	// Priority settings
	EnablePriority    bool          `json:"enable_priority"`
	DefaultPriority   Priority      `json:"default_priority"`
	
	// Worker settings
	WorkerCount       int           `json:"worker_count"`
	MaxWorkers        int           `json:"max_workers"`
	MinWorkers        int           `json:"min_workers"`
	AutoScale         bool          `json:"auto_scale"`
	PrefetchCount     int           `json:"prefetch_count"` // Jobs per worker
}

// RetryPolicy defines how failed jobs should be retried
type RetryPolicy struct {
	MaxAttempts      int           `json:"max_attempts"`
	InitialDelay     time.Duration `json:"initial_delay"`
	MaxDelay         time.Duration `json:"max_delay"`
	Multiplier       float64       `json:"multiplier"`
	MaxDoublings     int           `json:"max_doublings"`     // Cap exponential growth
	RetryableErrors  []ErrorCode   `json:"retryable_errors"`  // Which errors to retry
	NonRetryableErrors []ErrorCode `json:"non_retryable_errors"`
}

// DefaultRetryPolicy returns sensible defaults
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     1 * time.Hour,
		Multiplier:   2.0,
		MaxDoublings: 10, // Cap at ~1024x initial delay
		RetryableErrors: []ErrorCode{
			ErrTimeout,
			ErrDependency,
			ErrRateLimit,
		},
		NonRetryableErrors: []ErrorCode{
			ErrValidation,
			ErrCancelled,
			ErrExpired,
		},
	}
}

// ShouldRetry determines if an error is retryable
func (r RetryPolicy) ShouldRetry(err JobError) bool {
	// Check non-retryable first (takes precedence)
	for _, code := range r.NonRetryableErrors {
		if err.Code == code {
			return false
		}
	}
	
	// If retryable list is empty, retry all except non-retryable
	if len(r.RetryableErrors) == 0 {
		return err.Retryable
	}
	
	// Check if in retryable list
	for _, code := range r.RetryableErrors {
		if err.Code == code {
			return true
		}
	}
	
	return false
}

// CalculateBackoff calculates the next retry delay
func (r RetryPolicy) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return r.InitialDelay
	}
	
	delay := r.InitialDelay
	doublings := attempt - 1
	
	// Cap the number of doublings
	if doublings > r.MaxDoublings {
		doublings = r.MaxDoublings
	}
	
	for i := 0; i < doublings; i++ {
		delay = time.Duration(float64(delay) * r.Multiplier)
		if delay > r.MaxDelay {
			return r.MaxDelay
		}
	}
	
	return delay
}

// DefaultQueueConfig returns a sensible default configuration
func DefaultQueueConfig(name string) QueueConfig {
	return QueueConfig{
		Name:             name,
		StreamKey:        "queue:stream:" + name,
		ConsumerGroup:    "workers",
		MaxLength:        100000, // Prevent unbounded growth
		BlockingTimeout:  5 * time.Second,
		ClaimMinIdleTime: 30 * time.Second,
		RetryPolicy:      DefaultRetryPolicy(),
		EnableDLQ:        true,
		DLQMaxSize:       10000,
		DLQRetentionDays: 7,  // ADDED DEFAULT
		EnableScheduled:  true,
		EnablePriority:   true,
		DefaultPriority:  PriorityNormal,
		WorkerCount:      2,
		MinWorkers:       1,
		MaxWorkers:       10,
		AutoScale:        true,
		PrefetchCount:    1,
	}
}

// ManagerConfig holds configuration for the queue manager
type ManagerConfig struct {
	// Timeouts and delays
	DefaultTimeout        time.Duration
	ShutdownTimeout       time.Duration
	
	// Retry configuration
	MaxRetries           int
	RetryInitialDelay    time.Duration
	RetryMaxDelay        time.Duration
	RetryMultiplier      float64
	
	// Health and monitoring
	HealthCheckInterval  time.Duration
	MetricsEnabled       bool
	MetricsInterval      time.Duration
	
	// Dead Letter Queue
	DLQEnabled          bool
	DLQRetentionDays    int
	
	// Scheduled jobs
	ScheduledEnabled        bool
	ScheduledPollInterval   time.Duration
	
	// Auto-scaling
	AutoScale              bool
	ScaleUpThreshold       int
	ScaleDownThreshold     int
	ScaleInterval          time.Duration
	
	// Circuit breaker
	CircuitBreakerEnabled    bool
	CircuitBreakerThreshold  int
	CircuitBreakerTimeout    time.Duration
}