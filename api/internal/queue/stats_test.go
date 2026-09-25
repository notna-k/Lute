package queue

import (
	"context"
	"testing"
)

func TestStatsAccumulateWithinAMinute(t *testing.T) {
	_, db := newTestEngine(t)
	s := NewStats(db)
	ctx := context.Background()

	s.RecordEnqueued(ctx, "build")
	s.RecordEnqueued(ctx, "build")
	s.RecordProcessed(ctx, "build", 100)
	s.RecordProcessed(ctx, "build", 300)
	s.RecordFailed(ctx, "build")
	s.RecordEnqueued(ctx, "other")

	series, err := s.GetTimeSeries(ctx, "build", 3)
	if err != nil {
		t.Fatalf("time series: %v", err)
	}
	if len(series) != 3 {
		t.Fatalf("got %d points, want 3", len(series))
	}
	// A minute boundary can split the writes; sum the window instead of reading the last point.
	var got QueueStats
	var latencySamples float64
	for _, p := range series {
		got.Enqueued += p.Enqueued
		got.Processed += p.Processed
		got.Failed += p.Failed
		latencySamples += p.AvgLatencyMs * float64(p.Processed)
	}
	if got.Enqueued != 2 || got.Processed != 2 || got.Failed != 1 {
		t.Fatalf("counters = %+v, want enqueued 2, processed 2, failed 1", got)
	}
	if latencySamples != 400 {
		t.Fatalf("latency total = %v, want 400", latencySamples)
	}
	if series[0].Minute >= series[2].Minute {
		t.Fatalf("series is not oldest first: %+v", series)
	}
}
