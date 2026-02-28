package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MachineSnapshot is a per-machine point-in-time snapshot (canonical metrics, same keys as Machine.Metrics).
// Only written when the machine is alive; gaps in the time-series represent downtime.
type MachineSnapshot struct {
	MachineID primitive.ObjectID     `json:"machine_id" bson:"machine_id"`
	At        time.Time              `json:"at" bson:"at"`
	Metrics   map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"` // cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb
}
