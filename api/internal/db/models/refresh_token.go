package models

import "github.com/lute/api/internal/db/id"

// RefreshToken is one token in a session family (one login). A refresh marks it used
// and issues the next; presenting a used token again revokes the whole family, since
// that means it was stolen. Only sha256(token) is stored.
type RefreshToken struct {
	BaseModel
	UserID    id.ID  `json:"user_id" gorm:"column:user_id;size:24;index:idx_refresh_tokens_user"`
	FamilyID  id.ID  `json:"family_id" gorm:"column:family_id;size:24;index:idx_refresh_tokens_family"`
	TokenHash string `json:"-" gorm:"column:token_hash;uniqueIndex:idx_refresh_tokens_hash;size:64"`
	ExpiresAt int64  `json:"expires_at" gorm:"column:expires_at;index:idx_refresh_tokens_expires"`
	UsedAt    *int64 `json:"used_at,omitempty" gorm:"column:used_at"`
	RevokedAt *int64 `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
	UserAgent string `json:"user_agent" gorm:"column:user_agent;default:''"`
	IP        string `json:"ip" gorm:"column:ip;default:''"`
}

func (*RefreshToken) TableName() string { return "refresh_tokens" }
