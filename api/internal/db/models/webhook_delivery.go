package models

import (
	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// WebhookDelivery records an outbound webhook attempt for a run-level event.
type WebhookDelivery struct {
	BaseModel
	RunID           id.ID                       `json:"run_id" gorm:"size:24;not null"`
	JobID           string                      `json:"job_id"`
	UserID          id.ID                       `json:"user_id" gorm:"size:24;not null"`
	Event           string                     `json:"event"`
	URL             string                     `json:"url"`
	Payload         []byte                     `json:"-"`
	Signature       string                     `json:"signature"`
	SignedTimestamp int64                      `json:"signed_timestamp"`
	Status          enums.WebhookDeliveryStatus `json:"status" gorm:"type:varchar(32);not null"`
	Attempts        int                        `json:"attempts"`
	MaxAttempts     int                        `json:"max_attempts"`
	NextRetryAt     types.MilliTime             `json:"next_retry_at" gorm:"column:next_retry_at"`
	LastError       string                     `json:"last_error,omitempty"`
	ResponseStatus  int                        `json:"response_status"`
	DeliveredAt     *types.MilliTime           `json:"delivered_at,omitempty" gorm:"column:delivered_at"`
}

func (*WebhookDelivery) TableName() string { return "webhook_deliveries" }
