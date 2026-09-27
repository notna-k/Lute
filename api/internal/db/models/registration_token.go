package models

import (
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// RegistrationToken lets agents enrol themselves. Only its hash and a display prefix are stored.
type RegistrationToken struct {
	BaseModel
	Name       string           `json:"name"`
	Prefix     string           `json:"prefix"`
	TokenHash  string           `json:"-" gorm:"uniqueIndex;not null"`
	CreatedBy  id.ID            `json:"created_by" gorm:"size:24;not null"`
	LastUsedAt *types.MilliTime `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	RevokedAt  *types.MilliTime `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
}

func (*RegistrationToken) TableName() string { return "registration_tokens" }
