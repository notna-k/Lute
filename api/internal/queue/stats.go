package queue

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// StatsAggregator records per-minute counters for each queue (SQLite).
type StatsAggregator struct {
	db *sql.DB
}

func NewStatsAggregator(db *sql.DB) *StatsAggregator {
	return &StatsAggregator{db: db}
}

func currentMinuteBucket() int64 {
	return time.Now().Unix() / 60
}

func (s *StatsAggregator) pruneOldStats(ctx context.Context, keepMinutes int64) {
	if keepMinutes <= 0 {
		keepMinutes = 120
	}
	cutoff := currentMinuteBucket() - keepMinutes
	_, _ = s.db.ExecContext(ctx, `DELETE FROM queue_stats_minute WHERE minute_bucket < ?`, cutoff)
}

// RecordProcessed increments the processed counter for a queue.
func (s *StatsAggregator) RecordProcessed(ctx context.Context, queueName string, latencyMs int64) {
	s.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	const q = `
INSERT INTO queue_stats_minute (queue_name, minute_bucket, processed, latency_sum, latency_count)
VALUES (?, ?, 1, ?, 1)
ON CONFLICT(queue_name, minute_bucket) DO UPDATE SET
	processed = processed + 1,
	latency_sum = latency_sum + excluded.latency_sum,
	latency_count = latency_count + excluded.latency_count`
	_, _ = s.db.ExecContext(ctx, q, queueName, b, latencyMs)
}

// RecordFailed increments the failed counter for a queue.
func (s *StatsAggregator) RecordFailed(ctx context.Context, queueName string) {
	s.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	const q = `
INSERT INTO queue_stats_minute (queue_name, minute_bucket, failed)
VALUES (?, ?, 1)
ON CONFLICT(queue_name, minute_bucket) DO UPDATE SET failed = failed + 1`
	_, _ = s.db.ExecContext(ctx, q, queueName, b)
}

// RecordEnqueued increments the enqueued counter for a queue.
func (s *StatsAggregator) RecordEnqueued(ctx context.Context, queueName string) {
	s.pruneOldStats(ctx, 120)
	b := currentMinuteBucket()
	const q = `
INSERT INTO queue_stats_minute (queue_name, minute_bucket, enqueued)
VALUES (?, ?, 1)
ON CONFLICT(queue_name, minute_bucket) DO UPDATE SET enqueued = enqueued + 1`
	_, _ = s.db.ExecContext(ctx, q, queueName, b)
}

// QueueStats holds per-minute stats for a queue.
type QueueStats struct {
	Minute       int64   `json:"minute"`
	Processed    int64   `json:"processed"`
	Failed       int64   `json:"failed"`
	Enqueued     int64   `json:"enqueued"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// GetTimeSeries returns per-minute stats for the last N minutes.
func (s *StatsAggregator) GetTimeSeries(ctx context.Context, queueName string, minutes int) ([]QueueStats, error) {
	s.pruneOldStats(ctx, int64(minutes)+10)
	now := currentMinuteBucket()
	result := make([]QueueStats, 0, minutes)

	const sel = `
SELECT processed, failed, enqueued, latency_sum, latency_count
FROM queue_stats_minute WHERE queue_name = ? AND minute_bucket = ?`

	for i := minutes - 1; i >= 0; i-- {
		bucket := now - int64(i)
		var qs QueueStats
		qs.Minute = bucket * 60
		row := s.db.QueryRowContext(ctx, sel, queueName, bucket)
		var latencySum, latencyCount sql.NullInt64
		err := row.Scan(&qs.Processed, &qs.Failed, &qs.Enqueued, &latencySum, &latencyCount)
		if errors.Is(err, sql.ErrNoRows) {
			result = append(result, qs)
			continue
		}
		if err != nil {
			return nil, err
		}
		if latencyCount.Valid && latencyCount.Int64 > 0 && latencySum.Valid {
			qs.AvgLatencyMs = float64(latencySum.Int64) / float64(latencyCount.Int64)
		}
		result = append(result, qs)
	}
	return result, nil
}
