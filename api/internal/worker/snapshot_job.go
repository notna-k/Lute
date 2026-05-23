package worker

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/internal/db/repos"
)

var workerSnapshotMetricKeys = []string{"cpu_load", "mem_usage_mb", "disk_used_gb", "disk_total_gb"}

// WorkerSnapshotJob runs periodically to record per-worker snapshots (status + canonical metrics).
type WorkerSnapshotJob struct {
	workerRepo   *repos.WorkerRepository
	snapshotRepo *repos.WorkerSnapshotRepository
	interval     time.Duration
}

func NewWorkerSnapshotJob(workerRepo *repos.WorkerRepository, snapshotRepo *repos.WorkerSnapshotRepository, interval time.Duration) *WorkerSnapshotJob {
	return &WorkerSnapshotJob{
		workerRepo:   workerRepo,
		snapshotRepo: snapshotRepo,
		interval:     interval,
	}
}

func (j *WorkerSnapshotJob) Run(ctx context.Context) {
	log.Printf("worker snapshot: job started (interval %s)", j.interval)
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	j.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker snapshot: job stopped")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *WorkerSnapshotJob) runOnce(ctx context.Context) {
	now := time.Now()
	cutoff := now.Add(-30 * 24 * time.Hour)
	if err := j.snapshotRepo.PruneOlderThan(ctx, cutoff); err != nil {
		log.Printf("worker snapshot: prune failed: %v", err)
	}
	list, err := j.workerRepo.ListByStatus(ctx, "alive")
	if err != nil {
		log.Printf("worker snapshot: list workers failed: %v", err)
		return
	}
	log.Printf("worker snapshot: run once at %s, %d alive workers", now.Format(time.RFC3339), len(list))
	written := 0
	for _, w := range list {
		metrics := canonicalMetricsFrom(w.Metrics)
		if err := j.snapshotRepo.Insert(ctx, w.ID, now, metrics); err != nil {
			log.Printf("worker snapshot: insert for worker %s: %v", w.ID.Hex(), err)
			continue
		}
		written++
	}
	if written > 0 {
		log.Printf("worker snapshot: wrote %d alive snapshots", written)
	} else if len(list) == 0 {
		log.Printf("worker snapshot: no alive workers")
	} else {
		log.Printf("worker snapshot: wrote 0/%d (all inserts failed)", len(list))
	}
}

func canonicalMetricsFrom(src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(workerSnapshotMetricKeys))
	for _, k := range workerSnapshotMetricKeys {
		if src == nil {
			out[k] = float64(0)
			continue
		}
		v, ok := src[k]
		if !ok || v == nil {
			out[k] = float64(0)
			continue
		}
		switch x := v.(type) {
		case float64:
			out[k] = x
		case int:
			out[k] = float64(x)
		case int64:
			out[k] = float64(x)
		default:
			out[k] = float64(0)
		}
	}
	return out
}
