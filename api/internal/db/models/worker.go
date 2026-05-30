package models

import (
	"gorm.io/gorm"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// Worker is a registered compute node that runs the Lute agent and executes jobs.
type Worker struct {
	BaseModel
	UserID           id.ID                  `json:"user_id,omitempty" gorm:"size:24"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Status           enums.WorkerStatus     `json:"status" gorm:"type:varchar(32);not null"`
	Metadata         map[string]interface{} `json:"metadata,omitempty" gorm:"serializer:json"`
	AgentIP          string                 `json:"agent_ip,omitempty"`
	AgentVersion     string                 `json:"agent_version,omitempty"`
	LastSeen         *types.MilliTime       `json:"last_seen,omitempty" gorm:"column:last_seen"`
	Metrics          map[string]interface{} `json:"metrics,omitempty" gorm:"serializer:json"`
	Labels           map[string]string      `json:"labels,omitempty" gorm:"serializer:json"`
	HeartbeatRetry   int                    `json:"-" gorm:"column:heartbeat_retry;default:0"`
}

func (*Worker) TableName() string { return "workers" }

func (w *Worker) BeforeCreate(tx *gorm.DB) error {
	return w.BaseModel.BeforeCreate(tx)
}
