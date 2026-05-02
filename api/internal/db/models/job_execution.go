package models

import "time"

// JobExecution records the outcome of a job run, including references to
// log files stored on the worker that executed it.
type JobExecution struct {
	BaseModel
	JobID            string    `json:"job_id"`
	WorkerID         string    `json:"worker_id"`
	Queue            string    `json:"queue"`
	Type             string    `json:"type"`
	Success          bool      `json:"success"`
	Error            string    `json:"error,omitempty"`
	ElapsedMs        int64     `json:"elapsed_ms"`
	LogFile          string    `json:"log_file,omitempty"`
	ExecutionLogFile string    `json:"execution_log_file,omitempty"`
	FinishedAt       time.Time `json:"finished_at"`
}
