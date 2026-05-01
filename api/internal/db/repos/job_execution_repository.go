package repos

import (
	"context"
	"fmt"
	"sort"
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

// JobExecutionListFilter narrows the job_executions query.
type JobExecutionListFilter struct {
	Queue  string
	Type   string
	Status string // "", "success", or "failed"
}

func buildJobExecutionMatch(f JobExecutionListFilter) bson.M {
	q := bson.M{}
	if f.Queue != "" {
		q["queue"] = f.Queue
	}
	if f.Type != "" {
		q["type"] = f.Type
	}
	switch f.Status {
	case "success":
		q["success"] = true
	case "failed":
		q["success"] = false
	}
	return q
}

// List returns executions matching the filter, sorted by finished_at (descending if sortDesc).
func (r *JobExecutionRepository) List(ctx context.Context, filter JobExecutionListFilter, offset, limit int64, sortDesc bool) ([]models.JobExecution, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	match := buildJobExecutionMatch(filter)

	total, err := r.Collection.CountDocuments(ctx, match)
	if err != nil {
		return nil, 0, fmt.Errorf("count job executions: %w", err)
	}

	order := 1
	if sortDesc {
		order = -1
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "finished_at", Value: order}}).
		SetSkip(offset).
		SetLimit(limit)

	cur, err := r.Collection.Find(ctx, match, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("find job executions: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []models.JobExecution
	if err := cur.All(ctx, &out); err != nil {
		return nil, 0, err
	}
	if out == nil {
		out = []models.JobExecution{}
	}
	return out, total, nil
}

// DistinctQueuesAndTypes returns sorted unique queue and type values for filter UI.
func (r *JobExecutionRepository) DistinctQueuesAndTypes(ctx context.Context) (queues, types []string, err error) {
	rawQ, err := r.Collection.Distinct(ctx, "queue", bson.M{})
	if err != nil {
		return nil, nil, err
	}
	rawT, err := r.Collection.Distinct(ctx, "type", bson.M{})
	if err != nil {
		return nil, nil, err
	}
	queues = distinctStrings(rawQ)
	types = distinctStrings(rawT)
	sort.Strings(queues)
	sort.Strings(types)
	return queues, types, nil
}

func distinctStrings(raw []interface{}) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, v := range raw {
		s, _ := v.(string)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
