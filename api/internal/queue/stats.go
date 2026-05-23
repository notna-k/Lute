package queue

import (
	"context"

	"github.com/lute/api/internal/db/repos"
)

// QueueStats holds per-minute stats for a queue.
type QueueStats struct {
	Minute       int64   `json:"minute"`
	Processed    int64   `json:"processed"`
	Failed       int64   `json:"failed"`
	Enqueued     int64   `json:"enqueued"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// StatsAggregator records throughput/latency against the queue_stats_minute rows.
type StatsAggregator struct {
	r *repos.QueueStatsRepository
}

// NewStatsAggregator wraps QueueStatsRepository.
func NewStatsAggregator(r *repos.QueueStatsRepository) *StatsAggregator {
	return &StatsAggregator{r: r}
}

// RecordProcessed increments successful completions for current minute bucket.
func (s *StatsAggregator) RecordProcessed(ctx context.Context, queueName string, latencyMs int64) {
	if s != nil && s.r != nil {
		s.r.RecordProcessed(ctx, queueName, latencyMs)
	}
}

// RecordFailed increments failures for current minute bucket.
func (s *StatsAggregator) RecordFailed(ctx context.Context, queueName string) {
	if s != nil && s.r != nil {
		s.r.RecordFailed(ctx, queueName)
	}
}

// RecordEnqueued increments arrivals for current minute bucket.
func (s *StatsAggregator) RecordEnqueued(ctx context.Context, queueName string) {
	if s != nil && s.r != nil {
		s.r.RecordEnqueued(ctx, queueName)
	}
}

// GetTimeSeries returns per-minute counters for dashboards.
func (s *StatsAggregator) GetTimeSeries(ctx context.Context, queueName string, minutes int) ([]QueueStats, error) {
	if s == nil || s.r == nil {
		return nil, nil
	}
	rows, err := s.r.GetTimeSeries(ctx, queueName, minutes)
	if err != nil {
		return nil, err
	}
	out := make([]QueueStats, 0, len(rows))
	for _, rw := range rows {
		p := QueueStats{
			Minute:    rw.Minute,
			Processed: rw.Processed,
			Failed:    rw.Failed,
			Enqueued:  rw.Enqueued,
		}
		if rw.LatencyCount > 0 {
			p.AvgLatencyMs = float64(rw.LatencySum) / float64(rw.LatencyCount)
		}
		out = append(out, p)
	}
	return out, nil
}
