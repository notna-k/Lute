package models

import "github.com/lute/api/internal/db/id"

// Run is the user-facing record of a build; JobID links it to the queue.
type Run struct {
	BaseModel
	JobID    string `json:"job_id" gorm:"uniqueIndex"`
	UserID   id.ID  `json:"user_id" gorm:"size:24;not null"`
	APIKeyID id.ID  `json:"api_key_id,omitempty" gorm:"size:24"`
	Queue    string `json:"queue"`
	Type     string `json:"type"`
	// JobSlug is empty for runs not started from a definition.
	JobSlug     string `json:"job_slug,omitempty" gorm:"column:job_slug;size:191;index"`
	Environment string `json:"environment,omitempty" gorm:"column:environment;size:64"`
	// Params keep a build reproducible after its definition changes. Never holds secrets.
	Params map[string]string `json:"params,omitempty" gorm:"column:params;serializer:json"`
	AdHoc  bool              `json:"ad_hoc,omitempty" gorm:"column:ad_hoc;index"`
	// ParamSchema is the schema an ad-hoc build ran with, since the definition cannot explain it.
	ParamSchema    []ParameterField `json:"param_schema,omitempty" gorm:"column:param_schema;serializer:json"`
	IdempotencyKey string           `json:"idempotency_key,omitempty" gorm:"size:191"`
	WebhookURL     string           `json:"webhook_url,omitempty"`
	WebhookSecret  string           `json:"-"`
	WebhookEvents  []string         `json:"webhook_events,omitempty" gorm:"serializer:json"`
}

func (*Run) TableName() string { return "runs" }
