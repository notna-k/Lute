package models

import (
	"gorm.io/gorm"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

type Worker struct {
	BaseModel
	UserID       id.ID                  `json:"user_id,omitempty" gorm:"size:24"`
	Name         string                 `json:"name" gorm:"not null"`
	Description  string                 `json:"description"`
	Status       enums.WorkerStatus     `json:"status" gorm:"type:varchar(32);not null"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" gorm:"serializer:json"`
	AgentVersion string                 `json:"agent_version,omitempty"`
	Protocol     int32                  `json:"protocol,omitempty"`
	Engine       *Engine                `json:"engine,omitempty" gorm:"serializer:json"`
	// SecretHash is sha256 of the secret the agent got from Register; it proves every Connect.
	SecretHash     string            `json:"-"`
	LastSeen       *types.MilliTime  `json:"last_seen,omitempty" gorm:"column:last_seen"`
	Metrics        map[string]any    `json:"metrics,omitempty" gorm:"serializer:json"`
	Labels         map[string]string `json:"labels,omitempty" gorm:"serializer:json"`
	HeartbeatRetry int               `json:"-" gorm:"column:heartbeat_retry;default:0"`
	// Outdated is computed per response: the agent is older than core.
	Outdated bool `json:"outdated,omitempty" gorm:"-"`
}

// Engine is the container engine a worker runs jobs on, as the agent reported it.
type Engine struct {
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	Rootless    bool   `json:"rootless"`
	MemoryLimit bool   `json:"memory_limit"`
	CPULimit    bool   `json:"cpu_limit"`
	PidsLimit   bool   `json:"pids_limit"`
}

func (*Worker) TableName() string { return "workers" }

func (w *Worker) BeforeCreate(tx *gorm.DB) error {
	return w.BaseModel.BeforeCreate(tx)
}
