package models

import (
	"time"

	"github.com/lute/api/internal/db/id"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        id.ID     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate sets timestamps before creation
func (b *BaseModel) BeforeCreate() {
	now := time.Now()
	if b.ID.IsZero() {
		b.ID = id.New()
	}
	b.CreatedAt = now
	b.UpdatedAt = now
}

// BeforeUpdate sets updated timestamp
func (b *BaseModel) BeforeUpdate() {
	b.UpdatedAt = time.Now()
}
