package repos

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
	"github.com/lute/api/internal/queuejob"
)

func nowMilli() int64 { return time.Now().UTC().UnixMilli() }
func nowUnix() int64  { return time.Now().UTC().Unix() }

// JobQueueRepository persists FIFO queue_slots and DLQ rows (GORM only).
type JobQueueRepository struct {
	g *gorm.DB
}

func NewJobQueueRepository(db *gorm.DB) *JobQueueRepository {
	return &JobQueueRepository{g: db}
}

func (r *JobQueueRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

// Enqueue adds or replaces a queue slot row for the given job envelope.
func (r *JobQueueRepository) Enqueue(ctx context.Context, job *queuejob.Job, opts queuejob.EnqueueOpts) error {
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	if opts.TimeoutSec == 0 {
		opts.TimeoutSec = 300
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
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"queue_name", "payload", "lane", "priority", "release_at_ms", "updated_at_ms",
			}),
		}).Create(&slot).Error
	})
}

// PeekNextReadyJob reads the highest-priority ready job without dequeuing it.
// Returns nil if the queue is empty. Used by the dispatcher to inspect the selector before
// committing to a dequeue.
func (r *JobQueueRepository) PeekNextReadyJob(ctx context.Context, queueName string) (*queuejob.Job, error) {
	var slot models.QueueSlot
	err := r.q(ctx).Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Order("priority DESC, job_id ASC").First(&slot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var job queuejob.Job
	if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
		return nil, fmt.Errorf("peek job %s: %w", slot.JobID, err)
	}
	return &job, nil
}

// Dequeue assigns the highest-priority ready job to lane "none" and returns it, or nil if empty.
func (r *JobQueueRepository) Dequeue(ctx context.Context, queueName string) (*queuejob.Job, error) {
	var out *queuejob.Job
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
		var job queuejob.Job
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
				"lane":          enums.QueueLaneNone,
				"payload":       string(data),
				"updated_at_ms": nowMilli(),
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

// Complete marks a job done in its slot payload.
func (r *JobQueueRepository) Complete(ctx context.Context, jobID string, elapsedMs int64) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.Status = string(enums.QueueJobDone)
	job.DoneAt = nowUnix()
	return r.saveJob(ctx, job)
}

// Fail handles failure: retry with backoff or DLQ.
func (r *JobQueueRepository) Fail(ctx context.Context, jobID string, errMsg string) error {
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
			res := tx.Model(&models.QueueSlot{}).
				Where("job_id = ? AND lane = ?", jobID, enums.QueueLaneNone).
				Updates(map[string]interface{}{
					"lane":          enums.QueueLaneDelayed,
					"priority":      0,
					"release_at_ms": releaseMs,
					"payload":       string(data),
					"updated_at_ms": nowMilli(),
				})
			return res.Error
		})
	}

	job.Status = string(enums.QueueJobDead)
	job.DoneAt = nowUnix()

	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return r.q(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.QueueSlot{}).
			Where("job_id = ?", jobID).
			Updates(map[string]interface{}{
				"lane":             enums.QueueLaneNone,
				"payload":          string(data),
				"release_at_ms":    0,
				"updated_at_ms":    nowMilli(),
			}).Error; err != nil {
			return err
		}
		dlq := models.QueueDLQ{
			QueueName: job.Queue,
			JobID:     jobID,
		}
		return tx.Create(&dlq).Error
	})
}

// GetJob loads payload from queue_slots by job ID.
func (r *JobQueueRepository) GetJob(ctx context.Context, jobID string) (*queuejob.Job, error) {
	var slot models.QueueSlot
	if err := r.q(ctx).Where("job_id = ?", jobID).First(&slot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var job queuejob.Job
	if err := json.Unmarshal([]byte(slot.Payload), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job %s: %w", jobID, err)
	}
	return &job, nil
}

// SetWorkerID updates WorkerID inside the persisted job payload.
func (r *JobQueueRepository) SetWorkerID(ctx context.Context, jobID, workerID string) error {
	job, err := r.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	job.WorkerID = workerID
	return r.saveJob(ctx, job)
}

// DeleteJob removes slot and DLQ links.
func (r *JobQueueRepository) DeleteJob(ctx context.Context, jobID string) error {
	return r.q(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("job_id = ?", jobID).Delete(&models.QueueDLQ{}).Error; err != nil {
			return err
		}
		return tx.Where("job_id = ?", jobID).Delete(&models.QueueSlot{}).Error
	})
}

// CancelJob marks a pending job dead if it is still in ready or delayed lane.
func (r *JobQueueRepository) CancelJob(ctx context.Context, jobID string) error {
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
			"lane":             enums.QueueLaneNone,
			"release_at_ms":    0,
			"priority":         0,
			"payload":          string(data),
			"updated_at_ms":    nowMilli(),
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
func (r *JobQueueRepository) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	var n int64
	err := r.q(ctx).Model(&models.QueueSlot{}).
		Where("queue_name = ? AND lane = ?", queueName, enums.QueueLaneReady).
		Count(&n).Error
	return n, err
}

// ListQueues returns names that have ready/delayed slots or DLQ rows.
func (r *JobQueueRepository) ListQueues(ctx context.Context) ([]string, error) {
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
func (r *JobQueueRepository) ListQueueJobs(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
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
func (r *JobQueueRepository) DLQList(ctx context.Context, queueName string, offset, limit int64) ([]string, error) {
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
func (r *JobQueueRepository) DLQRetryAll(ctx context.Context, queueName string) (int, error) {
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
		if err := r.Enqueue(ctx, job, queuejob.EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

// PurgeQueue removes all ready jobs from a queue.
func (r *JobQueueRepository) PurgeQueue(ctx context.Context, queueName string) (int64, error) {
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

func (r *JobQueueRepository) saveJob(ctx context.Context, job *queuejob.Job) error {
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

// PromoteDelayed moves due delayed slots back to ready (transactional sweep).
func (r *JobQueueRepository) PromoteDelayed(ctx context.Context) (int, []string, error) {
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
			var job queuejob.Job
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
					"lane":             enums.QueueLaneReady,
					"release_at_ms":    int64(0),
					"priority":       float64(0),
					"payload":          string(data),
					"updated_at_ms":    ms,
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
