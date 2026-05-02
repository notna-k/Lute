package models

import "github.com/lute/api/internal/db/id"

// Run is a user-owned record of a programmatic job submission made through the
// public API. It stores ownership, webhook configuration, and idempotency keys
// alongside the job id used by the SQLite-backed queue and worker protocol.
type Run struct {
	BaseModel
	JobID          string   `json:"job_id"`
	UserID         id.ID    `json:"user_id"`
	APIKeyID       id.ID    `json:"api_key_id,omitempty"`
	Queue          string   `json:"queue"`
	Type           string   `json:"type"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
	WebhookURL     string   `json:"webhook_url,omitempty"`
	WebhookSecret  string   `json:"-"`
	WebhookEvents  []string `json:"webhook_events,omitempty"`
}
