package services

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/repository"
)

// UptimeSnapshotJob runs periodically to record per-user machine counts for the dashboard uptime graph.
type UptimeSnapshotJob struct {
	machineRepo *repository.MachineRepository
	snapshotRepo *repository.UptimeSnapshotRepository
	interval   time.Duration
}

// NewUptimeSnapshotJob creates a new UptimeSnapshotJob. interval is the time between snapshots (e.g. 5*time.Minute).
func NewUptimeSnapshotJob(machineRepo *repository.MachineRepository, snapshotRepo *repository.UptimeSnapshotRepository, interval time.Duration) *UptimeSnapshotJob {
	return &UptimeSnapshotJob{
		machineRepo:  machineRepo,
		snapshotRepo: snapshotRepo,
		interval:     interval,
	}
}

// Run runs the job in a loop until ctx is cancelled. Call from a goroutine.
func (j *UptimeSnapshotJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	// Run once shortly after start, then on interval
	j.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *UptimeSnapshotJob) runOnce(ctx context.Context) {
	rows, err := j.machineRepo.AggregateCountsByUserID(ctx)
	if err != nil {
		log.Printf("uptime snapshot: aggregate failed: %v", err)
		return
	}
	now := time.Now()
	for _, row := range rows {
		if err := j.snapshotRepo.Insert(ctx, row.UserID, now, row.Alive, row.Dead, row.Total); err != nil {
			log.Printf("uptime snapshot: insert for user %s: %v", row.UserID.Hex(), err)
		}
	}
	if len(rows) > 0 {
		log.Printf("uptime snapshot: wrote %d user snapshots at %s", len(rows), now.Format(time.RFC3339))
	}
}
