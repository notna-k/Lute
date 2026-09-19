package repos

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
)

// JobDefinitionRepository persists job definitions.
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

// Update rewrites a definition's mutable columns, keyed by slug: its spec, its
// place in Git, and the Git snapshot it is compared against. Callers load the
// row, change what they own, and write the whole thing back.
func (r *JobDefinitionRepository) Update(ctx context.Context, def *models.JobDefinition) error {
	// Struct update (not a map) so the json serializers are applied; Select
	// lists the columns explicitly so clearing a field still writes.
	def.UpdatedAt = types.NewMilliTime(time.Now())
	res := r.q(ctx).Model(&models.JobDefinition{}).
		Where("slug = ?", def.Slug).
		Select("name", "description", "queue", "label_selector", "runtime",
			"command", "source_repo", "parameters", "source_path",
			"source_commit", "git_spec", "updated_at").
		Updates(def)
	if res.Error != nil {
		return mapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSlugs removes the given definitions. Build history is kept: runs
// reference a slug, not a row.
func (r *JobDefinitionRepository) DeleteSlugs(ctx context.Context, slugs []string) error {
	if len(slugs) == 0 {
		return nil
	}
	return mapErr(r.q(ctx).Where("slug IN ?", slugs).Delete(&models.JobDefinition{}).Error)
}

// Create inserts a definition, failing if the slug is taken.
func (r *JobDefinitionRepository) Create(ctx context.Context, def *models.JobDefinition) error {
	return mapErr(r.q(ctx).Create(def).Error)
}
