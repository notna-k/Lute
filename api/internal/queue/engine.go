package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
)

// ErrJobNotRunning means the result is stale: the reaper already gave up on this attempt.
var ErrJobNotRunning = errors.New("job is not running")

func nowMilli() int64 { return time.Now().UTC().UnixMilli() }
func nowUnix() int64  { return time.Now().UTC().Unix() }

const (
	defaultTimeoutSec   = 300
	defaultLeaseGrace   = 60 * time.Second
	defaultReclaimAfter = 60 * time.Second
)

// Timings governs how long a dispatched job counts as alive and how long a
// sweeper owns one it has claimed. Zero values fall back to the defaults.
type Timings struct {
	// LeaseGrace pads the timeout so reaping does not race a result the worker is still reporting.
	LeaseGrace time.Duration
	// ReclaimAfter is how long a sweeper holds a claimed lease before another may retake it.
	ReclaimAfter time.Duration
}

func (t Timings) leaseGrace() time.Duration {
	if t.LeaseGrace <= 0 {
		return defaultLeaseGrace
	}
	return t.LeaseGrace
}

func (t Timings) reclaimAfter() time.Duration {
	if t.ReclaimAfter <= 0 {
		return defaultReclaimAfter
	}
	return t.ReclaimAfter
}

// Engine is the durable job queue: ready/delayed/leased slots in queue_slots plus the DLQ.
type Engine struct {
	g       *gorm.DB
	timings Timings
}

func NewEngine(db *gorm.DB, timings Timings) *Engine {
	return &Engine{g: db, timings: timings}
}

// leaseDeadlineMS is when a job dispatched now stops counting as alive.
func (r *Engine) leaseDeadlineMS(timeoutSec int) int64 {
	if timeoutSec <= 0 {
		timeoutSec = defaultTimeoutSec
	}
	return time.Now().Add(time.Duration(timeoutSec)*time.Second + r.timings.leaseGrace()).UnixMilli()
}

func (r *Engine) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

// Enqueue adds or replaces a queue slot row for the given job envelope.
func (r *Engine) Enqueue(ctx context.Context, job *Job, opts EnqueueOpts) error {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.TimeoutSec == 0 {
		opts.TimeoutSec = defaultTimeoutSec
	}
	job.Status = string(enums.QueueJobPending)
	job.MaxRetries = opts.MaxRetries
	job.TimeoutSec = opts.TimeoutSec
	if job.EnqueuedAt == 0 {
		job.EnqueuedAt = nowUnix()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	lane := enums.QueueLaneReady
	releaseMs := int64(0)
	if opts.Delay > 0 {
		lane = enums.QueueLaneDelayed
		releaseMs = time.Now().Add(opts.Delay).UnixMilli()
	}

	return r.q(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", job.ID).Delete(&models.QueueDLQ{}).Error; err != nil {
			return err
		}
		slot := models.QueueSlot{
			JobID:       job.ID,
			QueueName:   job.Queue,
			Payload:     string(data),
			Lane:        lane,
			Priority:    opts.Priority,
			ReleaseAtMS: releaseMs,
			UpdatedAtMS: nowMilli(),
		}
		// lease_expires_at_ms is updated too, so re-enqueuing a dispatched slot clears its old lease.
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"queue_name", "payload", "lane", "priority", "release_at_ms",
				"lease_expires_at_ms", "updated_at_ms",
			}),
		}).Create(&slot).Error
	})
}

// PeekNextReadyJob reads the highest-priority ready job without dequeuing it.
// Returns nil if the queue is empty. Used by the dispatcher to inspect the selector before
// committing to a dequeue.
func (r *Engine) PeekNextReadyJob(ctx context.Context, queueName string) (*Job, error) {
	var slot models.QueueSlot
	err := r.q(ctx).Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Order("priority DESC, job_id ASC").First(&slot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
		return nil, fmt.Errorf("peek job %s: %w", slot.JobID, err)
	}
	return &job, nil
}

// Dequeue assigns the highest-priority ready job to lane "none" and returns it, or nil if empty.
func (r *Engine) Dequeue(ctx context.Context, queueName string) (*Job, error) {
	var out *Job
	err := r.q(ctx).Transaction(func(tx *gorm.DB) error {
		var slot models.QueueSlot
		err := tx.Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
			Order("priority DESC, job_id ASC").First(&slot).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var job Job
		if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
			return fmt.Errorf("unmarshal job %s: %w", slot.JobID, err)
		}
		job.Status = string(enums.QueueJobRunning)
		job.StartedAt = nowUnix()
		job.Attempts++

		data, err := json.Marshal(&job)
		if err != nil {
			return err
		}
		res := tx.Model(&models.QueueSlot{}).
			Where("job_id = ? AND queue_name = ? AND lane = ?", slot.JobID, queueName, enums.QueueLaneReady).
			Updates(map[string]interface{}{
				"lane":                enums.QueueLaneNone,
				"payload":             string(data),
				"lease_expires_at_ms": r.leaseDeadlineMS(job.TimeoutSec),
				"updated_at_ms":       nowMilli(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		out = &job
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// leasedSlot scopes an update to a currently dispatched job: a live lease is what
// separates one a worker still owns from one already requeued, dead-lettered or reported on.
func leasedSlot(tx *gorm.DB, jobID string) *gorm.DB {
	return tx.Where("job_id = ? AND lane = ? AND lease_expires_at_ms > 0", jobID, enums.QueueLaneNone)
}

// Complete marks a job done and releases its lease, or returns ErrJobNotRunning if it was reaped.
func (r *Engine) Complete(ctx context.Context, jobID string, elapsedMs int64) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = string(enums.QueueJobDone)
	job.DoneAt = nowUnix()
	job.ElapsedMs = elapsedMs

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	res := leasedSlot(r.q(ctx).Model(&models.QueueSlot{}), jobID).
		Updates(map[string]interface{}{
			"payload":             string(data),
			"lease_expires_at_ms": int64(0),
			"updated_at_ms":       nowMilli(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrJobNotRunning
	}
	return nil
}

// Fail handles failure: retry with backoff or DLQ. Returns ErrJobNotRunning like Complete.
func (r *Engine) Fail(ctx context.Context, jobID string, errMsg string) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Error = errMsg

	if job.Attempts < job.MaxRetries {
		job.Status = string(enums.QueueJobPending)
		backoff := time.Duration(1<<uint(job.Attempts)) * time.Second
		releaseMs := time.Now().Add(backoff).UnixMilli()

		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		return r.q(ctx).Transaction(func(tx *gorm.DB) error {
			res := leasedSlot(tx.Model(&models.QueueSlot{}), jobID).
				Updates(map[string]interface{}{
					"lane":                enums.QueueLaneDelayed,
					"priority":            0,
					"release_at_ms":       releaseMs,
					"lease_expires_at_ms": int64(0),
					"payload":             string(data),
					"updated_at_ms":       nowMilli(),
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				return ErrJobNotRunning
			}
			return nil
		})
	}

	job.Status = string(enums.QueueJobDead)
	job.DoneAt = nowUnix()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return r.q(ctx).Transaction(func(tx *gorm.DB) error {
		res := leasedSlot(tx.Model(&models.QueueSlot{}), jobID).
			Updates(map[string]interface{}{
				"lane":                enums.QueueLaneNone,
				"payload":             string(data),
				"release_at_ms":       0,
				"lease_expires_at_ms": int64(0),
				"updated_at_ms":       nowMilli(),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrJobNotRunning
		}
		dlq := models.QueueDLQ{
			QueueName: job.Queue,
			JobID:     jobID,
		}
		// A job reaches the DLQ once; a duplicate must not abort the transaction.
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "job_id"}},
			DoNothing: true,
		}).Create(&dlq).Error
	})
}

// GetJob loads payload from queue_slots by job ID.
func (r *Engine) GetJob(ctx context.Context, jobID string) (*Job, error) {
	var slot models.QueueSlot
	if err := r.q(ctx).Where("job_id = ?", jobID).First(&slot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repos.ErrNotFound
		}
		return nil, err
	}
	var job Job
	if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job %s: %w", jobID, err)
	}
	return &job, nil
}

// SetWorkerID updates WorkerID inside the persisted job payload.
func (r *Engine) SetWorkerID(ctx context.Context, jobID, workerID string) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.WorkerID = workerID
	return r.saveJob(ctx, job)
}

// DeleteJob removes slot and DLQ links.
func (r *Engine) DeleteJob(ctx context.Context, jobID string) error {
	return r.q(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", jobID).Delete(&models.QueueDLQ{}).Error; err != nil {
			return err
		}
		return tx.Where("job_id = ?", jobID).Delete(&models.QueueSlot{}).Error
	})
}

// CancelJob marks a pending job dead if it is still in ready or delayed lane.
func (r *Engine) CancelJob(ctx context.Context, jobID string) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != string(enums.QueueJobPending) {
		return fmt.Errorf("can only cancel pending jobs, status is %s", job.Status)
	}
	job.Status = string(enums.QueueJobDead)
	job.Error = "cancelled"
	job.DoneAt = nowUnix()
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	res := r.q(ctx).Model(&models.QueueSlot{}).
		Where("job_id = ? AND lane IN ?", jobID, []enums.QueueLane{enums.QueueLaneReady, enums.QueueLaneDelayed}).
		Updates(map[string]interface{}{
			"lane":          enums.QueueLaneNone,
			"release_at_ms": 0,
			"priority":      0,
			"payload":       string(data),
			"updated_at_ms": nowMilli(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("can only cancel pending jobs, status is %s", job.Status)
	}
	return nil
}

// QueueDepth counts ready jobs for a queue name.
func (r *Engine) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	var n int64
	err := r.q(ctx).Model(&models.QueueSlot{}).
		Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Count(&n).Error
	return n, err
}

// ListQueues returns names that have ready/delayed slots or DLQ rows.
func (r *Engine) ListQueues(ctx context.Context) ([]string, error) {
	var fromSlots []string
	if err := r.q(ctx).Model(&models.QueueSlot{}).
		Distinct("queue_name").
		Where("lane IN ?", []enums.QueueLane{enums.QueueLaneReady, enums.QueueLaneDelayed}).
		Pluck("queue_name", &fromSlots).Error; err != nil {
		return nil, err
	}
	var fromDLQ []string
	if err := r.q(ctx).Model(&models.QueueDLQ{}).Distinct("queue_name").Pluck("queue_name", &fromDLQ).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, s := range fromSlots {
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	for _, s := range fromDLQ {
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// ListQueueJobs lists ready job IDs with pagination.
func (r *Engine) ListQueueJobs(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	var ids []string
	err := r.q(ctx).Model(&models.QueueSlot{}).
		Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Order("priority DESC, job_id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Pluck("job_id", &ids).Error
	return ids, err
}

// DLQList lists DLQ job IDs for a queue.
func (r *Engine) DLQList(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	var ids []string
	err := r.q(ctx).Model(&models.QueueDLQ{}).
		Where("queue_name = ?", queueName).
		Order("id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Pluck("job_id", &ids).Error
	return ids, err
}

// DLQRetryAll re-enqueues every DLQ job for the queue.
func (r *Engine) DLQRetryAll(ctx context.Context, queueName string) (int, error) {
	jobIDs, err := r.DLQList(ctx, queueName, 0, 100000)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, jobID := range jobIDs {
		job, err := r.GetJob(ctx, jobID)
		if err != nil {
			continue
		}
		job.Status = string(enums.QueueJobPending)
		job.Attempts = 0
		job.Error = ""
		job.DoneAt = 0
		if err := r.Enqueue(ctx, job, EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// PurgeQueue removes all ready jobs from a queue.
func (r *Engine) PurgeQueue(ctx context.Context, queueName string) (int64, error) {
	var ids []string
	if err := r.q(ctx).Model(&models.QueueSlot{}).
		Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Pluck("job_id", &ids).Error; err != nil {
		return 0, err
	}
	n := int64(len(ids))
	err := r.q(ctx).Transaction(func(tx *gorm.DB) error {
		for _, jobID := range ids {
			if err := tx.Where("job_id = ?", jobID).Delete(&models.QueueDLQ{}).Error; err != nil {
				return err
			}
			if err := tx.Where("job_id = ?", jobID).Delete(&models.QueueSlot{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return n, err
}

func (r *Engine) saveJob(ctx context.Context, job *Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return r.q(ctx).Model(&models.QueueSlot{}).
		Where("job_id = ?", job.ID).
		Updates(map[string]interface{}{
			"payload":       string(data),
			"queue_name":    job.Queue,
			"updated_at_ms": nowMilli(),
		}).Error
}

// ExpiredLease identifies a dispatched job whose worker died, hung, or ran past its timeout.
type ExpiredLease struct {
	JobID    string
	Queue    string
	WorkerID string
}

// ClaimExpiredLeases takes jobs whose lease ran out, for the caller to fail. The claim pushes
// the lease forward, so sweepers cannot collide and one dying mid-reap does not strand the job.
func (r *Engine) ClaimExpiredLeases(ctx context.Context) ([]ExpiredLease, error) {
	ms := nowMilli()
	nextMS := ms + r.timings.reclaimAfter().Milliseconds()

	var claimed []ExpiredLease
	err := r.q(ctx).Transaction(func(tx *gorm.DB) error {
		var slots []models.QueueSlot
		if err := tx.Where("lane = ? AND lease_expires_at_ms > 0 AND lease_expires_at_ms <= ?",
			enums.QueueLaneNone, ms).Find(&slots).Error; err != nil {
			return err
		}
		for _, slot := range slots {
			res := tx.Model(&models.QueueSlot{}).
				Where("job_id = ? AND lane = ? AND lease_expires_at_ms = ?",
					slot.JobID, enums.QueueLaneNone, slot.LeaseExpiresAtMS).
				Updates(map[string]interface{}{
					"lease_expires_at_ms": nextMS,
					"updated_at_ms":       ms,
				})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				continue // another sweeper, or the worker's own result, got there first
			}
			lease := ExpiredLease{JobID: slot.JobID, Queue: slot.QueueName}
			var job Job
			if err := json.Unmarshal([]byte(slot.Payload), &job); err == nil {
				lease.WorkerID = job.WorkerID
			}
			claimed = append(claimed, lease)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// PromoteDelayed moves due delayed slots back to ready (transactional sweep).
func (r *Engine) PromoteDelayed(ctx context.Context) (int, []string, error) {
	ms := nowMilli()

	var promoted int
	var queueNames []string
	err := r.q(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Model(&models.QueueSlot{}).
			Where("lane = ? AND release_at_ms > 0 AND release_at_ms <= ?", enums.QueueLaneDelayed, ms).
			Pluck("job_id", &ids).Error; err != nil {
			return err
		}
		qset := make(map[string]struct{})
		count := 0
		for _, jid := range ids {
			var slot models.QueueSlot
			if err := tx.Where("job_id = ?", jid).First(&slot).Error; err != nil {
				continue
			}
			if slot.Lane != enums.QueueLaneDelayed || slot.ReleaseAtMS > ms {
				continue
			}
			var job Job
			if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
				continue
			}
			job.Status = string(enums.QueueJobPending)
			data, err := json.Marshal(&job)
			if err != nil {
				continue
			}
			res := tx.Model(&models.QueueSlot{}).
				Where("job_id = ? AND lane = ? AND release_at_ms = ?", jid, enums.QueueLaneDelayed, slot.ReleaseAtMS).
				Updates(map[string]interface{}{
					"lane":          enums.QueueLaneReady,
					"release_at_ms": int64(0),
					"priority":      float64(0),
					"payload":       string(data),
					"updated_at_ms": ms,
				})
			if res.Error != nil {
				continue
			}
			if res.RowsAffected == 0 {
				continue
			}
			qset[slot.QueueName] = struct{}{}
			count++
		}
		promoted = count
		queueNames = make([]string, 0, len(qset))
		for q := range qset {
			queueNames = append(queueNames, q)
		}
		sort.Strings(queueNames)
		return nil
	})
	return promoted, queueNames, err
}
