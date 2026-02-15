package repository

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// Repository provides a base repository interface
type Repository struct {
	Collection *mongo.Collection
}

// NewRepository creates a new repository instance
func NewRepository(db *mongo.Database, collectionName string) *Repository {
	return &Repository{
		Collection: db.Collection(collectionName),
	}
}

// HealthCheck verifies the repository connection
func (r *Repository) HealthCheck(ctx context.Context) error {
	return r.Collection.Database().Client().Ping(ctx, nil)
}

