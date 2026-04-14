package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Job represents a job stored in Redis.
type Job struct {
	ID         string            `json:"id"`
	Queue      string            `json:"queue"`
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Status     string            `json:"status"` // pending, running, done, dead
	Attempts   int               `json:"attempts"`
	MaxRetries int               `json:"max_retries"`
	TimeoutSec int               `json:"timeout_sec"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	Error      string            `json:"error,omitempty"`
	WorkerID   string            `json:"worker_id,omitempty"`
	EnqueuedAt int64             `json:"enqueued_at"`
	StartedAt  int64             `json:"started_at,omitempty"`
	DoneAt     int64             `json:"done_at,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// EnqueueOpts are options when enqueuing a job.
type EnqueueOpts struct {
	Priority   float64       // higher = dequeued first (default 0)
	Delay      time.Duration // run after delay (uses delayed ZSET)
	MaxRetries int           // default 3
	TimeoutSec int           // default 300 (5 min)
}

// Engine wraps Redis primitives for the job queue.
type Engine struct {
	rdb *redis.Client
}

func NewEngine(rdb *redis.Client) *Engine {
	return &Engine{rdb: rdb}
}

// Redis key helpers
func queueKey(name string) string  { return "queue:" + name }
func jobKey(id string) string      { return "job:" + id }
func dlqKey(name string) string    { return "dlq:" + name }
func delayedKey() string { return "delayed" }

// Enqueue adds a job to the queue (or delayed set if opts.Delay > 0).
func (e *Engine) Enqueue(ctx context.Context, job *Job, opts EnqueueOpts) error {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.TimeoutSec == 0 {
		opts.TimeoutSec = 300
	}

	job.Status = "pending"
	job.MaxRetries = opts.MaxRetries
	job.TimeoutSec = opts.TimeoutSec
	job.EnqueuedAt = time.Now().Unix()

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	pipe := e.rdb.TxPipeline()

	pipe.HSet(ctx, jobKey(job.ID), "data", data)

	if opts.Delay > 0 {
		runAt := float64(time.Now().Add(opts.Delay).Unix())
		pipe.ZAdd(ctx, delayedKey(), redis.Z{Score: runAt, Member: job.ID})
	} else {
		pipe.ZAdd(ctx, queueKey(job.Queue), redis.Z{Score: opts.Priority, Member: job.ID})
	}

	_, err = pipe.Exec(ctx)
	return err
}

// Dequeue removes the highest-priority job from the given queue.
// Returns nil, nil if the queue is empty.
func (e *Engine) Dequeue(ctx context.Context, queueName string) (*Job, error) {
	results, err := e.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     queueKey(queueName),
		Start:   "-inf",
		Stop:    "+inf",
		ByScore: true,
		Rev:     true,
		Offset:  0,
		Count:   1,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrangebyscore: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	jobID := results[0]
	removed, err := e.rdb.ZRem(ctx, queueKey(queueName), jobID).Result()
	if err != nil {
		return nil, fmt.Errorf("zrem: %w", err)
	}
	if removed == 0 {
		return nil, nil
	}

	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	job.Status = "running"
	job.StartedAt = time.Now().Unix()
	job.Attempts++
	if err := e.saveJob(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

// Complete marks a job as done.
func (e *Engine) Complete(ctx context.Context, jobID string, elapsedMs int64) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = "done"
	job.DoneAt = time.Now().Unix()
	return e.saveJob(ctx, job)
}

// Fail handles a failed job — retries with backoff or moves to DLQ.
func (e *Engine) Fail(ctx context.Context, jobID string, errMsg string) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	job.Error = errMsg

	if job.Attempts < job.MaxRetries {
		backoff := time.Duration(1<<uint(job.Attempts)) * time.Second
		job.Status = "pending"
		runAt := float64(time.Now().Add(backoff).Unix())

		pipe := e.rdb.TxPipeline()
		data, _ := json.Marshal(job)
		pipe.HSet(ctx, jobKey(job.ID), "data", data)
		pipe.ZAdd(ctx, delayedKey(), redis.Z{Score: runAt, Member: job.ID})
		_, err = pipe.Exec(ctx)
		return err
	}

	job.Status = "dead"
	job.DoneAt = time.Now().Unix()

	pipe := e.rdb.TxPipeline()
	data, _ := json.Marshal(job)
	pipe.HSet(ctx, jobKey(job.ID), "data", data)
	pipe.RPush(ctx, dlqKey(job.Queue), job.ID)
	_, err = pipe.Exec(ctx)
	return err
}

// GetJob retrieves a job by ID.
func (e *Engine) GetJob(ctx context.Context, jobID string) (*Job, error) {
	data, err := e.rdb.HGet(ctx, jobKey(jobID), "data").Result()
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", jobID, err)
	}
	var job Job
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job %s: %w", jobID, err)
	}
	return &job, nil
}

// SetWorkerID records which machine is executing a running job.
func (e *Engine) SetWorkerID(ctx context.Context, jobID, workerID string) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.WorkerID = workerID
	return e.saveJob(ctx, job)
}

// DeleteJob removes a job from Redis.
func (e *Engine) DeleteJob(ctx context.Context, jobID string) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}

	pipe := e.rdb.TxPipeline()
	pipe.Del(ctx, jobKey(jobID))
	if job.Status == "pending" {
		pipe.ZRem(ctx, queueKey(job.Queue), jobID)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// CancelJob cancels a pending job by removing it from its queue.
func (e *Engine) CancelJob(ctx context.Context, jobID string) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "pending" {
		return fmt.Errorf("can only cancel pending jobs, status is %s", job.Status)
	}

	pipe := e.rdb.TxPipeline()
	pipe.ZRem(ctx, queueKey(job.Queue), jobID)
	pipe.ZRem(ctx, delayedKey(), jobID)
	job.Status = "dead"
	job.Error = "cancelled"
	job.DoneAt = time.Now().Unix()
	data, _ := json.Marshal(job)
	pipe.HSet(ctx, jobKey(job.ID), "data", data)
	_, err = pipe.Exec(ctx)
	return err
}

// QueueDepth returns the number of pending jobs in a queue.
func (e *Engine) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	return e.rdb.ZCard(ctx, queueKey(queueName)).Result()
}

// ListQueues returns all known queue names by scanning queue:* keys.
func (e *Engine) ListQueues(ctx context.Context) ([]string, error) {
	var queues []string
	var cursor uint64
	for {
		keys, next, err := e.rdb.Scan(ctx, cursor, "queue:*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			queues = append(queues, k[len("queue:"):])
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return queues, nil
}

// ListQueueJobs returns paginated job IDs from a queue.
func (e *Engine) ListQueueJobs(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	return e.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   queueKey(queueName),
		Start: offset,
		Stop:  offset + limit - 1,
		Rev:   true,
	}).Result()
}

// DLQList returns job IDs from the dead letter queue.
func (e *Engine) DLQList(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	return e.rdb.LRange(ctx, dlqKey(queueName), offset, offset+limit-1).Result()
}

// DLQRetryAll moves all jobs from DLQ back to their queue.
func (e *Engine) DLQRetryAll(ctx context.Context, queueName string) (int, error) {
	count := 0
	for {
		jobID, err := e.rdb.LPop(ctx, dlqKey(queueName)).Result()
		if err == redis.Nil {
			break
		}
		if err != nil {
			return count, err
		}

		job, err := e.GetJob(ctx, jobID)
		if err != nil {
			continue
		}

		job.Status = "pending"
		job.Attempts = 0
		job.Error = ""
		job.DoneAt = 0
		if err := e.Enqueue(ctx, job, EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// PurgeQueue removes all pending jobs from a queue.
func (e *Engine) PurgeQueue(ctx context.Context, queueName string) (int64, error) {
	jobIDs, err := e.rdb.ZRange(ctx, queueKey(queueName), 0, -1).Result()
	if err != nil {
		return 0, err
	}

	pipe := e.rdb.TxPipeline()
	for _, id := range jobIDs {
		pipe.Del(ctx, jobKey(id))
	}
	pipe.Del(ctx, queueKey(queueName))
	_, err = pipe.Exec(ctx)
	return int64(len(jobIDs)), err
}

func (e *Engine) saveJob(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return e.rdb.HSet(ctx, jobKey(job.ID), "data", data).Err()
}

// PromoteDelayed moves delayed jobs whose run_at has passed back to their queues.
// Returns the number of jobs promoted and the distinct queue names that received work.
func (e *Engine) PromoteDelayed(ctx context.Context) (int, []string, error) {
	now := float64(time.Now().Unix())
	jobIDs, err := e.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     delayedKey(),
		Start:   "-inf",
		Stop:    fmt.Sprintf("%f", now),
		ByScore: true,
	}).Result()
	if err != nil {
		return 0, nil, err
	}

	count := 0
	queues := make(map[string]struct{})
	for _, jobID := range jobIDs {
		removed, err := e.rdb.ZRem(ctx, delayedKey(), jobID).Result()
		if err != nil || removed == 0 {
			continue
		}
		job, err := e.GetJob(ctx, jobID)
		if err != nil {
			continue
		}
		if err := e.rdb.ZAdd(ctx, queueKey(job.Queue), redis.Z{Score: 0, Member: job.ID}).Err(); err != nil {
			continue
		}
		queues[job.Queue] = struct{}{}
		count++
	}
	out := make([]string, 0, len(queues))
	for q := range queues {
		out = append(out, q)
	}
	return count, out, nil
}
