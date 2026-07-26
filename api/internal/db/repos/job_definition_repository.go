package repos

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
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
			"parameters", "origin", "updated_at",
		}),
	}).Create(def).Error)
}

// DeleteMissing removes Git-sourced definitions whose slug is not in keep.
// Called after a full sync so definitions deleted from the source disappear
// from the panel.
//
// Panel-created definitions are never pruned here: no file backs them, so a
// slug-based sweep would delete every one on the next sync.
func (r *JobDefinitionRepository) DeleteMissing(ctx context.Context, keep []string) error {
	q := r.q(ctx).Session(&gorm.Session{}).Where("origin = ?", models.OriginGit)
	if len(keep) == 0 {
		return mapErr(q.Delete(&models.JobDefinition{}).Error)
	}
	return mapErr(q.Where("slug NOT IN ?", keep).Delete(&models.JobDefinition{}).Error)
}

// Update rewrites a definition's editable config, keyed by slug. Origin, slug
// and the source ref are not touched — those describe where it came from, not
// what it does.
func (r *JobDefinitionRepository) Update(ctx context.Context, def *models.JobDefinition) error {
	// Struct update (not a map) so the json serializer on label_selector and
	// parameters is applied; Select lists the columns explicitly so clearing a
	// field to its zero value still writes.
	res := r.q(ctx).Model(&models.JobDefinition{}).
		Where("slug = ?", def.Slug).
		Select("name", "description", "queue", "label_selector", "runtime",
			"command", "source_repo", "parameters", "updated_at").
		Updates(&models.JobDefinition{
			Name:          def.Name,
			Description:   def.Description,
			Queue:         def.Queue,
			LabelSelector: def.LabelSelector,
			Runtime:       def.Runtime,
			Command:       def.Command,
			SourceRepo:    def.SourceRepo,
			Parameters:    def.Parameters,
			BaseModel:     models.BaseModel{UpdatedAt: types.NewMilliTime(time.Now())},
		})
	if res.Error != nil {
		return mapErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Create inserts a panel-authored definition, failing if the slug is taken.
func (r *JobDefinitionRepository) Create(ctx context.Context, def *models.JobDefinition) error {
	return mapErr(r.q(ctx).Create(def).Error)
}
