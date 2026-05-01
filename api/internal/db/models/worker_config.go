package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WorkerConfig holds configuration for a worker agent.
type WorkerConfig struct {
	BaseModel         `bson:",inline"`
	WorkerID          primitive.ObjectID `json:"worker_id" bson:"worker_id"`
	HeartbeatInterval int                `json:"heartbeat_interval" bson:"heartbeat_interval"` // seconds
	LogLevel          string             `json:"log_level" bson:"log_level"`                   // "debug", "info", "warn", "error"
	Extra             map[string]string  `json:"extra,omitempty" bson:"extra,omitempty"`
}
