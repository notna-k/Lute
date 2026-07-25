package repos

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lute/api/internal/db/models"
)

// JobDefinitionRepository persists Git-managed job definitions.
type JobDefinitionRepository struct {
	g *gorm.DB
}

func NewJobDefinitionRepository(db *gorm.DB) *JobDefinitionRepository {
	return &JobDefinitionRepository{g: db}
}

func (r *JobDefinitionRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

// List returns all definitions, alphabetical by slug.
func (r *JobDefinitionRepository) List(ctx context.Context) ([]models.JobDefinition, error) {
	var rows []models.JobDefinition
	if err := r.q(ctx).Order("slug ASC").Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	if rows == nil {
		rows = []models.JobDefinition{}
	}
	return rows, nil
}

// GetBySlug loads one definition or returns ErrNotFound.
func (r *JobDefinitionRepository) GetBySlug(ctx context.Context, slug string) (*models.JobDefinition, error) {
	var row models.JobDefinition
	if err := r.q(ctx).Where("slug = ?", slug).First(&row).Error; err != nil {
		return nil, mapErr(err)
	}
	return &row, nil
}

// Upsert inserts or updates a definition, keyed by slug. Used by the sync loop
// so re-running against the same source is idempotent.
func (r *JobDefinitionRepository) Upsert(ctx context.Context, def *models.JobDefinition) error {
	return mapErr(r.q(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "slug"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "description", "queue", "label_selector", "runtime",
			"command", "source_repo", "source_path", "source_commit",
			"parameters", "updated_at",
		}),
	}).Create(def).Error)
}

// DeleteMissing removes definitions whose slug is not in keep. Called after a
// full sync so definitions deleted from the source disappear from the panel.
func (r *JobDefinitionRepository) DeleteMissing(ctx context.Context, keep []string) error {
	q := r.q(ctx).Session(&gorm.Session{})
	if len(keep) == 0 {
		return mapErr(q.Where("1 = 1").Delete(&models.JobDefinition{}).Error)
	}
	return mapErr(q.Where("slug NOT IN ?", keep).Delete(&models.JobDefinition{}).Error)
}
