package models

import "time"

// JobExecution records the outcome of a job run, including references to
// log files stored on the worker that executed it.
type JobExecution struct {
	BaseModel        `bson:",inline"`
	JobID            string    `json:"job_id" bson:"job_id"`
	WorkerID         string    `json:"worker_id" bson:"worker_id"`
	Queue            string    `json:"queue" bson:"queue"`
	Type             string    `json:"type" bson:"type"`
	Success          bool      `json:"success" bson:"success"`
	Error            string    `json:"error,omitempty" bson:"error,omitempty"`
	ElapsedMs        int64     `json:"elapsed_ms" bson:"elapsed_ms"`
	LogFile          string    `json:"log_file,omitempty" bson:"log_file,omitempty"`
	ExecutionLogFile string    `json:"execution_log_file,omitempty" bson:"execution_log_file,omitempty"`
	FinishedAt       time.Time `json:"finished_at" bson:"finished_at"`
}
