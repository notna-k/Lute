package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/lute/api/models"
)

type AgentRepository struct {
	*Repository
}

func NewAgentRepository(db *mongo.Database) *AgentRepository {
	return &AgentRepository{
		Repository: NewRepository(db, "agents"),
	}
}

func (r *AgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	agent.BeforeCreate()
	_, err := r.Collection.InsertOne(ctx, agent)
	return err
}

func (r *AgentRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*models.Agent, error) {
	var agent models.Agent
	err := r.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepository) GetByAgentID(ctx context.Context, agentID string) (*models.Agent, error) {
	var agent models.Agent
	err := r.Collection.FindOne(ctx, bson.M{"agent_id": agentID}).Decode(&agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepository) GetByMachineID(ctx context.Context, machineID primitive.ObjectID) (*models.Agent, error) {
	var agent models.Agent
	err := r.Collection.FindOne(ctx, bson.M{"machine_id": machineID}).Decode(&agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *AgentRepository) UpdateStatus(ctx context.Context, agentID string, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"last_seen":  time.Now(),
			"updated_at": bson.M{"$currentDate": true},
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"agent_id": agentID}, update)
	return err
}

func (r *AgentRepository) UpdateLastSeen(ctx context.Context, agentID string) error {
	update := bson.M{
		"$set": bson.M{
			"last_seen":  time.Now(),
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"agent_id": agentID}, update)
	return err
}

func (r *AgentRepository) UpdateMetrics(ctx context.Context, agentID string, metrics map[string]string) error {
	update := bson.M{
		"$set": bson.M{
			"metrics":    metrics,
			"updated_at": time.Now(),
		},
	}
	_, err := r.Collection.UpdateOne(ctx, bson.M{"agent_id": agentID}, update)
	return err
}

func (r *AgentRepository) ListConnected(ctx context.Context) ([]*models.Agent, error) {
	cursor, err := r.Collection.Find(ctx, bson.M{"status": "connected"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var agents []*models.Agent
	if err := cursor.All(ctx, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}
