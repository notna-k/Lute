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

type CommandRepository struct {
	*Repository
}

func NewCommandRepository(db *mongo.Database) *CommandRepository {
	return &CommandRepository{
		Repository: NewRepository(db, connection.CollectionCommands),
	}
}

func (r *CommandRepository) Create(ctx context.Context, cmd *models.Command) error {
	cmd.BeforeCreate()
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	_, err := r.Collection.InsertOne(ctx, cmd)
	return err
}

func (r *CommandRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Command, error) {
	var cmd models.Command
	err := r.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&cmd)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

// GetPendingByWorkerID returns all pending commands for a worker, ordered by creation time
func (r *CommandRepository) GetPendingByWorkerID(ctx context.Context, workerID primitive.ObjectID) ([]*models.Command, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.Collection.Find(ctx, bson.M{
		"worker_id": workerID,
		"status":    "pending",
	}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var commands []*models.Command
	if err := cursor.All(ctx, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

// GetByWorkerID returns all commands for a worker, ordered by creation time (newest first)
func (r *CommandRepository) GetByWorkerID(ctx context.Context, workerID primitive.ObjectID, limit int64) ([]*models.Command, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cursor, err := r.Collection.Find(ctx, bson.M{"worker_id": workerID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var commands []*models.Command
	if err := cursor.All(ctx, &commands); err != nil {
		return nil, err
	}
	return commands, nil
}

// UpdateResult updates the status and optionally the output/error of a command
func (r *CommandRepository) UpdateResult(ctx context.Context, id primitive.ObjectID, status string, output string, exitCode int, errMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"output":     output,
			"exit_code":  exitCode,
			"error":      errMsg,
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

// MarkRunning marks a command as running
func (r *CommandRepository) MarkRunning(ctx context.Context, id primitive.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"status":     "running",
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}
