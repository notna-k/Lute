package models

import (
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// WorkerSnapshot is a sampled metrics series for workers.
type WorkerSnapshot struct {
	ID       uint64                 `json:"-" gorm:"primaryKey;autoIncrement"`
	WorkerID id.ID                  `json:"worker_id" gorm:"column:worker_id;size:24"`
	At       types.MilliTime        `json:"at" gorm:"column:at"`
	Metrics  map[string]interface{} `json:"metrics,omitempty" gorm:"serializer:json"`
}

func (*WorkerSnapshot) TableName() string { return "worker_snapshots" }
