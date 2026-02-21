package services

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/repository"
)

// Canonical metric keys (must match heartbeat_checker and agent).
var machineSnapshotMetricKeys = []string{"cpu_load", "mem_usage_mb", "disk_used_gb", "disk_total_gb"}

// MachineSnapshotJob runs periodically to record per-machine snapshots (status + canonical metrics).
type MachineSnapshotJob struct {
	machineRepo  *repository.MachineRepository
	snapshotRepo *repository.MachineSnapshotRepository
	interval     time.Duration
}

// NewMachineSnapshotJob creates a new MachineSnapshotJob. interval is the time between snapshots (e.g. 5*time.Minute).
func NewMachineSnapshotJob(machineRepo *repository.MachineRepository, snapshotRepo *repository.MachineSnapshotRepository, interval time.Duration) *MachineSnapshotJob {
	return &MachineSnapshotJob{
		machineRepo:  machineRepo,
		snapshotRepo: snapshotRepo,
		interval:     interval,
	}
}

// Run runs the job in a loop until ctx is cancelled. Call from a goroutine.
func (j *MachineSnapshotJob) Run(ctx context.Context) {
	log.Printf("machine snapshot: job started (interval %s)", j.interval)
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Printf("machine snapshot: job stopped")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *MachineSnapshotJob) runOnce(ctx context.Context) {
	now := time.Now()
	// Only snapshot alive machines; gaps in the time-series represent downtime.
	machines, err := j.machineRepo.ListByStatus(ctx, "alive")
	if err != nil {
		log.Printf("machine snapshot: list machines failed: %v", err)
		return
	}
	log.Printf("machine snapshot: run once at %s, %d alive machines", now.Format(time.RFC3339), len(machines))
	written := 0
	for _, m := range machines {
		metrics := canonicalMetricsFrom(m.Metrics)
		if err := j.snapshotRepo.Insert(ctx, m.ID, now, metrics); err != nil {
			log.Printf("machine snapshot: insert for machine %s: %v", m.ID.Hex(), err)
			continue
		}
		written++
	}
	if written > 0 {
		log.Printf("machine snapshot: wrote %d alive snapshots", written)
	} else if len(machines) == 0 {
		log.Printf("machine snapshot: no alive machines")
	} else {
		log.Printf("machine snapshot: wrote 0/%d (all inserts failed)", len(machines))
	}
}

// canonicalMetricsFrom returns a map with exactly the canonical keys (same shape as Machine.Metrics). Missing keys get 0.
func canonicalMetricsFrom(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(machineSnapshotMetricKeys))
	for _, k := range machineSnapshotMetricKeys {
		v := 0.0
		if m != nil {
			if x, ok := m[k]; ok && x != nil {
				switch val := x.(type) {
				case float64:
					v = val
				case int:
					v = float64(val)
				case int64:
					v = float64(val)
				}
			}
		}
		out[k] = v
	}
	return out
}
