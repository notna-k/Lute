package queue

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// StatsAggregator records per-minute counters for each queue.
type StatsAggregator struct {
	rdb *redis.Client
}

func NewStatsAggregator(rdb *redis.Client) *StatsAggregator {
	return &StatsAggregator{rdb: rdb}
}

func statsKey(queueName string, minute int64) string {
	return fmt.Sprintf("stats:%s:%d", queueName, minute)
}

func currentMinute() int64 {
	return time.Now().Unix() / 60
}

// RecordProcessed increments the processed counter for a queue.
func (s *StatsAggregator) RecordProcessed(ctx context.Context, queueName string, latencyMs int64) {
	key := statsKey(queueName, currentMinute())
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, "processed", 1)
	pipe.HIncrBy(ctx, key, "latency_sum", latencyMs)
	pipe.HIncrBy(ctx, key, "latency_count", 1)
	pipe.Expire(ctx, key, 2*time.Hour)
	pipe.Exec(ctx) //nolint:errcheck
}

// RecordFailed increments the failed counter for a queue.
func (s *StatsAggregator) RecordFailed(ctx context.Context, queueName string) {
	key := statsKey(queueName, currentMinute())
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, "failed", 1)
	pipe.Expire(ctx, key, 2*time.Hour)
	pipe.Exec(ctx) //nolint:errcheck
}

// RecordEnqueued increments the enqueued counter for a queue.
func (s *StatsAggregator) RecordEnqueued(ctx context.Context, queueName string) {
	key := statsKey(queueName, currentMinute())
	pipe := s.rdb.Pipeline()
	pipe.HIncrBy(ctx, key, "enqueued", 1)
	pipe.Expire(ctx, key, 2*time.Hour)
	pipe.Exec(ctx) //nolint:errcheck
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
	now := currentMinute()
	result := make([]QueueStats, 0, minutes)

	for i := minutes - 1; i >= 0; i-- {
		minute := now - int64(i)
		key := statsKey(queueName, minute)
		vals, err := s.rdb.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}

		qs := QueueStats{Minute: minute * 60}
		qs.Processed = parseInt64(vals["processed"])
		qs.Failed = parseInt64(vals["failed"])
		qs.Enqueued = parseInt64(vals["enqueued"])

		latencySum := parseInt64(vals["latency_sum"])
		latencyCount := parseInt64(vals["latency_count"])
		if latencyCount > 0 {
			qs.AvgLatencyMs = float64(latencySum) / float64(latencyCount)
		}

		result = append(result, qs)
	}
	return result, nil
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
