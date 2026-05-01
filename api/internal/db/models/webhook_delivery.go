package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebhookDelivery records an outbound webhook attempt for a run-level event.
// The dispatcher polls rows with Status == "pending" and NextRetryAt <= now.
type WebhookDelivery struct {
	BaseModel       `bson:",inline"`
	RunID           primitive.ObjectID `json:"run_id" bson:"run_id"`
	JobID           string             `json:"job_id" bson:"job_id"`
	UserID          primitive.ObjectID `json:"user_id" bson:"user_id"`
	Event           string             `json:"event" bson:"event"`
	URL             string             `json:"url" bson:"url"`
	Payload         []byte             `json:"-" bson:"payload"`
	Signature       string             `json:"signature" bson:"signature"`
	SignedTimestamp int64              `json:"signed_timestamp" bson:"signed_timestamp"`
	Status          string             `json:"status" bson:"status"`
	Attempts        int                `json:"attempts" bson:"attempts"`
	MaxAttempts     int                `json:"max_attempts" bson:"max_attempts"`
	NextRetryAt     time.Time          `json:"next_retry_at" bson:"next_retry_at"`
	LastError       string             `json:"last_error,omitempty" bson:"last_error,omitempty"`
	ResponseStatus  int                `json:"response_status,omitempty" bson:"response_status,omitempty"`
	DeliveredAt     *time.Time         `json:"delivered_at,omitempty" bson:"delivered_at,omitempty"`
}
