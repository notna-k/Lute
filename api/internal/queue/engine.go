package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Job represents a job stored in SQLite (queueSlots + serialized payload).
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
	Delay      time.Duration // run after delay (uses delayed lane)
	MaxRetries int           // default 3
	TimeoutSec int           // default 300 (5 min)
}

// Engine persists the job queue in SQLite (same DB as API domain tables).
type Engine struct {
	db *sql.DB
}

func NewEngine(db *sql.DB) *Engine {
	return &Engine{db: db}
}

func nowMilli() int64 { return time.Now().UnixMilli() }
func nowUnix() int64  { return time.Now().Unix() }

// Enqueue adds a job to the queue (or delayed lane if opts.Delay > 0).
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
	if job.EnqueuedAt == 0 {
		job.EnqueuedAt = nowUnix()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	lane := "ready"
	releaseMs := int64(0)
	if opts.Delay > 0 {
		lane = "delayed"
		releaseMs = time.Now().Add(opts.Delay).UnixMilli()
	}

	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM queue_dlq WHERE job_id = ?`, job.ID); err != nil {
		return err
	}

	const upsert = `
INSERT INTO queue_slots (job_id, queue_name, payload, lane, priority, release_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_id) DO UPDATE SET
	queue_name = excluded.queue_name,
	payload = excluded.payload,
	lane = excluded.lane,
	priority = excluded.priority,
	release_at_ms = excluded.release_at_ms,
	updated_at_ms = excluded.updated_at_ms`

	if _, err := tx.ExecContext(ctx, upsert,
		job.ID, job.Queue, string(data), lane, opts.Priority, releaseMs, nowMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

// Dequeue removes the highest-priority job from the given queue.
// Returns nil, nil if the queue is empty.
func (e *Engine) Dequeue(ctx context.Context, queueName string) (*Job, error) {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	const sel = `
SELECT job_id, payload FROM queue_slots
WHERE queue_name = ? AND lane = 'ready'
ORDER BY priority DESC, job_id ASC
LIMIT 1`

	var (
		jobID   string
		payload string
	)
	err = tx.QueryRowContext(ctx, sel, queueName).Scan(&jobID, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		_ = tx.Commit()
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var job Job
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job %s: %w", jobID, err)
	}

	job.Status = "running"
	job.StartedAt = nowUnix()
	job.Attempts++

	data, err := json.Marshal(&job)
	if err != nil {
		return nil, err
	}

	const upd = `UPDATE queue_slots SET lane = 'none', payload = ?, updated_at_ms = ? WHERE job_id = ? AND queue_name = ? AND lane = 'ready'`
	res, err := tx.ExecContext(ctx, upd, string(data), nowMilli(), jobID, queueName)
	if err != nil {
		return nil, err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		_ = tx.Commit()
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &job, nil
}

// Complete marks a job as done.
func (e *Engine) Complete(ctx context.Context, jobID string, elapsedMs int64) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = "done"
	job.DoneAt = nowUnix()
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
		job.Status = "pending"
		backoff := time.Duration(1<<uint(job.Attempts)) * time.Second
		releaseMs := time.Now().Add(backoff).UnixMilli()

		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		tx, err := e.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()

		const q = `UPDATE queue_slots SET lane = 'delayed', priority = 0, release_at_ms = ?, payload = ?, updated_at_ms = ? WHERE job_id = ? AND lane = 'none'`
		if _, err := tx.ExecContext(ctx, q, releaseMs, string(data), nowMilli(), jobID); err != nil {
			return err
		}
		return tx.Commit()
	}

	job.Status = "dead"
	job.DoneAt = nowUnix()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const qSlot = `UPDATE queue_slots SET lane = 'none', payload = ?, release_at_ms = 0, updated_at_ms = ? WHERE job_id = ?`
	if _, err := tx.ExecContext(ctx, qSlot, string(data), nowMilli(), jobID); err != nil {
		return err
	}
	const qDlq = `INSERT INTO queue_dlq (queue_name, job_id) VALUES (?, ?)`
	if _, err := tx.ExecContext(ctx, qDlq, job.Queue, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

// GetJob retrieves a job by ID.
func (e *Engine) GetJob(ctx context.Context, jobID string) (*Job, error) {
	const q = `SELECT payload FROM queue_slots WHERE job_id = ?`
	var payload string
	if err := e.db.QueryRowContext(ctx, q, jobID).Scan(&payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var job Job
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
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

// DeleteJob removes a job from the store (including DLQ linkage).
func (e *Engine) DeleteJob(ctx context.Context, jobID string) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM queue_dlq WHERE job_id = ?`, jobID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM queue_slots WHERE job_id = ?`, jobID); err != nil {
		return err
	}
	return tx.Commit()
}

// CancelJob cancels a pending job by removing it from ready/delayed lanes.
func (e *Engine) CancelJob(ctx context.Context, jobID string) error {
	job, err := e.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "pending" {
		return fmt.Errorf("can only cancel pending jobs, status is %s", job.Status)
	}
	job.Status = "dead"
	job.Error = "cancelled"
	job.DoneAt = nowUnix()
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	const q = `UPDATE queue_slots SET lane = 'none', release_at_ms = 0, priority = 0, payload = ?, updated_at_ms = ? WHERE job_id = ? AND lane IN ('ready','delayed')`
	res, err := e.db.ExecContext(ctx, q, string(data), nowMilli(), jobID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return fmt.Errorf("can only cancel pending jobs, status is %s", job.Status)
	}
	return nil
}

// QueueDepth returns the number of ready jobs waiting in a queue.
func (e *Engine) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	const q = `SELECT COUNT(*) FROM queue_slots WHERE queue_name = ? AND lane = 'ready'`
	var n int64
	if err := e.db.QueryRowContext(ctx, q, queueName).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListQueues returns queue names that have ready, delayed entries, or DLQ rows.
func (e *Engine) ListQueues(ctx context.Context) ([]string, error) {
	const q = `
SELECT queue_name FROM (
	SELECT DISTINCT queue_name FROM queue_slots WHERE lane IN ('ready','delayed')
	UNION
	SELECT DISTINCT queue_name FROM queue_dlq
) ORDER BY queue_name`

	rows, err := e.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// ListQueueJobs returns paginated job IDs from a queue (ready lane, priority order).
func (e *Engine) ListQueueJobs(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `SELECT job_id FROM queue_slots WHERE queue_name = ? AND lane = 'ready' ORDER BY priority DESC, job_id ASC LIMIT ? OFFSET ?`
	rows, err := e.db.QueryContext(ctx, q, queueName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DLQList returns job IDs from the dead letter queue for a queue name.
func (e *Engine) DLQList(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `SELECT job_id FROM queue_dlq WHERE queue_name = ? ORDER BY id ASC LIMIT ? OFFSET ?`
	rows, err := e.db.QueryContext(ctx, q, queueName, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DLQRetryAll moves all jobs from DLQ back to their queue.
func (e *Engine) DLQRetryAll(ctx context.Context, queueName string) (int, error) {
	jobIDs, err := e.DLQList(ctx, queueName, 0, 100000)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, jobID := range jobIDs {
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

// PurgeQueue removes all ready jobs from a queue (drops slot rows entirely).
func (e *Engine) PurgeQueue(ctx context.Context, queueName string) (int64, error) {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	const sel = `SELECT job_id FROM queue_slots WHERE queue_name = ? AND lane = 'ready'`
	rows, err := tx.QueryContext(ctx, sel, queueName)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM queue_dlq WHERE job_id = ?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM queue_slots WHERE job_id = ?`, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (e *Engine) saveJob(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	const q = `UPDATE queue_slots SET payload = ?, queue_name = ?, updated_at_ms = ? WHERE job_id = ?`
	_, err = e.db.ExecContext(ctx, q, string(data), job.Queue, nowMilli(), job.ID)
	return err
}

// PromoteDelayed moves delayed jobs whose release time has passed back to ready.
// Returns the number of jobs promoted and the distinct queue names that received work.
func (e *Engine) PromoteDelayed(ctx context.Context) (int, []string, error) {
	ms := nowMilli()
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	const qids = `
SELECT job_id FROM queue_slots
WHERE lane = 'delayed' AND release_at_ms > 0 AND release_at_ms <= ?`

	rows, err := tx.QueryContext(ctx, qids, ms)
	if err != nil {
		return 0, nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()

	queuesMap := map[string]struct{}{}
	count := 0

	for _, id := range ids {
		var payload, lane string
		var rel int64
		var queueName string
		err := tx.QueryRowContext(ctx,
			`SELECT payload, lane, release_at_ms, queue_name FROM queue_slots WHERE job_id = ?`,
			id).Scan(&payload, &lane, &rel, &queueName)
		if err != nil {
			continue
		}
		if lane != "delayed" || rel > ms {
			continue
		}
		var job Job
		if err := json.Unmarshal([]byte(payload), &job); err != nil {
			continue
		}
		job.Status = "pending"
		data, err := json.Marshal(&job)
		if err != nil {
			continue
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE queue_slots SET lane = 'ready', release_at_ms = 0, priority = 0, payload = ?, updated_at_ms = ? WHERE job_id = ? AND lane = 'delayed' AND release_at_ms = ?`,
			string(data), nowMilli(), id, rel)
		if err != nil {
			continue
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			continue
		}
		queuesMap[queueName] = struct{}{}
		count++
	}

	out := make([]string, 0, len(queuesMap))
	for q := range queuesMap {
		out = append(out, q)
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return count, out, nil
}
