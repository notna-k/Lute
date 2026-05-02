package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/types"
)

// BaseModel embeds identifiers and timestamps for domain rows.
type BaseModel struct {
	ID        id.ID           `json:"id" gorm:"primaryKey;size:24"`
	CreatedAt types.MilliTime `json:"created_at" gorm:"column:created_at"`
	UpdatedAt types.MilliTime `json:"updated_at" gorm:"column:updated_at"`
}

// BeforeCreate initializes id and timestamps before insert (GORM callback).
func (b *BaseModel) BeforeCreate(_ *gorm.DB) error {
	if b.ID.IsZero() {
		b.ID = id.New()
	}
	now := types.NewMilliTime(time.Now())
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	b.UpdatedAt = now
	return nil
}

// BeforeUpdate refreshes UpdatedAt before write (GORM callback).
func (b *BaseModel) BeforeUpdate(_ *gorm.DB) error {
	b.UpdatedAt = types.NewMilliTime(time.Now())
	return nil
}
