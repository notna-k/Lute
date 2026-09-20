package models

import "github.com/lute/api/internal/db/id"

// WorkerConfig holds configuration for a worker agent.
type WorkerConfig struct {
	BaseModel
	WorkerID          id.ID             `json:"worker_id"`
	HeartbeatInterval int               `json:"heartbeat_interval"` // seconds
	LogLevel          string            `json:"log_level"`            // "debug", "info", "warn", "error"
	Extra             map[string]string `json:"extra,omitempty"`
}
