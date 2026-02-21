package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/models"
)

type MachineRepository struct {
	*Repository
}

func NewMachineRepository(db *mongo.Database) *MachineRepository {
	return &MachineRepository{
		Repository: NewRepository(db, "machines"),
	}
}

func (r *MachineRepository) Create(ctx context.Context, machine *models.Machine) error {
	machine.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, machine)
	return err
}

func (r *MachineRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Machine, error) {
	var machine models.Machine
	err := r.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&machine)
	if err != nil {
		return nil, err
	}
	return &machine, nil
}

func (r *MachineRepository) GetByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.Machine, error) {
	// Include both user-owned machines AND agent-registered machines (zero user_id)
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
	defer cursor.Close(ctx)

	var machines []*models.Machine
	if err := cursor.All(ctx, &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

func (r *MachineRepository) GetPublic(ctx context.Context) ([]*models.Machine, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{"is_public": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var machines []*models.Machine
	if err := cursor.All(ctx, &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

func (r *MachineRepository) Update(ctx context.Context, id primitive.ObjectID, machine *models.Machine) error {
	machine.BeforeUpdate()
	update := bson.M{
		"$set": machine,
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *MachineRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.Collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *MachineRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": bson.M{"$currentDate": true},
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": id}, update)
	return err
}

func (r *MachineRepository) FindByAgentID(ctx context.Context, agentID string) (*models.Machine, error) {
	// This method is kept for backward compatibility but agentID is no longer stored
	// It's now a no-op that returns an error
	return nil, mongo.ErrNoDocuments
}

func (r *MachineRepository) List(ctx context.Context, filter bson.M, opts *options.FindOptions) ([]*models.Machine, error) {
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var machines []*models.Machine
	if err := cursor.All(ctx, &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

// UpdateLastSeen updates the last_seen timestamp for a machine
func (r *MachineRepository) UpdateLastSeen(ctx context.Context, machineID primitive.ObjectID) error {
	update := bson.M{
		"$set": bson.M{
			"last_seen":  time.Now(),
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": machineID}, update)
	return err
}

// UpdateMetrics updates the metrics for a machine
func (r *MachineRepository) UpdateMetrics(ctx context.Context, machineID primitive.ObjectID, metrics map[string]interface{}) error {
	update := bson.M{
		"$set": bson.M{
			"metrics":    metrics,
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": machineID}, update)
	return err
}

// UpdateAgentInfo updates agent-related fields (IP, version, last_seen)
func (r *MachineRepository) UpdateAgentInfo(ctx context.Context, machineID primitive.ObjectID, ipAddress string, version string) error {
	update := bson.M{
		"$set": bson.M{
			"agent_ip":      ipAddress,
			"agent_version": version,
			"last_seen":     time.Now(),
			"updated_at":    time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"_id": machineID}, update)
	return err
}

// ListByStatus returns all machines with a given status
func (r *MachineRepository) ListByStatus(ctx context.Context, status string) ([]*models.Machine, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{"status": status})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var machines []*models.Machine
	if err := cursor.All(ctx, &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

// UpdateStatusAndLastSeen updates both status and last_seen in a single operation
func (r *MachineRepository) UpdateStatusAndLastSeen(ctx context.Context, machineID primitive.ObjectID, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"last_seen":  time.Now(),
			"updated_at": time.Now(),
		},
	}
	result, err := r.Collection.UpdateOne(ctx, bson.M{"_id": machineID}, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// UpdateHeartbeat sets status to alive, resets heartbeat_retry, updates
// last_seen, and stores metrics in a single write.
func (r *MachineRepository) UpdateHeartbeat(ctx context.Context, machineID primitive.ObjectID, metrics map[string]interface{}) error {
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
	result, err := r.Collection.UpdateOne(ctx, bson.M{"_id": machineID}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// IncrementHeartbeatRetry atomically increments heartbeat_retry and returns
// the new value.
func (r *MachineRepository) IncrementHeartbeatRetry(ctx context.Context, machineID primitive.ObjectID) (int, error) {
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Machine
	err := r.Collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": machineID},
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

// ListMonitored returns machines with status "alive" or "registered".
func (r *MachineRepository) ListMonitored(ctx context.Context) ([]*models.Machine, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{
		"status": bson.M{"$in": []string{"alive", "registered"}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var machines []*models.Machine
	if err := cursor.All(ctx, &machines); err != nil {
		return nil, err
	}
	return machines, nil
}

// CountByUserIDAndStatusResult is one row from AggregateCountsByUserID.
type CountByUserIDAndStatusResult struct {
	UserID primitive.ObjectID `bson:"_id"`
	Alive  int                `bson:"alive"`
	Dead   int                `bson:"dead"`
	Total  int                `bson:"total"`
}

// AggregateCountsByUserID groups machines by user_id (excluding nil user_id) and counts alive, dead, total.
func (r *MachineRepository) AggregateCountsByUserID(ctx context.Context) ([]CountByUserIDAndStatusResult, error) {
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
	defer cursor.Close(ctx)

	var out []CountByUserIDAndStatusResult
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
