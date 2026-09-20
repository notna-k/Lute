package queuejob

import (
	"encoding/json"
	"time"
)

// Job is the JSON envelope persisted in queue_slots.payload.
type Job struct {
	ID         string            `json:"id"`
	Queue      string            `json:"queue"`
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Status     string            `json:"status"`
	Attempts   int               `json:"attempts"`
	MaxRetries int               `json:"max_retries"`
	TimeoutSec int               `json:"timeout_sec"`
	DependsOn  []string          `json:"depends_on,omitempty"`
	Error      string            `json:"error,omitempty"`
	WorkerID   string            `json:"worker_id,omitempty"`
	EnqueuedAt int64             `json:"enqueued_at"`
	StartedAt  int64             `json:"started_at,omitempty"`
	DoneAt     int64             `json:"done_at,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
	Selector   map[string]string `json:"selector,omitempty"`
}

// EnqueueOpts are options when enqueuing a job.
type EnqueueOpts struct {
	Priority   float64
	Delay      time.Duration
	MaxRetries int
	TimeoutSec int
}
