package models

import "github.com/lute/api/internal/db/id"

// Run is an API-visible record linking a queued job id to tenant config.
type Run struct {
	BaseModel
	JobID    string `json:"job_id" gorm:"uniqueIndex"`
	UserID   id.ID  `json:"user_id" gorm:"size:24;not null"`
	APIKeyID id.ID  `json:"api_key_id,omitempty" gorm:"size:24"`
	Queue    string `json:"queue"`
	Type     string `json:"type"`
	// JobSlug links a run to the JobDefinition that produced it. Empty for
	// ad-hoc runs (the hybrid model in PRODUCT.md).
	JobSlug string `json:"job_slug,omitempty" gorm:"column:job_slug;size:191;index"`
	// Environment is the resolved value of a job's `environment` parameter,
	// surfaced on the build list. Optional.
	Environment string `json:"environment,omitempty" gorm:"column:environment;size:64"`
	// Params are the resolved parameter values this build was triggered with
	// (env var name → value), so a build stays auditable and reproducible after
	// the definition moves on. Secrets are never stored here.
	Params map[string]string `json:"params,omitempty" gorm:"column:params;serializer:json"`
	// AdHoc marks a build whose parameter schema differed from the definition
	// synced from Git — edited in the panel workbench, not committed.
	AdHoc bool `json:"ad_hoc,omitempty" gorm:"column:ad_hoc;index"`
	// ParamSchema snapshots the schema an ad-hoc build actually ran with. The
	// stored definition cannot explain such a run, so without this the build is
	// unauditable. Empty for builds that ran the committed definition.
	ParamSchema    []ParameterField `json:"param_schema,omitempty" gorm:"column:param_schema;serializer:json"`
	IdempotencyKey string           `json:"idempotency_key,omitempty" gorm:"size:191"`
	WebhookURL     string           `json:"webhook_url,omitempty"`
	WebhookSecret  string           `json:"-"`
	WebhookEvents  []string         `json:"webhook_events,omitempty" gorm:"serializer:json"`
}

func (*Run) TableName() string { return "runs" }
