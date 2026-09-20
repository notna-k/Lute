package models

import (
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// APIKey is a programmatic credential (prefix + hash only at rest).
type APIKey struct {
	BaseModel
	UserID     id.ID          `json:"user_id" gorm:"size:24;not null;index:idx_api_keys_user_created,priority:1"`
	Name       string         `json:"name"`
	Prefix     string         `json:"prefix" gorm:"uniqueIndex"`
	Hash       string         `json:"-"`
	LastUsedAt *types.MilliTime `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	RevokedAt  *types.MilliTime `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
}

func (*APIKey) TableName() string { return "api_keys" }
