package repos

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type JobExecutionRepository struct {
	g *gorm.DB
}

func NewJobExecutionRepository(db *gorm.DB) *JobExecutionRepository {
	return &JobExecutionRepository{g: db}
}

func (r *JobExecutionRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *JobExecutionRepository) Upsert(ctx context.Context, exec *models.JobExecution) error {
	now := time.Now().UTC()
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = types.NewMilliTime(now)
	}
	exec.UpdatedAt = types.NewMilliTime(now)
	if exec.ID.IsZero() {
		exec.ID = id.New()
	}
	if exec.FinishedAt.IsZero() {
		exec.FinishedAt = types.NewMilliTime(now)
	}
	return r.q(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "job_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "worker_id", "queue", "type", "success",
			"error", "elapsed_ms", "log_file", "execution_log_file", "finished_at",
		}),
	}).Create(exec).Error
}

func (r *JobExecutionRepository) GetByJobID(ctx context.Context, jobID string) (*models.JobExecution, error) {
	var e models.JobExecution
	if err := r.q(ctx).Where("job_id = ?", jobID).First(&e).Error; err != nil {
		return nil, mapErr(err)
	}
	return &e, nil
}

// ListByJobIDs loads executions for many queue-job IDs in one query, keyed by
// job ID. Used by the job-definition handlers so rendering N jobs' build stats
// costs one query instead of one per build.
func (r *JobExecutionRepository) ListByJobIDs(ctx context.Context, jobIDs []string) (map[string]*models.JobExecution, error) {
	out := make(map[string]*models.JobExecution, len(jobIDs))
	if len(jobIDs) == 0 {
		return out, nil
	}
	var rows []models.JobExecution
	if err := r.q(ctx).Where("job_id IN ?", jobIDs).Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	for i := range rows {
		out[rows[i].JobID] = &rows[i]
	}
	return out, nil
}

// JobExecutionOrders is the closed set of orders the list endpoint accepts,
// keyed by the value the panel sends.
var JobExecutionOrders = map[string]string{
	"finished_at_desc": "finished_at DESC",
	"finished_at_asc":  "finished_at ASC",
	"elapsed_desc":     "elapsed_ms DESC",
	"elapsed_asc":      "elapsed_ms ASC",
}

// DefaultJobExecutionOrder is what an unknown or missing sort falls back to.
const DefaultJobExecutionOrder = "finished_at DESC"

type JobExecutionListFilter struct {
	// Queues and Types narrow to any of the given values; empty means all.
	Queues []string
	Types  []string
	Status string // "", "success", or "failed"
	// Search matches a job id, worker id or error message, case-insensitively.
	// The panel's Builds list offers one box over the three because an operator
	// arrives with one string and does not know which column it came from.
	Search string
}

// List returns one page of executions. sortOrder is one of the SQL fragments in
// JobExecutionOrders; anything else is rejected, so the caller may pass a query
// parameter straight through without opening an injection hole.
func (r *JobExecutionRepository) List(ctx context.Context, filter JobExecutionListFilter, offset, limit int64, sortOrder string) ([]models.JobExecution, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	q := r.q(ctx).Model(&models.JobExecution{})
	if len(filter.Queues) > 0 {
		q = q.Where("queue IN ?", filter.Queues)
	}
	if len(filter.Types) > 0 {
		q = q.Where("type IN ?", filter.Types)
	}
	switch filter.Status {
	case "success":
		q = q.Where("success = ?", true)
	case "failed":
		q = q.Where("success = ?", false)
	}
	if s := strings.TrimSpace(filter.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where(
			r.q(ctx).Where("job_id ILIKE ?", like).
				Or("worker_id ILIKE ?", like).
				Or("error ILIKE ?", like),
		)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count job executions: %w", err)
	}

	order := DefaultJobExecutionOrder
	for _, allowed := range JobExecutionOrders {
		if sortOrder == allowed {
			order = sortOrder
			break
		}
	}

	var out []models.JobExecution
	err := q.Order(order).Limit(int(limit)).Offset(int(offset)).Find(&out).Error
	if err != nil {
		return nil, 0, fmt.Errorf("find job executions: %w", err)
	}
	if out == nil {
		out = []models.JobExecution{}
	}
	return out, total, nil
}

func (r *JobExecutionRepository) DistinctQueuesAndTypes(ctx context.Context) (queues, typesCol []string, err error) {
	rawQ, err := r.distinctCol(ctx, "queue")
	if err != nil {
		return nil, nil, err
	}
	rawT, err := r.distinctCol(ctx, "type")
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(rawQ)
	sort.Strings(rawT)
	return rawQ, rawT, nil
}

func (r *JobExecutionRepository) distinctCol(ctx context.Context, col string) ([]string, error) {
	if col != "queue" && col != "type" {
		return nil, fmt.Errorf("unsupported column %q", col)
	}
	var raw []string
	if err := r.q(ctx).Model(&models.JobExecution{}).Distinct(col).Pluck(col, &raw).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, s := range raw {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out, nil
}
