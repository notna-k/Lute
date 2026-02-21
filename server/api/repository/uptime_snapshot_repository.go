package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/database"
	"github.com/lute/api/models"
)

// UptimeSnapshotRepository handles uptime_snapshots collection.
type UptimeSnapshotRepository struct {
	*Repository
}

// NewUptimeSnapshotRepository creates a new UptimeSnapshotRepository.
func NewUptimeSnapshotRepository(db *mongo.Database) *UptimeSnapshotRepository {
	return &UptimeSnapshotRepository{
		Repository: NewRepository(db, database.CollectionUptimeSnapshots),
	}
}

// Insert inserts one snapshot for a user at the given time.
func (r *UptimeSnapshotRepository) Insert(ctx context.Context, userID primitive.ObjectID, at time.Time, alive, dead, total int) error {
	doc := &models.UptimeSnapshot{
		UserID: userID,
		At:     at,
		Alive:  alive,
		Dead:   dead,
		Total:  total,
	}
	_, err := r.Collection.InsertOne(ctx, doc)
	return err
}

// GetByUserID returns all snapshots for the user since the given time, ordered by at ascending.
func (r *UptimeSnapshotRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID, since time.Time) ([]*models.UptimeSnapshot, error) {
	filter := bson.M{
		"user_id": userID,
		"at":      bson.M{"$gte": since},
	}
	opts := options.Find().SetSort(bson.M{"at": 1})
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []*models.UptimeSnapshot
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
