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
	BaseModel
	Email       string `json:"email" bson:"email"`
	DisplayName string `json:"display_name" bson:"display_name"`
	FirebaseUID string `json:"firebase_uid" bson:"firebase_uid"`
}

// Machine represents a virtual machine
type Machine struct {
	BaseModel
	UserID      primitive.ObjectID `json:"user_id" bson:"user_id"`
	Name        string             `json:"name" bson:"name"`
	Description string             `json:"description" bson:"description"`
	Status      string             `json:"status" bson:"status"` // "running", "stopped", "pending"
	IsPublic    bool               `json:"is_public" bson:"is_public"`
	AgentID     string             `json:"agent_id,omitempty" bson:"agent_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// Agent represents a Go agent running on a VM
type Agent struct {
	BaseModel
	MachineID primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	AgentID   string             `json:"agent_id" bson:"agent_id"`
	Status    string             `json:"status" bson:"status"` // "connected", "disconnected", "registered", "running", "stopped"
	LastSeen  time.Time          `json:"last_seen" bson:"last_seen"`
	IPAddress string             `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	Version   string             `json:"version,omitempty" bson:"version,omitempty"`
	Metrics   map[string]string  `json:"metrics,omitempty" bson:"metrics,omitempty"`
}

// Command represents a queued command for an agent to execute
type Command struct {
	BaseModel
	MachineID primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	AgentID   string             `json:"agent_id" bson:"agent_id"`
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
	BaseModel
	MachineID         primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	HeartbeatInterval int                `json:"heartbeat_interval" bson:"heartbeat_interval"` // seconds
	LogLevel          string             `json:"log_level" bson:"log_level"`                   // "debug", "info", "warn", "error"
	Extra             map[string]string  `json:"extra,omitempty" bson:"extra,omitempty"`
}

