package repos

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
)

type JobExecutionRepository struct {
	*Repository
}

func NewJobExecutionRepository(db *mongo.Database) *JobExecutionRepository {
	return &JobExecutionRepository{
		Repository: NewRepository(db, connection.CollectionJobExecutions),
	}
}

// Upsert creates or replaces the execution record for a given job_id.
// The last result wins (e.g. after a retry the newer execution overwrites).
func (r *JobExecutionRepository) Upsert(ctx context.Context, exec *models.JobExecution) error {
	now := time.Now()
	exec.UpdatedAt = now
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = now
	}

	filter := bson.M{"job_id": exec.JobID}
	update := bson.M{"$set": exec}
	opts := options.Update().SetUpsert(true)
	_, err := r.Collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetByJobID retrieves the execution record for a job.
func (r *JobExecutionRepository) GetByJobID(ctx context.Context, jobID string) (*models.JobExecution, error) {
	var exec models.JobExecution
	err := r.Collection.FindOne(ctx, bson.M{"job_id": jobID}).Decode(&exec)
	if err != nil {
		return nil, err
	}
	return &exec, nil
}
