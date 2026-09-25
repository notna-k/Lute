package queue

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lute/api/internal/db/models"
)

// statsRetentionMinutes is how much per-minute history the stats table keeps.
const statsRetentionMinutes = 120

// QueueStats is one minute of throughput and latency for a queue.
type QueueStats struct {
	Minute       int64   `json:"minute"`
	Processed    int64   `json:"processed"`
	Failed       int64   `json:"failed"`
	Enqueued     int64   `json:"enqueued"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// Stats records per-minute queue counters in queue_stats_minute. Recording is
// best-effort: a lost increment must never fail the job it describes.
type Stats struct {
	g *gorm.DB
}

func NewStats(db *gorm.DB) *Stats {
	return &Stats{g: db}
}

func (s *Stats) RecordProcessed(ctx context.Context, queueName string, latencyMs int64) {
	s.bump(ctx, queueName, models.QueueStatsMinute{Processed: 1, LatencySum: latencyMs, LatencyCount: 1})
}

func (s *Stats) RecordFailed(ctx context.Context, queueName string) {
	s.bump(ctx, queueName, models.QueueStatsMinute{Failed: 1})
}

func (s *Stats) RecordEnqueued(ctx context.Context, queueName string) {
	s.bump(ctx, queueName, models.QueueStatsMinute{Enqueued: 1})
}

// GetTimeSeries returns one entry per minute for the last `minutes` minutes, oldest first.
func (s *Stats) GetTimeSeries(ctx context.Context, queueName string, minutes int) ([]QueueStats, error) {
	if minutes <= 0 {
		minutes = 1
	}
	s.prune(ctx, int64(minutes)+10)
	now := currentMinuteBucket()

	var rows []models.QueueStatsMinute
	if err := s.g.WithContext(ctx).
		Where("queue_name = ? AND minute_bucket > ?", queueName, now-int64(minutes)).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	byBucket := make(map[int64]models.QueueStatsMinute, len(rows))
	for _, r := range rows {
		byBucket[r.MinuteBucket] = r
	}

	out := make([]QueueStats, 0, minutes)
	for i := minutes - 1; i >= 0; i-- {
		bucket := now - int64(i)
		row := byBucket[bucket]
		p := QueueStats{
			Minute:    bucket * 60,
			Processed: row.Processed,
			Failed:    row.Failed,
			Enqueued:  row.Enqueued,
		}
		if row.LatencyCount > 0 {
			p.AvgLatencyMs = float64(row.LatencySum) / float64(row.LatencyCount)
		}
		out = append(out, p)
	}
	return out, nil
}

func currentMinuteBucket() int64 {
	return time.Now().Unix() / 60
}

func (s *Stats) prune(ctx context.Context, keepMinutes int64) {
	cutoff := currentMinuteBucket() - keepMinutes
	_ = s.g.WithContext(ctx).Where("minute_bucket < ?", cutoff).Delete(&models.QueueStatsMinute{}).Error
}

func (s *Stats) bump(ctx context.Context, queueName string, d models.QueueStatsMinute) {
	s.prune(ctx, statsRetentionMinutes)
	d.QueueName, d.MinuteBucket = queueName, currentMinuteBucket()
	_ = s.g.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "queue_name"}, {Name: "minute_bucket"}},
		DoUpdates: clause.Set{
			{Column: clause.Column{Name: "processed"}, Value: gorm.Expr("queue_stats_minute.processed + ?", d.Processed)},
			{Column: clause.Column{Name: "failed"}, Value: gorm.Expr("queue_stats_minute.failed + ?", d.Failed)},
			{Column: clause.Column{Name: "enqueued"}, Value: gorm.Expr("queue_stats_minute.enqueued + ?", d.Enqueued)},
			{Column: clause.Column{Name: "latency_sum"}, Value: gorm.Expr("queue_stats_minute.latency_sum + ?", d.LatencySum)},
			{Column: clause.Column{Name: "latency_count"}, Value: gorm.Expr("queue_stats_minute.latency_count + ?", d.LatencyCount)},
		},
	}).Create(&d).Error
}
