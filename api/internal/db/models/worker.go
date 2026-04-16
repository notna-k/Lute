package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Worker is a registered compute node that runs the Lute agent and executes jobs.
type Worker struct {
	BaseModel      `bson:",inline"`
	UserID         primitive.ObjectID     `json:"user_id" bson:"user_id"`
	Name           string                 `json:"name" bson:"name"`
	Description    string                 `json:"description" bson:"description"`
	Status         string                 `json:"status" bson:"status"` // "pending", "registered", "alive", "dead"
	Metadata       map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	AgentIP        string                 `json:"agent_ip,omitempty" bson:"agent_ip,omitempty"`
	AgentVersion   string                 `json:"agent_version,omitempty" bson:"agent_version,omitempty"`
	LastSeen       time.Time              `json:"last_seen,omitempty" bson:"last_seen,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"`
	HeartbeatRetry int                    `json:"-" bson:"heartbeat_retry"`
}
