package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// APIKey represents a programmatic access credential owned by a user.
// The plaintext token is returned only once at creation time; only Prefix
// (for lookup) and Hash (for comparison) are persisted.
type APIKey struct {
	BaseModel  `bson:",inline"`
	UserID     primitive.ObjectID `json:"user_id" bson:"user_id"`
	Name       string             `json:"name" bson:"name"`
	Prefix     string             `json:"prefix" bson:"prefix"`
	Hash       string             `json:"-" bson:"hash"`
	LastUsedAt *time.Time         `json:"last_used_at,omitempty" bson:"last_used_at,omitempty"`
	RevokedAt  *time.Time         `json:"revoked_at,omitempty" bson:"revoked_at,omitempty"`
}
