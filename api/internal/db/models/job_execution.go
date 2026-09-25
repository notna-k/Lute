package models

import (
	"github.com/lute/api/internal/db/types"
)

type JobExecution struct {
	BaseModel
	JobID            string          `json:"job_id" gorm:"uniqueIndex"`
	WorkerID         string          `json:"worker_id"`
	Queue            string          `json:"queue"`
	Type             string          `json:"type"`
	Success          bool            `json:"success"`
	Error            string          `json:"error,omitempty"`
	ElapsedMs        int64           `json:"elapsed_ms"`
	LogFile          string          `json:"log_file,omitempty"`
	ExecutionLogFile string          `json:"execution_log_file,omitempty"`
	FinishedAt       types.MilliTime `json:"finished_at" gorm:"column:finished_at"`
}

func (*JobExecution) TableName() string { return "job_executions" }
