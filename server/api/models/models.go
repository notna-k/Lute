package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

// BeforeCreate sets timestamps before creation
func (b *BaseModel) BeforeCreate() {
	now := time.Now()
	if b.ID.IsZero() {
		b.ID = primitive.NewObjectID()
	}
	b.CreatedAt = now
	b.UpdatedAt = now
}

// BeforeUpdate sets updated timestamp
func (b *BaseModel) BeforeUpdate() {
	b.UpdatedAt = time.Now()
}

// User represents a user in the system
type User struct {
	BaseModel        `bson:",inline"`
	Email       string `json:"email" bson:"email"`
	DisplayName string `json:"display_name" bson:"display_name"`
	FirebaseUID string `json:"firebase_uid" bson:"firebase_uid"`
}

// Machine represents a virtual machine with embedded agent data
type Machine struct {
	BaseModel        `bson:",inline"`
	UserID       primitive.ObjectID     `json:"user_id" bson:"user_id"`
	Name         string                 `json:"name" bson:"name"`
	Description  string                 `json:"description" bson:"description"`
	Status       string                 `json:"status" bson:"status"` // "pending", "registered", "alive", "dead"
	IsPublic     bool                   `json:"is_public" bson:"is_public"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	AgentIP      string                 `json:"agent_ip,omitempty" bson:"agent_ip,omitempty"`
	AgentVersion string                 `json:"agent_version,omitempty" bson:"agent_version,omitempty"`
	LastSeen       time.Time              `json:"last_seen,omitempty" bson:"last_seen,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"`
	HeartbeatRetry int                    `json:"-" bson:"heartbeat_retry"`
}

// Agent model has been removed - agent data is now embedded in Machine

// Command represents a queued command for an agent to execute
type Command struct {
	BaseModel        `bson:",inline"`
	MachineID primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	Command   string             `json:"command" bson:"command"`
	Args      []string           `json:"args,omitempty" bson:"args,omitempty"`
	Env       map[string]string  `json:"env,omitempty" bson:"env,omitempty"`
	Status    string             `json:"status" bson:"status"` // "pending", "running", "completed", "failed"
	Output    string             `json:"output,omitempty" bson:"output,omitempty"`
	ExitCode  int                `json:"exit_code" bson:"exit_code"`
	Error     string             `json:"error,omitempty" bson:"error,omitempty"`
}

// MachineConfig holds configuration for a machine/agent
type MachineConfig struct {
	BaseModel        `bson:",inline"`
	MachineID         primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	HeartbeatInterval int                `json:"heartbeat_interval" bson:"heartbeat_interval"` // seconds
	LogLevel          string             `json:"log_level" bson:"log_level"`                   // "debug", "info", "warn", "error"
	Extra             map[string]string  `json:"extra,omitempty" bson:"extra,omitempty"`
}

// UptimeSnapshot is a per-user snapshot of machine counts at a point in time (for dashboard uptime graph).
type UptimeSnapshot struct {
	UserID primitive.ObjectID `json:"user_id" bson:"user_id"`
	At     time.Time          `json:"at" bson:"at"`
	Alive  int                `json:"alive" bson:"alive"`
	Dead   int                `json:"dead" bson:"dead"`
	Total  int                `json:"total" bson:"total"`
}

// MachineSnapshot is a per-machine point-in-time snapshot (canonical metrics, same keys as Machine.Metrics).
// Only written when the machine is alive; gaps in the time-series represent downtime.
type MachineSnapshot struct {
	MachineID primitive.ObjectID     `json:"machine_id" bson:"machine_id"`
	At        time.Time              `json:"at" bson:"at"`
	Metrics   map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"` // cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb
}
