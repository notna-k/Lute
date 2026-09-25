package publicapi

import (
	"encoding/json"
	"time"
)

type CreateRunRequest struct {
	Queue          string            `json:"queue" binding:"required"`
	Type           string            `json:"type" binding:"required"`
	Payload        json.RawMessage   `json:"payload"`
	Priority       float64           `json:"priority"`
	DelayMs        int64             `json:"delay_ms"`
	MaxRetries     int               `json:"max_retries"`
	TimeoutSec     int               `json:"timeout_sec"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
	Webhook        *WebhookConfig    `json:"webhook,omitempty"`
	Selector       map[string]string `json:"selector,omitempty"`
}

// WebhookConfig with an empty Secret gets a generated one, returned once on create.
type WebhookConfig struct {
	URL    string   `json:"url" binding:"required"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events,omitempty"`
}

type RunResponse struct {
	ID             string    `json:"id"`
	Queue          string    `json:"queue"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Priority       float64   `json:"priority,omitempty"`
	Attempts       int       `json:"attempts"`
	MaxRetries     int       `json:"max_retries"`
	TimeoutSec     int       `json:"timeout_sec"`
	Error          string    `json:"error,omitempty"`
	WorkerID       string    `json:"worker_id,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	WebhookURL     string    `json:"webhook_url,omitempty"`
	WebhookEvents  []string  `json:"webhook_events,omitempty"`
	EnqueuedAt     time.Time `json:"enqueued_at"`
	StartedAt      time.Time `json:"started_at,omitempty"`
	FinishedAt     time.Time `json:"finished_at,omitempty"`
	ElapsedMs      int64     `json:"elapsed_ms,omitempty"`
}

type CreateRunResponse struct {
	RunResponse
	WebhookSecret string `json:"webhook_secret,omitempty"`
}

type ListRunsResponse struct {
	Runs   []RunResponse `json:"runs"`
	Total  int64         `json:"total"`
	Offset int64         `json:"offset"`
	Limit  int64         `json:"limit"`
}
