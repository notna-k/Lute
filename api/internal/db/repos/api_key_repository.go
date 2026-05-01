package repos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
)

type APIKeyRepository struct {
	*Repository
}

func NewAPIKeyRepository(db *mongo.Database) *APIKeyRepository {
	return &APIKeyRepository{Repository: NewRepository(db, connection.CollectionAPIKeys)}
}

func (r *APIKeyRepository) Create(ctx context.Context, k *models.APIKey) error {
	k.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, k)
	return err
}

// GetByPrefix returns the non-revoked key matching the public prefix, or ErrNoDocuments.
func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	var k models.APIKey
	filter := bson.M{"prefix": prefix, "revoked_at": bson.M{"$exists": false}}
	if err := r.Collection.FindOne(ctx, filter).Decode(&k); err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *APIKeyRepository) ListByUser(ctx context.Context, userID primitive.ObjectID) ([]*models.APIKey, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.Collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []*models.APIKey
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Revoke marks the key as revoked; it is kept for audit purposes.
func (r *APIKeyRepository) Revoke(ctx context.Context, id, userID primitive.ObjectID) error {
	now := time.Now()
	res, err := r.Collection.UpdateOne(
		ctx,
		bson.M{"_id": id, "user_id": userID, "revoked_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"revoked_at": now, "updated_at": now}},
	)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// TouchUsed updates the last_used_at timestamp without blocking the request path if it fails.
func (r *APIKeyRepository) TouchUsed(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.Collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": bson.M{"last_used_at": now}},
	)
	return err
}
