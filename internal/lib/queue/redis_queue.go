// internal/lib/queue/redis_queue.go
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/0xsj/go-core/internal/lib/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisQueue implements Queue interface using Redis STREAM
type RedisQueue struct {
	name          string
	config        QueueConfig
	redis         *redis.Client
	logger        logger.Logger
	streamKey     string
	consumerGroup string
	dlqKey        string
	scheduledKey  string
	processingKey string
	isPaused      bool
}

// NewRedisQueue creates a new Redis-based queue
func NewRedisQueue(name string, config QueueConfig, redis *redis.Client, appLogger logger.Logger) (*RedisQueue, error) {
	q := &RedisQueue{
		name:          name,
		config:        config,
		redis:         redis,
		logger:        appLogger,
		streamKey:     config.StreamKey,
		consumerGroup: config.ConsumerGroup,
		dlqKey:        fmt.Sprintf("%s:dlq", config.StreamKey),
		scheduledKey:  fmt.Sprintf("%s:scheduled", config.StreamKey),
		processingKey: fmt.Sprintf("%s:processing", config.StreamKey),
		isPaused:      false,
	}

	// Create consumer group if it doesn't exist
	ctx := context.Background()
	err := q.createConsumerGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	appLogger.Info("Redis queue initialized",
		logger.String("queue", name),
		logger.String("stream_key", q.streamKey),
		logger.String("consumer_group", q.consumerGroup),
	)

	return q, nil
}

// createConsumerGroup creates the Redis consumer group for this queue
func (q *RedisQueue) createConsumerGroup(ctx context.Context) error {
	// Try to create the consumer group
	// If stream doesn't exist, create it with the consumer group
	err := q.redis.XGroupCreateMkStream(ctx, q.streamKey, q.consumerGroup, "0").Err()
	if err != nil {
		// Check if group already exists (not an error)
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
		return err
	}
	return nil
}

// Enqueue adds a job to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, job *Job) error {
	if q.isPaused {
		return fmt.Errorf("queue %s is paused", q.name)
	}

	// Set job metadata
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	job.Queue = q.name
	job.Status = StatusPending
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	// If no priority set, use queue default
	if job.Priority == 0 {
		job.Priority = q.config.DefaultPriority
	}

	// If scheduled job, add to scheduled set instead
	if job.ScheduledAt != nil && job.ScheduledAt.After(time.Now()) {
		return q.EnqueueScheduled(ctx, job, *job.ScheduledAt)
	}

	// Serialize job
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Add to stream with priority as a field for sorting
	streamID, err := q.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		MaxLen: q.config.MaxLength,
		Approx: true,
		Values: map[string]interface{}{
			"job_id":   job.ID,
			"job_type": job.Type,
			"priority": int(job.Priority),
			"data":     jobData,
		},
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	q.logger.Debug("Job enqueued",
		logger.String("job_id", job.ID),
		logger.String("job_type", job.Type),
		logger.String("stream_id", streamID),
		logger.Int("priority", int(job.Priority)),
	)

	return nil
}

// EnqueueWithOptions creates and enqueues a job with options
func (q *RedisQueue) EnqueueWithOptions(ctx context.Context, jobType string, payload []byte, opts JobOptions) error {
	job := &Job{
		ID:            uuid.New().String(),
		Type:          jobType,
		Payload:       payload,
		Priority:      opts.Priority,
		MaxAttempts:   opts.MaxAttempts,
		ScheduledAt:   opts.ScheduledAt,
		TTL:           opts.TTL,
		GroupID:       opts.GroupID,
		DependsOn:     opts.DependsOn,
		CorrelationID: opts.CorrelationID,
		UserID:        opts.UserID,
		TenantID:      opts.TenantID,
		Metadata:      opts.Metadata,
	}

	if job.MaxAttempts == 0 {
		job.MaxAttempts = q.config.RetryPolicy.MaxAttempts
	}

	return q.Enqueue(ctx, job)
}

// Dequeue retrieves and claims a job from the queue
func (q *RedisQueue) Dequeue(ctx context.Context, timeout time.Duration) (*Job, error) {
	if q.isPaused {
		return nil, fmt.Errorf("queue %s is paused", q.name)
	}

	consumerName := fmt.Sprintf("worker-%s", uuid.New().String())

	// Read from stream with blocking
	streams, err := q.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.consumerGroup,
		Consumer: consumerName,
		Streams:  []string{q.streamKey, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			// No messages available
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}

	message := streams[0].Messages[0]

	// Extract job data
	jobData, ok := message.Values["data"].(string)
	if !ok {
		// Acknowledge the message to remove it from pending
		q.redis.XAck(ctx, q.streamKey, q.consumerGroup, message.ID)
		return nil, fmt.Errorf("invalid job data in stream")
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		// Acknowledge the bad message
		q.redis.XAck(ctx, q.streamKey, q.consumerGroup, message.ID)
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Update job status
	job.Status = StatusProcessing
	now := time.Now()
	job.StartedAt = &now
	job.UpdatedAt = time.Now()
	job.Attempts++

	// Store the stream ID for acknowledgment
	if job.Metadata == nil {
		job.Metadata = make(map[string]string)
	}
	job.Metadata["stream_id"] = message.ID
	job.Metadata["consumer_name"] = consumerName

	// Track in processing set with TTL
	processingData, _ := json.Marshal(job)
	q.redis.Set(ctx, fmt.Sprintf("%s:%s", q.processingKey, job.ID), processingData, 30*time.Minute)

	q.logger.Debug("Job dequeued",
		logger.String("job_id", job.ID),
		logger.String("job_type", job.Type),
		logger.String("stream_id", message.ID),
		logger.Int("attempt", job.Attempts),
	)

	return &job, nil
}

// Ack acknowledges successful job completion
func (q *RedisQueue) Ack(ctx context.Context, jobID string) error {
	// Get job from processing set
	processingKey := fmt.Sprintf("%s:%s", q.processingKey, jobID)
	jobData, err := q.redis.Get(ctx, processingKey).Result()
	if err != nil {
		return fmt.Errorf("job not found in processing: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Get stream ID from metadata
	streamID, ok := job.Metadata["stream_id"]
	if !ok {
		return fmt.Errorf("stream ID not found in job metadata")
	}

	// Acknowledge the message in Redis STREAM
	count, err := q.redis.XAck(ctx, q.streamKey, q.consumerGroup, streamID).Result()
	if err != nil {
		return fmt.Errorf("failed to acknowledge job: %w", err)
	}

	if count == 0 {
		q.logger.Warn("Job already acknowledged",
			logger.String("job_id", jobID),
			logger.String("stream_id", streamID),
		)
	}

	// Remove from processing set
	q.redis.Del(ctx, processingKey)

	// Delete the message from stream to save memory
	q.redis.XDel(ctx, q.streamKey, streamID)

	q.logger.Debug("Job acknowledged",
		logger.String("job_id", jobID),
		logger.String("stream_id", streamID),
	)

	return nil
}

// Nack negatively acknowledges a job (retry or move to DLQ)
func (q *RedisQueue) Nack(ctx context.Context, jobID string, requeue bool) error {
	// Get job from processing set
	processingKey := fmt.Sprintf("%s:%s", q.processingKey, jobID)
	jobData, err := q.redis.Get(ctx, processingKey).Result()
	if err != nil {
		return fmt.Errorf("job not found in processing: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Get stream ID from metadata
	streamID, ok := job.Metadata["stream_id"]
	if !ok {
		return fmt.Errorf("stream ID not found in job metadata")
	}

	// First, acknowledge the message to remove from pending
	q.redis.XAck(ctx, q.streamKey, q.consumerGroup, streamID)

	// Delete the original message
	q.redis.XDel(ctx, q.streamKey, streamID)

	// Remove from processing set
	q.redis.Del(ctx, processingKey)

	if !requeue {
		// Move to DLQ
		return q.MoveToDLQ(ctx, &job, "nacked without requeue")
	}

	// Check if we should retry
	if job.Attempts >= job.MaxAttempts {
		// Max attempts reached, move to DLQ
		return q.MoveToDLQ(ctx, &job, fmt.Sprintf("max attempts (%d) reached", job.MaxAttempts))
	}

	// Calculate backoff
	backoff := q.config.RetryPolicy.CalculateBackoff(job.Attempts)
	nextRetry := time.Now().Add(backoff)
	job.NextRetryAt = &nextRetry
	job.Status = StatusScheduled

	// Re-enqueue with delay
	return q.EnqueueScheduled(ctx, &job, nextRetry)
}

// GetJob retrieves a job by ID without dequeuing
func (q *RedisQueue) GetJob(ctx context.Context, jobID string) (*Job, error) {
	// Check processing set first
	processingKey := fmt.Sprintf("%s:%s", q.processingKey, jobID)
	jobData, err := q.redis.Get(ctx, processingKey).Result()
	if err == nil {
		var job Job
		if err := json.Unmarshal([]byte(jobData), &job); err == nil {
			return &job, nil
		}
	}

	// Check scheduled set
	score, err := q.redis.ZScore(ctx, q.scheduledKey, jobID).Result()
	if err == nil && score > 0 {
		members, err := q.redis.ZRangeByScore(ctx, q.scheduledKey, &redis.ZRangeBy{
			Min:   fmt.Sprintf("%f", score),
			Max:   fmt.Sprintf("%f", score),
			Count: 1,
		}).Result()

		if err == nil && len(members) > 0 {
			var job Job
			if err := json.Unmarshal([]byte(members[0]), &job); err == nil {
				return &job, nil
			}
		}
	}

	// Check DLQ
	dlqData, err := q.redis.HGet(ctx, q.dlqKey, jobID).Result()
	if err == nil {
		var job Job
		if err := json.Unmarshal([]byte(dlqData), &job); err == nil {
			return &job, nil
		}
	}

	return nil, fmt.Errorf("job %s not found", jobID)
}

// PeekNext returns the next job without dequeuing
func (q *RedisQueue) PeekNext(ctx context.Context) (*Job, error) {
	// Check scheduled jobs first
	if err := q.ProcessScheduledJobs(ctx); err != nil {
		q.logger.Error("Failed to process scheduled jobs", logger.Err(err))
	}

	// Peek at the next message in stream
	messages, err := q.redis.XRangeN(ctx, q.streamKey, "-", "+", 1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to peek stream: %w", err)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	jobData, ok := messages[0].Values["data"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid job data in stream")
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Len returns the number of jobs in the queue
func (q *RedisQueue) Len(ctx context.Context) (int64, error) {
	length, err := q.redis.XLen(ctx, q.streamKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get stream length: %w", err)
	}
	return length, nil
}

// Clear removes all jobs from the queue
func (q *RedisQueue) Clear(ctx context.Context) error {
	// Clear main stream
	if err := q.redis.Del(ctx, q.streamKey).Err(); err != nil {
		return fmt.Errorf("failed to clear stream: %w", err)
	}

	// Recreate consumer group
	if err := q.createConsumerGroup(ctx); err != nil {
		return fmt.Errorf("failed to recreate consumer group: %w", err)
	}

	// Clear scheduled jobs
	if err := q.redis.Del(ctx, q.scheduledKey).Err(); err != nil {
		return fmt.Errorf("failed to clear scheduled jobs: %w", err)
	}

	// Clear processing set
	pattern := fmt.Sprintf("%s:*", q.processingKey)
	keys, err := q.redis.Keys(ctx, pattern).Result()
	if err == nil && len(keys) > 0 {
		q.redis.Del(ctx, keys...)
	}

	q.logger.Info("Queue cleared", logger.String("queue", q.name))
	return nil
}

// ... continuing from Clear method

// Pause stops processing of the queue
func (q *RedisQueue) Pause(ctx context.Context) error {
	q.isPaused = true

	// Set pause flag in Redis for distributed coordination
	pauseKey := fmt.Sprintf("%s:paused", q.streamKey)
	if err := q.redis.Set(ctx, pauseKey, "true", 0).Err(); err != nil {
		return fmt.Errorf("failed to set pause flag: %w", err)
	}

	q.logger.Info("Queue paused", logger.String("queue", q.name))
	return nil
}

// Resume resumes processing of the queue
func (q *RedisQueue) Resume(ctx context.Context) error {
	q.isPaused = false

	// Remove pause flag in Redis
	pauseKey := fmt.Sprintf("%s:paused", q.streamKey)
	if err := q.redis.Del(ctx, pauseKey).Err(); err != nil {
		return fmt.Errorf("failed to remove pause flag: %w", err)
	}

	q.logger.Info("Queue resumed", logger.String("queue", q.name))
	return nil
}

// MoveToDLQ moves a job to the dead letter queue
func (q *RedisQueue) MoveToDLQ(ctx context.Context, job *Job, reason string) error {
	if !q.config.EnableDLQ {
		q.logger.Warn("DLQ disabled, dropping job",
			logger.String("job_id", job.ID),
			logger.String("reason", reason),
		)
		return nil
	}

	// Update job status
	job.Status = StatusDead
	job.UpdatedAt = time.Now()
	if job.Errors == nil {
		job.Errors = []JobError{}
	}
	job.Errors = append(job.Errors, JobError{
		Code:       ErrExpired,
		Message:    reason,
		Retryable:  false,
		OccurredAt: time.Now(),
	})

	// Serialize job
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job for DLQ: %w", err)
	}

	// Check DLQ size limit
	dlqSize, _ := q.redis.HLen(ctx, q.dlqKey).Result()
	if q.config.DLQMaxSize > 0 && dlqSize >= q.config.DLQMaxSize {
		// Remove oldest entry (this is a simple approach, could be improved)
		q.logger.Warn("DLQ at max size, removing oldest entry",
			logger.String("queue", q.name),
			logger.Int("size", int(dlqSize)),
		)

		// Get all keys and remove the oldest one
		allKeys, _ := q.redis.HKeys(ctx, q.dlqKey).Result()
		if len(allKeys) > 0 {
			q.redis.HDel(ctx, q.dlqKey, allKeys[0])
		}
	}

	// Add to DLQ hash with job ID as key
	if err := q.redis.HSet(ctx, q.dlqKey, job.ID, jobData).Err(); err != nil {
		return fmt.Errorf("failed to add job to DLQ: %w", err)
	}

	// Set expiration for DLQ entry if retention is configured
	if q.config.DLQRetentionDays > 0 {
		dlqEntryKey := fmt.Sprintf("%s:%s", q.dlqKey, job.ID)
		retention := time.Duration(q.config.DLQRetentionDays) * 24 * time.Hour
		q.redis.Set(ctx, dlqEntryKey, time.Now().Unix(), retention)
	}

	q.logger.Warn("Job moved to DLQ",
		logger.String("job_id", job.ID),
		logger.String("job_type", job.Type),
		logger.String("reason", reason),
		logger.Int("attempts", job.Attempts),
	)

	return nil
}

// GetDLQ retrieves jobs from the dead letter queue
func (q *RedisQueue) GetDLQ(ctx context.Context, limit int) ([]*Job, error) {
	// Get all DLQ entries
	dlqData, err := q.redis.HGetAll(ctx, q.dlqKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ entries: %w", err)
	}

	jobs := make([]*Job, 0, len(dlqData))
	count := 0

	for _, jobData := range dlqData {
		if limit > 0 && count >= limit {
			break
		}

		var job Job
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			q.logger.Error("Failed to unmarshal DLQ job", logger.Err(err))
			continue
		}

		jobs = append(jobs, &job)
		count++
	}

	return jobs, nil
}

// ReprocessDLQ moves a job from DLQ back to the queue
func (q *RedisQueue) ReprocessDLQ(ctx context.Context, jobID string) error {
	// Get job from DLQ
	jobData, err := q.redis.HGet(ctx, q.dlqKey, jobID).Result()
	if err != nil {
		return fmt.Errorf("job %s not found in DLQ: %w", jobID, err)
	}

	var job Job
	if err := json.Unmarshal([]byte(jobData), &job); err != nil {
		return fmt.Errorf("failed to unmarshal DLQ job: %w", err)
	}

	// Reset job for reprocessing
	job.Status = StatusPending
	job.Attempts = 0
	job.LastError = ""
	job.NextRetryAt = nil
	job.StartedAt = nil
	job.CompletedAt = nil
	job.UpdatedAt = time.Now()

	// Remove from DLQ
	if err := q.redis.HDel(ctx, q.dlqKey, jobID).Err(); err != nil {
		return fmt.Errorf("failed to remove job from DLQ: %w", err)
	}

	// Re-enqueue
	if err := q.Enqueue(ctx, &job); err != nil {
		// If re-enqueue fails, put it back in DLQ
		q.redis.HSet(ctx, q.dlqKey, jobID, jobData)
		return fmt.Errorf("failed to reprocess DLQ job: %w", err)
	}

	q.logger.Info("Job reprocessed from DLQ",
		logger.String("job_id", jobID),
		logger.String("job_type", job.Type),
	)

	return nil
}

// ClearDLQ removes all jobs from the dead letter queue
func (q *RedisQueue) ClearDLQ(ctx context.Context) error {
	count, err := q.redis.Del(ctx, q.dlqKey).Result()
	if err != nil {
		return fmt.Errorf("failed to clear DLQ: %w", err)
	}

	q.logger.Info("DLQ cleared",
		logger.String("queue", q.name),
		logger.Int("removed_count", int(count)),
	)

	return nil
}

// EnqueueScheduled adds a job to be processed at a specific time
func (q *RedisQueue) EnqueueScheduled(ctx context.Context, job *Job, scheduledAt time.Time) error {
	if !q.config.EnableScheduled {
		return fmt.Errorf("scheduled jobs are disabled for queue %s", q.name)
	}

	// Set job metadata
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	job.Queue = q.name
	job.Status = StatusScheduled
	job.ScheduledAt = &scheduledAt
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()

	// Serialize job
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal scheduled job: %w", err)
	}

	// Add to scheduled sorted set with timestamp as score
	score := float64(scheduledAt.Unix())
	if err := q.redis.ZAdd(ctx, q.scheduledKey, redis.Z{
		Score:  score,
		Member: jobData,
	}).Err(); err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	q.logger.Debug("Job scheduled",
		logger.String("job_id", job.ID),
		logger.String("job_type", job.Type),
		logger.String("scheduled_at", scheduledAt.Format(time.RFC3339)),
	)

	return nil
}

// RescheduleJob updates the scheduled time for a job
func (q *RedisQueue) RescheduleJob(ctx context.Context, jobID string, newTime time.Time) error {
	// Get all scheduled jobs
	scheduled, err := q.redis.ZRangeWithScores(ctx, q.scheduledKey, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	for _, z := range scheduled {
		jobData := z.Member.(string)
		var job Job
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			continue
		}

		if job.ID == jobID {
			// Remove old entry
			if err := q.redis.ZRem(ctx, q.scheduledKey, jobData).Err(); err != nil {
				return fmt.Errorf("failed to remove old scheduled job: %w", err)
			}

			// Update scheduled time
			job.ScheduledAt = &newTime
			job.UpdatedAt = time.Now()

			// Re-add with new time
			return q.EnqueueScheduled(ctx, &job, newTime)
		}
	}

	return fmt.Errorf("scheduled job %s not found", jobID)
}

// ProcessScheduledJobs moves due scheduled jobs to the main queue
func (q *RedisQueue) ProcessScheduledJobs(ctx context.Context) error {
	if !q.config.EnableScheduled {
		return nil
	}

	now := time.Now()
	maxScore := fmt.Sprintf("%d", now.Unix())

	// Get all jobs scheduled for now or earlier
	scheduled, err := q.redis.ZRangeByScoreWithScores(ctx, q.scheduledKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: maxScore,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	for _, z := range scheduled {
		jobData := z.Member.(string)

		// Remove from scheduled set
		if err := q.redis.ZRem(ctx, q.scheduledKey, jobData).Err(); err != nil {
			q.logger.Error("Failed to remove scheduled job", logger.Err(err))
			continue
		}

		// Unmarshal job
		var job Job
		if err := json.Unmarshal([]byte(jobData), &job); err != nil {
			q.logger.Error("Failed to unmarshal scheduled job", logger.Err(err))
			continue
		}

		// Clear scheduled time and enqueue
		job.ScheduledAt = nil
		job.Status = StatusPending
		job.UpdatedAt = time.Now()

		if err := q.Enqueue(ctx, &job); err != nil {
			q.logger.Error("Failed to enqueue scheduled job",
				logger.String("job_id", job.ID),
				logger.Err(err),
			)

			// Put it back in scheduled set for retry
			q.redis.ZAdd(ctx, q.scheduledKey, redis.Z{
				Score:  z.Score + 60, // Retry in 1 minute
				Member: jobData,
			})
		} else {
			q.logger.Debug("Scheduled job enqueued",
				logger.String("job_id", job.ID),
				logger.String("job_type", job.Type),
			)
		}
	}

	return nil
}

// func (q *RedisQueue) checkPaused(ctx context.Context) bool {
// 	pauseKey := fmt.Sprintf("%s:paused", q.streamKey)
// 	val, err := q.redis.Get(ctx, pauseKey).Result()
// 	if err == nil && val == "true" {
// 		q.isPaused = true
// 		return true
// 	}
// 	q.isPaused = false
// 	return false
// }

// ClaimStuckMessages claims messages that have been pending too long
func (q *RedisQueue) ClaimStuckMessages(ctx context.Context, minIdleTime time.Duration) error {
	// Get pending messages
	pending, err := q.redis.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: q.streamKey,
		Group:  q.consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	for _, p := range pending {
		if p.Idle >= minIdleTime {
			// Try to claim the message
			messages, err := q.redis.XClaim(ctx, &redis.XClaimArgs{
				Stream:   q.streamKey,
				Group:    q.consumerGroup,
				Consumer: "reclaimer",
				MinIdle:  minIdleTime,
				Messages: []string{p.ID},
			}).Result()
			if err != nil {
				q.logger.Error("Failed to claim stuck message",
					logger.String("message_id", p.ID),
					logger.Err(err),
				)
				continue
			}

			if len(messages) > 0 {
				// Extract and re-enqueue the job
				jobData, ok := messages[0].Values["data"].(string)
				if ok {
					var job Job
					if err := json.Unmarshal([]byte(jobData), &job); err == nil {
						// Mark as failed and move to DLQ or retry
						if err := q.Nack(ctx, job.ID, job.Attempts < job.MaxAttempts); err != nil {
							q.logger.Error("Failed to nack stuck job",
								logger.String("job_id", job.ID),
								logger.String("message_id", p.ID),
								logger.Err(err),
							)
							// Continue processing other stuck messages
						}
					}
				}

				q.logger.Info("Claimed stuck message",
					logger.String("message_id", p.ID),
					logger.Duration("idle_time", p.Idle),
				)
			}
		}
	}

	return nil
}

// GetStats returns statistics for this queue
func (q *RedisQueue) GetStats(ctx context.Context) (QueueStats, error) {
	stats := QueueStats{
		Name: q.name,
	}

	// Get queue depth
	depth, err := q.Len(ctx)
	if err == nil {
		stats.QueueDepth = depth
	}

	// Get scheduled count
	scheduledCount, err := q.redis.ZCard(ctx, q.scheduledKey).Result()
	if err == nil {
		stats.ScheduledCount = scheduledCount
	}

	// Get DLQ count
	dlqCount, err := q.redis.HLen(ctx, q.dlqKey).Result()
	if err == nil {
		stats.DLQCount = dlqCount
	}

	// Get processing count
	pattern := fmt.Sprintf("%s:*", q.processingKey)
	keys, err := q.redis.Keys(ctx, pattern).Result()
	if err == nil {
		stats.ProcessingCount = int64(len(keys))
	}

	return stats, nil
}
