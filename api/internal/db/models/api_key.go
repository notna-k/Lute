package models

import (
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// Key scopes: who a key acts as and who can see it.
const (
	// KeyScopeAccount acts as the user who made it; only that user sees it.
	KeyScopeAccount = "account"
	// KeyScopeService acts as itself and belongs to the instance, so it outlives its creator.
	KeyScopeService = "service"
)

// APIKey stores only the token's public prefix and hash.
type APIKey struct {
	BaseModel
	Scope string `json:"scope" gorm:"size:16;not null;default:account"`
	// UserID is the owner an account key acts as; empty for a service key.
	UserID     id.ID            `json:"user_id,omitempty" gorm:"size:24;index:idx_api_keys_user_created,priority:1"`
	CreatedBy  id.ID            `json:"created_by" gorm:"size:24"`
	Name       string           `json:"name"`
	Prefix     string           `json:"prefix" gorm:"uniqueIndex"`
	Hash       string           `json:"-"`
	LastUsedAt *types.MilliTime `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	RevokedAt  *types.MilliTime `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
}

func (*APIKey) TableName() string { return "api_keys" }

func (k *APIKey) IsService() bool { return k.Scope == KeyScopeService }
