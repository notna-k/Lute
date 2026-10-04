package repos

import (
	"context"
	"errors"
	"strings"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

// ErrAmbiguous means a run id prefix matches more than one run.
var ErrAmbiguous = errors.New("run id prefix matches more than one run")

type RunRepository struct {
	g *gorm.DB
}

func NewRunRepository(db *gorm.DB) *RunRepository {
	return &RunRepository{g: db}
}

func (r *RunRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

// RunScope is whose runs a query covers. A user sees their own runs and those service
// keys started; All covers every run, which a service key gets until roles exist.
type RunScope struct {
	UserID id.ID
	All    bool
}

func (s RunScope) apply(q *gorm.DB) *gorm.DB {
	if s.All {
		return q
	}
	return q.Where("(runs.user_id = ? OR runs.user_id IS NULL)", s.UserID.Hex())
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

// GetByRef finds a run in scope by its full id or a prefix of it. A prefix that matches
// several runs returns ErrAmbiguous.
func (r *RunRepository) GetByRef(ctx context.Context, scope RunScope, ref string) (*models.Run, error) {
	var rows []models.Run
	q := scope.apply(r.q(ctx).Model(&models.Run{}))
	if len(ref) == 24 {
		q = q.Where("runs.id = ?", ref)
	} else {
		q = q.Where("runs.id LIKE ?", escapeLike(ref)+"%")
	}
	if err := q.Limit(2).Find(&rows).Error; err != nil {
		return nil, err
	}
	switch len(rows) {
	case 0:
		return nil, ErrNotFound
	case 1:
		return &rows[0], nil
	default:
		return nil, ErrAmbiguous
	}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// GetByIdempotency finds the run an owner already started with key: a user's, or a
// service key's when userID is empty.
func (r *RunRepository) GetByIdempotency(ctx context.Context, userID, apiKeyID id.ID, key string) (*models.Run, error) {
	q := r.q(ctx).Where("idempotency_key = ?", key)
	if userID.IsZero() {
		q = q.Where("user_id IS NULL AND api_key_id = ?", apiKeyID.Hex())
	} else {
		q = q.Where("user_id = ?", userID.Hex())
	}
	var row models.Run
	if err := q.First(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	return &row, nil
}

type RunListFilter struct {
	Scope   RunScope
	Queue   string
	Type    string
	JobSlug string
	// Status is a public run status, derived as RunStatusSQL does.
	Status string
}

// RunStatusSQL derives a run's public status from its queue slot and execution record:
// a successful execution wins, then the slot's own status, then a failed execution.
const RunStatusSQL = `CASE
	WHEN je.success THEN 'done'
	WHEN qs.job_id IS NOT NULL THEN qs.payload::jsonb->>'status'
	WHEN je.job_id IS NOT NULL THEN 'failed'
	ELSE 'unknown' END`

// ListByJobSlugs returns up to perSlug newest runs in scope per slug in one query.
func (r *RunRepository) ListByJobSlugs(ctx context.Context, scope RunScope, slugs []string, perSlug int) (map[string][]models.Run, error) {
	out := make(map[string][]models.Run, len(slugs))
	if len(slugs) == 0 {
		return out, nil
	}
	if perSlug <= 0 || perSlug > 100 {
		perSlug = 100
	}
	// Ranked per slug, so a busy job cannot crowd the others out of the result.
	ranked := scope.apply(r.q(ctx).Model(&models.Run{})).
		Select("runs.*, ROW_NUMBER() OVER (PARTITION BY job_slug ORDER BY created_at DESC) AS rn").
		Where("job_slug IN ?", slugs)
	var rows []models.Run
	err := r.q(ctx).Table("(?) AS ranked", ranked).
		Where("rn <= ?", perSlug).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, mapErr(err)
	}
	for i := range rows {
		slug := rows[i].JobSlug
		out[slug] = append(out[slug], rows[i])
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

	q := f.Scope.apply(r.q(ctx).Model(&models.Run{}))
	if f.Queue != "" {
		q = q.Where("runs.queue = ?", f.Queue)
	}
	if f.Type != "" {
		q = q.Where("runs.type = ?", f.Type)
	}
	if f.JobSlug != "" {
		q = q.Where("runs.job_slug = ?", f.JobSlug)
	}
	if f.Status != "" {
		q = q.Joins("LEFT JOIN job_executions je ON je.job_id = runs.job_id").
			Joins("LEFT JOIN queue_slots qs ON qs.job_id = runs.job_id").
			Where(RunStatusSQL+" = ?", f.Status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []models.Run
	if err := q.Select("runs.*").Order("runs.created_at DESC").Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []models.Run{}
	}
	return rows, total, nil
}
