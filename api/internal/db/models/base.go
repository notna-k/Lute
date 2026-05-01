package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

// BeforeCreate sets timestamps before creation
func (b *BaseModel) BeforeCreate() {
	now := time.Now()
	if b.ID.IsZero() {
		b.ID = primitive.NewObjectID()
	}
	b.CreatedAt = now
	b.UpdatedAt = now
}

// BeforeUpdate sets updated timestamp
func (b *BaseModel) BeforeUpdate() {
	b.UpdatedAt = time.Now()
}
