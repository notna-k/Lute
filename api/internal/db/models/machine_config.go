package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MachineConfig holds configuration for a machine/agent
type MachineConfig struct {
	BaseModel         `bson:",inline"`
	MachineID         primitive.ObjectID `json:"machine_id" bson:"machine_id"`
	HeartbeatInterval int                `json:"heartbeat_interval" bson:"heartbeat_interval"` // seconds
	LogLevel          string             `json:"log_level" bson:"log_level"`                   // "debug", "info", "warn", "error"
	Extra             map[string]string  `json:"extra,omitempty" bson:"extra,omitempty"`
}
