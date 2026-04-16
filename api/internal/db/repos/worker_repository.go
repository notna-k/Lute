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

type WorkerRepository struct {
	*Repository
}

func NewWorkerRepository(db *mongo.Database) *WorkerRepository {
	return &WorkerRepository{
		Repository: NewRepository(db, connection.CollectionWorkers),
	}
}

func (r *WorkerRepository) Create(ctx context.Context, w *models.Worker) error {
	w.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, w)
	return err
}

func (r *WorkerRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Worker, error) {
	var w models.Worker
	err := r.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkerRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Worker, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"user_id": userID},
			{"user_id": primitive.NilObjectID},
		},
	}
	cursor, err := r.Collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.Worker
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WorkerRepository) Update(ctx context.Context, id primitive.ObjectID, w *models.Worker) error {
	w.BeforeUpdate()
	update := bson.M{
		"$set": w,
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *WorkerRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *WorkerRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *WorkerRepository) FindByAgentID(ctx context.Context, agentID string) (*models.Worker, error) {
	return nil, mongo.ErrNoDocuments
}

// GetByUserIDAndIP returns all workers owned by userID whose agent_ip matches ip.
// IP uniqueness is scoped per user because private-range addresses aren't globally unique.
func (r *WorkerRepository) GetByUserIDAndIP(ctx context.Context, userID primitive.ObjectID, ip string) ([]*models.Worker, error) {
	if ip == "" {
		return nil, nil
	}
	cursor, err := r.Collection.Find(ctx, bson.M{
		"user_id":  userID,
		"agent_ip": ip,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.Worker
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WorkerRepository) List(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*models.Worker, error) {
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.Worker
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WorkerRepository) UpdateLastSeen(ctx context.Context, workerID primitive.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"last_seen":  time.Now(),
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": workerID}, update)
	return err
}

func (r *WorkerRepository) UpdateMetrics(ctx context.Context, workerID primitive.ObjectID, metrics map[string]interface{}) error {
	update := bson.M{
		"$set": bson.M{
			"metrics":    metrics,
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": workerID}, update)
	return err
}

func (r *WorkerRepository) UpdateAgentInfo(ctx context.Context, workerID primitive.ObjectID, ipAddress string, version string) error {
	update := bson.M{
		"$set": bson.M{
			"agent_ip":      ipAddress,
			"agent_version": version,
			"last_seen":     time.Now(),
			"updated_at":    time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": workerID}, update)
	return err
}

func (r *WorkerRepository) ListByStatus(ctx context.Context, status string) ([]*models.Worker, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.Worker
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WorkerRepository) UpdateStatusAndLastSeen(ctx context.Context, workerID primitive.ObjectID, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"last_seen":  time.Now(),
			"updated_at": time.Now(),
		},
	}
	result, err := r.Collection.UpdateOne(ctx, bson.M{"_id": workerID}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *WorkerRepository) UpdateHeartbeat(ctx context.Context, workerID primitive.ObjectID, metrics map[string]interface{}) error {
	now := time.Now()
	set := bson.M{
		"status":          "alive",
		"heartbeat_retry": 0,
		"last_seen":       now,
		"updated_at":      now,
	}
	if len(metrics) > 0 {
		set["metrics"] = metrics
	}
	result, err := r.Collection.UpdateOne(ctx, bson.M{"_id": workerID}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *WorkerRepository) IncrementHeartbeatRetry(ctx context.Context, workerID primitive.ObjectID) (int, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Worker
	err := r.Collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": workerID},
		bson.M{
			"$inc": bson.M{"heartbeat_retry": 1},
			"$set": bson.M{"updated_at": time.Now()},
		},
		opts,
	).Decode(&updated)
	if err != nil {
		return 0, err
	}
	return updated.HeartbeatRetry, nil
}

// ListMonitored returns workers with status "alive" or "registered".
func (r *WorkerRepository) ListMonitored(ctx context.Context) ([]*models.Worker, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{
		"status": bson.M{"$in": []string{"alive", "registered"}},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []*models.Worker
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CountByUserIDAndStatusResult is one row from AggregateCountsByUserID.
type CountByUserIDAndStatusResult struct {
	UserID primitive.ObjectID `bson:"_id"`
	Alive  int                `bson:"alive"`
	Dead   int                `bson:"dead"`
	Total  int                `bson:"total"`
}

// AggregateCountsByUserID groups workers by user_id (excluding nil user_id) and counts alive, dead, total.
func (r *WorkerRepository) AggregateCountsByUserID(ctx context.Context) ([]CountByUserIDAndStatusResult, error) {
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"user_id": bson.M{"$ne": primitive.NilObjectID}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$user_id",
			"alive": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "alive"}}, 1, 0}}},
			"dead":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$status", "dead"}}, 1, 0}}},
			"total": bson.M{"$sum": 1},
		}}},
	}
	cursor, err := r.Collection.Aggregate(ctx, pipe)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []CountByUserIDAndStatusResult
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
