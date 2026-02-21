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

// MachineSnapshotRepository handles machine_snapshots collection.
type MachineSnapshotRepository struct {
	*Repository
}

// NewMachineSnapshotRepository creates a new MachineSnapshotRepository.
func NewMachineSnapshotRepository(db *mongo.Database) *MachineSnapshotRepository {
	return &MachineSnapshotRepository{
		Repository: NewRepository(db, database.CollectionMachineSnapshots),
	}
}

// Insert inserts one snapshot for a machine. metrics must have same shape as Machine.Metrics (cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb).
func (r *MachineSnapshotRepository) Insert(ctx context.Context, machineID primitive.ObjectID, at time.Time, status string, metrics map[string]interface{}) error {
	doc := &models.MachineSnapshot{
		MachineID: machineID,
		At:        at,
		Status:    status,
		Metrics:   metrics,
	}
	_, err := r.Collection.InsertOne(ctx, doc)
	return err
}

// GetByMachineID returns snapshots for one machine since the given time, sorted by at ascending.
func (r *MachineSnapshotRepository) GetByMachineID(ctx context.Context, machineID primitive.ObjectID, since time.Time) ([]*models.MachineSnapshot, error) {
	return r.GetByMachineIDs(ctx, []primitive.ObjectID{machineID}, since)
}

// GetByMachineIDs returns snapshots for any of the given machine IDs since the given time, sorted by at ascending.
func (r *MachineSnapshotRepository) GetByMachineIDs(ctx context.Context, machineIDs []primitive.ObjectID, since time.Time) ([]*models.MachineSnapshot, error) {
	if len(machineIDs) == 0 {
		return nil, nil
	}
	filter := bson.M{
		"machine_id": bson.M{"$in": machineIDs},
		"at":         bson.M{"$gte": since},
	}
	opts := options.Find().SetSort(bson.M{"at": 1})
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []*models.MachineSnapshot
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}
