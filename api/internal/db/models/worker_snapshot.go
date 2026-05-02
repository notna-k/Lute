package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// WorkerSnapshot is a per-worker point-in-time snapshot (canonical metrics, same keys as Worker.Metrics).
// Only written when the worker is alive; gaps in the time-series represent downtime.
type WorkerSnapshot struct {
	WorkerID id.ID                  `json:"worker_id"`
	At       time.Time              `json:"at"`
	Metrics  map[string]interface{} `json:"metrics,omitempty"` // cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb
}
