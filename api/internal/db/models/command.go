package models

import "github.com/lute/api/internal/db/id"

// Command represents a queued command for an agent to execute
type Command struct {
	BaseModel
	WorkerID id.ID             `json:"worker_id"`
	Command  string             `json:"command"`
	Args     []string           `json:"args,omitempty"`
	Env      map[string]string  `json:"env,omitempty"`
	Status   string             `json:"status"` // "pending", "running", "completed", "failed"
	Output   string             `json:"output,omitempty"`
	ExitCode int                `json:"exit_code"`
	Error    string             `json:"error,omitempty"`
}
