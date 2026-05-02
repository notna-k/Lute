package repos

import (
	"context"
	"fmt"
	"sort"
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

type JobExecutionListFilter struct {
	Queue  string
	Type   string
	Status string // "", "success", or "failed"
}

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

	q := r.q(ctx).Model(&models.JobExecution{})
	if filter.Queue != "" {
		q = q.Where("queue = ?", filter.Queue)
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	switch filter.Status {
	case "success":
		q = q.Where("success = ?", true)
	case "failed":
		q = q.Where("success = ?", false)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count job executions: %w", err)
	}

	order := "finished_at ASC"
	if sortDesc {
		order = "finished_at DESC"
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
