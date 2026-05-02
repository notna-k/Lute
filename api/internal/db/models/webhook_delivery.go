package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// WebhookDelivery records an outbound webhook attempt for a run-level event.
// The dispatcher polls rows with Status == "pending" and NextRetryAt <= now.
type WebhookDelivery struct {
	BaseModel
	RunID           id.ID      `json:"run_id"`
	JobID           string     `json:"job_id"`
	UserID          id.ID      `json:"user_id"`
	Event           string     `json:"event"`
	URL             string     `json:"url"`
	Payload         []byte     `json:"-"`
	Signature       string     `json:"signature"`
	SignedTimestamp int64      `json:"signed_timestamp"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"max_attempts"`
	NextRetryAt     time.Time  `json:"next_retry_at"`
	LastError       string     `json:"last_error,omitempty"`
	ResponseStatus  int        `json:"response_status,omitempty"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
}
