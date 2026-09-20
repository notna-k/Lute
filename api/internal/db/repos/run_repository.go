package repos

import (
	"context"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

type RunRepository struct {
	g *gorm.DB
}

func NewRunRepository(db *gorm.DB) *RunRepository {
	return &RunRepository{g: db}
}

func (r *RunRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *RunRepository) Create(ctx context.Context, run *models.Run) error {
	return mapErr(r.q(ctx).Create(run).Error)
}

func (r *RunRepository) GetByJobID(ctx context.Context, jobID string) (*models.Run, error) {
	var row models.Run
	if err := r.q(ctx).Where("job_id = ?", jobID).First(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	return &row, nil
}

func (r *RunRepository) GetByIdempotency(ctx context.Context, userID id.ID, key string) (*models.Run, error) {
	var row models.Run
	if err := r.q(ctx).Where("user_id = ? AND idempotency_key = ?", userID.Hex(), key).First(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	return &row, nil
}

type RunListFilter struct {
	UserID  id.ID
	Queue   string
	Type    string
	JobSlug string
}

// ListByJobSlug returns the most recent runs (builds) for a job definition,
// scoped to a user, newest first.
func (r *RunRepository) ListByJobSlug(ctx context.Context, userID id.ID, slug string, limit int) ([]models.Run, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var rows []models.Run
	err := r.q(ctx).
		Where("user_id = ? AND job_slug = ?", userID.Hex(), slug).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, mapErr(err)
	}
	if rows == nil {
		rows = []models.Run{}
	}
	return rows, nil
}

// ListByJobSlugs returns recent runs across several job definitions in one
// query, grouped by slug and capped at perSlug newest-first entries each. Used
// to render the jobs list without a query per job.
func (r *RunRepository) ListByJobSlugs(ctx context.Context, userID id.ID, slugs []string, perSlug int) (map[string][]models.Run, error) {
	out := make(map[string][]models.Run, len(slugs))
	if len(slugs) == 0 {
		return out, nil
	}
	if perSlug <= 0 || perSlug > 100 {
		perSlug = 100
	}
	var rows []models.Run
	err := r.q(ctx).
		Where("user_id = ? AND job_slug IN ?", userID.Hex(), slugs).
		Order("created_at DESC").
		Limit(perSlug * len(slugs)).
		Find(&rows).Error
	if err != nil {
		return nil, mapErr(err)
	}
	for i := range rows {
		slug := rows[i].JobSlug
		if len(out[slug]) < perSlug {
			out[slug] = append(out[slug], rows[i])
		}
	}
	return out, nil
}

func (r *RunRepository) List(ctx context.Context, f RunListFilter, offset, limit int64) ([]models.Run, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	q := r.q(ctx).Model(&models.Run{}).Where("user_id = ?", f.UserID.Hex())
	if f.Queue != "" {
		q = q.Where("queue = ?", f.Queue)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}
	if f.JobSlug != "" {
		q = q.Where("job_slug = ?", f.JobSlug)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.Run
	if err := q.Order("created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []models.Run{}
	}
	return rows, total, nil
}
