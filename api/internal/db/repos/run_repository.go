package repos

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
)

type RunRepository struct {
	*Repository
}

func NewRunRepository(db *mongo.Database) *RunRepository {
	return &RunRepository{Repository: NewRepository(db, connection.CollectionRuns)}
}

func (r *RunRepository) Create(ctx context.Context, run *models.Run) error {
	run.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, run)
	return err
}

// GetByJobID returns the ownership record for a given queue job id.
func (r *RunRepository) GetByJobID(ctx context.Context, jobID string) (*models.Run, error) {
	var run models.Run
	if err := r.Collection.FindOne(ctx, bson.M{"job_id": jobID}).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

// GetByIdempotency returns an existing run for (userID, idempotencyKey) or ErrNoDocuments.
func (r *RunRepository) GetByIdempotency(ctx context.Context, userID primitive.ObjectID, key string) (*models.Run, error) {
	var run models.Run
	filter := bson.M{"user_id": userID, "idempotency_key": key}
	if err := r.Collection.FindOne(ctx, filter).Decode(&run); err != nil {
		return nil, err
	}
	return &run, nil
}

// RunListFilter narrows the user-scoped run list.
type RunListFilter struct {
	UserID primitive.ObjectID
	Queue  string
	Type   string
}

// List returns the user's runs newest-first with total count for pagination.
func (r *RunRepository) List(ctx context.Context, f RunListFilter, offset, limit int64) ([]models.Run, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	match := bson.M{"user_id": f.UserID}
	if f.Queue != "" {
		match["queue"] = f.Queue
	}
	if f.Type != "" {
		match["type"] = f.Type
	}

	total, err := r.Collection.CountDocuments(ctx, match)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(offset).
		SetLimit(limit)

	cur, err := r.Collection.Find(ctx, match, opts)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = cur.Close(ctx) }()

	var runs []models.Run
	if err := cur.All(ctx, &runs); err != nil {
		return nil, 0, err
	}
	if runs == nil {
		runs = []models.Run{}
	}
	return runs, total, nil
}
