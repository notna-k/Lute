package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Run is a user-owned record of a programmatic job submission made through the
// public API. It stores ownership, webhook configuration, and idempotency keys
// alongside the job id used by the Redis queue and worker protocol.
type Run struct {
	BaseModel      `bson:",inline"`
	JobID          string             `json:"job_id" bson:"job_id"`
	UserID         primitive.ObjectID `json:"user_id" bson:"user_id"`
	APIKeyID       primitive.ObjectID `json:"api_key_id,omitempty" bson:"api_key_id,omitempty"`
	Queue          string             `json:"queue" bson:"queue"`
	Type           string             `json:"type" bson:"type"`
	IdempotencyKey string             `json:"idempotency_key,omitempty" bson:"idempotency_key,omitempty"`
	WebhookURL     string             `json:"webhook_url,omitempty" bson:"webhook_url,omitempty"`
	WebhookSecret  string             `json:"-" bson:"webhook_secret,omitempty"`
	WebhookEvents  []string           `json:"webhook_events,omitempty" bson:"webhook_events,omitempty"`
}
