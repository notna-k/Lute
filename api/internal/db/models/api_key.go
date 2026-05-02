package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// APIKey represents a programmatic access credential owned by a user.
// The plaintext token is returned only once at creation time; only Prefix
// (for lookup) and Hash (for comparison) are persisted.
type APIKey struct {
	BaseModel
	UserID     id.ID      `json:"user_id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}
