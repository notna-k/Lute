package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// Worker is a registered compute node that runs the Lute agent and executes jobs.
type Worker struct {
	BaseModel
	UserID         id.ID                  `json:"user_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"` // "pending", "registered", "alive", "dead"
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	AgentIP        string                 `json:"agent_ip,omitempty"`
	AgentVersion   string                 `json:"agent_version,omitempty"`
	LastSeen       time.Time              `json:"last_seen,omitempty"`
	Metrics        map[string]interface{} `json:"metrics,omitempty"`
	HeartbeatRetry int                    `json:"-"`
}
