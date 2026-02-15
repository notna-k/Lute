package repository

import (
	"context"

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
	var machine models.Machine
	err := r.Collection.FindOne(ctx, bson.M{"agent_id": agentID}).Decode(&machine)
	if err != nil {
		return nil, err
	}
	return &machine, nil
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
