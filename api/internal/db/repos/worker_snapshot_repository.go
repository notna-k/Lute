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

// WorkerSnapshotRepository handles worker_snapshots collection.
type WorkerSnapshotRepository struct {
	*Repository
}

func NewWorkerSnapshotRepository(db *mongo.Database) *WorkerSnapshotRepository {
	return &WorkerSnapshotRepository{
		Repository: NewRepository(db, connection.CollectionWorkerSnapshots),
	}
}

// Insert inserts one snapshot for a worker. Only call for alive workers.
func (r *WorkerSnapshotRepository) Insert(ctx context.Context, workerID primitive.ObjectID, at time.Time, metrics map[string]interface{}) error {
	doc := &models.WorkerSnapshot{
		WorkerID: workerID,
		At:       at,
		Metrics:  metrics,
	}
	_, err := r.Collection.InsertOne(ctx, doc)
	return err
}

// GetByWorkerID returns snapshots for one worker since the given time, sorted by at ascending.
func (r *WorkerSnapshotRepository) GetByWorkerID(ctx context.Context, workerID primitive.ObjectID, since time.Time) ([]*models.WorkerSnapshot, error) {
	return r.GetByWorkerIDs(ctx, []primitive.ObjectID{workerID}, since)
}

// GetByWorkerIDs returns snapshots for any of the given worker IDs since the given time, sorted by at ascending.
func (r *WorkerSnapshotRepository) GetByWorkerIDs(ctx context.Context, workerIDs []primitive.ObjectID, since time.Time) ([]*models.WorkerSnapshot, error) {
	if len(workerIDs) == 0 {
		return nil, nil
	}
	filter := bson.M{
		"worker_id": bson.M{"$in": workerIDs},
		"at":        bson.M{"$gte": since},
	}
	opts := options.Find().SetSort(bson.M{"at": 1})
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.WorkerSnapshot
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
