package repos

import (
	"context"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

type CommandRepository struct {
	g *gorm.DB
}

func NewCommandRepository(db *gorm.DB) *CommandRepository {
	return &CommandRepository{g: db}
}

func (r *CommandRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *CommandRepository) Create(ctx context.Context, cmd *models.Command) error {
	if cmd.Status == "" {
		cmd.Status = enums.CommandPending
	}
	return mapErr(r.q(ctx).Create(cmd).Error)
}

func (r *CommandRepository) GetByID(ctx context.Context, cid id.ID) (*models.Command, error) {
	var c models.Command
	if err := r.q(ctx).Where("id = ?", cid.Hex()).First(&c).Error; err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (r *CommandRepository) GetPendingByWorkerID(ctx context.Context, workerID id.ID) ([]*models.Command, error) {
	var out []*models.Command
	err := r.q(ctx).
		Where("worker_id = ? AND status = ?", workerID.Hex(), enums.CommandPending).
		Order("created_at ASC").
		Find(&out).Error
	return out, err
}

func (r *CommandRepository) GetByWorkerID(ctx context.Context, workerID id.ID, limit int64) ([]*models.Command, error) {
	q := r.q(ctx).Where("worker_id = ?", workerID.Hex()).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(int(limit))
	}
	var out []*models.Command
	err := q.Find(&out).Error
	return out, err
}

func (r *CommandRepository) UpdateResult(ctx context.Context, cid id.ID, status, output string, exitCode int, errMsg string) error {
	var c models.Command
	if err := r.q(ctx).Where("id = ?", cid.Hex()).First(&c).Error; err != nil {
		return mapErr(err)
	}
	c.Status = enums.CommandStatus(status)
	c.Output = output
	c.ExitCode = exitCode
	c.Error = errMsg
	return mapErr(r.q(ctx).Save(&c).Error)
}

func (r *CommandRepository) MarkRunning(ctx context.Context, cid id.ID) error {
	var c models.Command
	if err := r.q(ctx).Where("id = ?", cid.Hex()).First(&c).Error; err != nil {
		return mapErr(err)
	}
	c.Status = enums.CommandRunning
	return mapErr(r.q(ctx).Save(&c).Error)
}
