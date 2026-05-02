package repos

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/models"
)

// QueueStatsRow is one minute bucket returned from the queue stats table.
type QueueStatsRow struct {
	Minute       int64
	Processed    int64
	Failed       int64
	Enqueued     int64
	LatencySum   int64
	LatencyCount int64
}

// QueueStatsRepository persists per-minute queue counters.
type QueueStatsRepository struct {
	g *gorm.DB
}

func NewQueueStatsRepository(db *gorm.DB) *QueueStatsRepository {
	return &QueueStatsRepository{g: db}
}

func (r *QueueStatsRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func currentMinuteBucket() int64 {
	return time.Now().Unix() / 60
}

func (r *QueueStatsRepository) pruneOldStats(ctx context.Context, keepMinutes int64) {
	if keepMinutes <= 0 {
		keepMinutes = 120
	}
	cutoff := currentMinuteBucket() - keepMinutes
	_ = r.q(ctx).Where("minute_bucket < ?", cutoff).Delete(&models.QueueStatsMinute{}).Error
}

// RecordProcessed increments processed + latency accumulators for the current minute bucket.
func (r *QueueStatsRepository) RecordProcessed(ctx context.Context, queueName string, latencyMs int64) {
	r.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	_ = r.q(ctx).Transaction(func(tx *gorm.DB) error {
		return r.bumpStats(tx, queueName, b, bumpSpec{
			processedDelta:    1,
			latencySumDelta:   latencyMs,
			latencyCountDelta: 1,
		})
	})
}

// RecordFailed increments failed for the current minute bucket.
func (r *QueueStatsRepository) RecordFailed(ctx context.Context, queueName string) {
	r.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	_ = r.q(ctx).Transaction(func(tx *gorm.DB) error {
		return r.bumpStats(tx, queueName, b, bumpSpec{failedDelta: 1})
	})
}

// RecordEnqueued increments enqueued for the current minute bucket.
func (r *QueueStatsRepository) RecordEnqueued(ctx context.Context, queueName string) {
	r.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	_ = r.q(ctx).Transaction(func(tx *gorm.DB) error {
		return r.bumpStats(tx, queueName, b, bumpSpec{enqueuedDelta: 1})
	})
}

type bumpSpec struct {
	processedDelta    int64
	failedDelta       int64
	enqueuedDelta     int64
	latencySumDelta   int64
	latencyCountDelta int64
}

func (r *QueueStatsRepository) bumpStats(tx *gorm.DB, queueName string, bucket int64, spec bumpSpec) error {
	var cur models.QueueStatsMinute
	err := tx.Where("queue_name = ? AND minute_bucket = ?", queueName, bucket).
		Take(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&models.QueueStatsMinute{
			QueueName:    queueName,
			MinuteBucket: bucket,
			Processed:    spec.processedDelta,
			Failed:       spec.failedDelta,
			Enqueued:     spec.enqueuedDelta,
			LatencySum:   spec.latencySumDelta,
			LatencyCount: spec.latencyCountDelta,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&models.QueueStatsMinute{}).
		Where("queue_name = ? AND minute_bucket = ?", queueName, bucket).
		Updates(map[string]interface{}{
			"processed":     gorm.Expr("processed + ?", spec.processedDelta),
			"failed":        gorm.Expr("failed + ?", spec.failedDelta),
			"enqueued":      gorm.Expr("enqueued + ?", spec.enqueuedDelta),
			"latency_sum":   gorm.Expr("latency_sum + ?", spec.latencySumDelta),
			"latency_count": gorm.Expr("latency_count + ?", spec.latencyCountDelta),
		}).Error
}

// GetTimeSeries returns sparse rows for buckets in range (caller aligns to chart).
func (r *QueueStatsRepository) GetTimeSeries(ctx context.Context, queueName string, minutes int) ([]QueueStatsRow, error) {
	if minutes <= 0 {
		minutes = 1
	}
	r.pruneOldStats(ctx, int64(minutes)+10)
	now := currentMinuteBucket()
	result := make([]QueueStatsRow, 0, minutes)

	for i := minutes - 1; i >= 0; i-- {
		bucket := now - int64(i)
		var qr QueueStatsRow
		qr.Minute = bucket * 60

		var row models.QueueStatsMinute
		err := r.q(ctx).
			Where("queue_name = ? AND minute_bucket = ?", queueName, bucket).
			Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result = append(result, qr)
			continue
		}
		if err != nil {
			return nil, err
		}
		qr.Processed = row.Processed
		qr.Failed = row.Failed
		qr.Enqueued = row.Enqueued
		qr.LatencySum = row.LatencySum
		qr.LatencyCount = row.LatencyCount
		result = append(result, qr)
	}
	return result, nil
}
