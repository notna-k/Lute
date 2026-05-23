package models

import "github.com/lute/api/internal/db/id"

// Run is an API-visible record linking a queued job id to tenant config.
type Run struct {
	BaseModel
	JobID          string            `json:"job_id" gorm:"uniqueIndex"`
	UserID         id.ID             `json:"user_id" gorm:"size:24;not null"`
	APIKeyID       id.ID             `json:"api_key_id,omitempty" gorm:"size:24"`
	Queue          string            `json:"queue"`
	Type           string            `json:"type"`
	IdempotencyKey string            `json:"idempotency_key,omitempty" gorm:"size:191"`
	WebhookURL     string            `json:"webhook_url,omitempty"`
	WebhookSecret  string            `json:"-"`
	WebhookEvents  []string          `json:"webhook_events,omitempty" gorm:"serializer:json"`
}

func (*Run) TableName() string { return "runs" }
