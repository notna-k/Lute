package models

import (
	"gorm.io/gorm"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
)

// Command represents a queued command for an agent to execute.
type Command struct {
	BaseModel
	WorkerID id.ID                   `json:"worker_id" gorm:"column:worker_id;size:24;not null;index"`
	Command  string                  `json:"command"`
	Args     []string                `json:"args,omitempty" gorm:"serializer:json"`
	Env      map[string]string       `json:"env,omitempty" gorm:"serializer:json"`
	Status   enums.CommandStatus     `json:"status" gorm:"type:varchar(32);not null"`
	Output   string                  `json:"output,omitempty"`
	ExitCode int                     `json:"exit_code"`
	Error    string                  `json:"error,omitempty"`
}

func (*Command) TableName() string { return "commands" }

func (c *Command) BeforeCreate(tx *gorm.DB) error {
	if err := c.BaseModel.BeforeCreate(tx); err != nil {
		return err
	}
	if c.Status == "" {
		c.Status = enums.CommandPending
	}
	return nil
}
