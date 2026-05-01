package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WorkerSnapshot is a per-worker point-in-time snapshot (canonical metrics, same keys as Worker.Metrics).
// Only written when the worker is alive; gaps in the time-series represent downtime.
type WorkerSnapshot struct {
	WorkerID primitive.ObjectID     `json:"worker_id" bson:"worker_id"`
	At       time.Time              `json:"at" bson:"at"`
	Metrics  map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"` // cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb
}
