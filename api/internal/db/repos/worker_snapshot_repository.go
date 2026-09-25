package repos

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
)

type WorkerSnapshotRepository struct {
	g *gorm.DB
}

func NewWorkerSnapshotRepository(db *gorm.DB) *WorkerSnapshotRepository {
	return &WorkerSnapshotRepository{g: db}
}

func (r *WorkerSnapshotRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *WorkerSnapshotRepository) Insert(ctx context.Context, workerID id.ID, at time.Time, metrics map[string]interface{}) error {
	row := &models.WorkerSnapshot{
		WorkerID: workerID,
		At:       types.NewMilliTime(at),
		Metrics:  metrics,
	}
	return r.q(ctx).Create(row).Error
}

func (r *WorkerSnapshotRepository) GetByWorkerID(ctx context.Context, workerID id.ID, since time.Time) ([]*models.WorkerSnapshot, error) {
	return r.GetByWorkerIDs(ctx, []id.ID{workerID}, since)
}

func (r *WorkerSnapshotRepository) GetByWorkerIDs(ctx context.Context, workerIDs []id.ID, since time.Time) ([]*models.WorkerSnapshot, error) {
	if len(workerIDs) == 0 {
		return nil, nil
	}
	ids := make([]string, len(workerIDs))
	for i, w := range workerIDs {
		ids[i] = w.Hex()
	}
	var out []*models.WorkerSnapshot
	err := r.q(ctx).
		Where("worker_id IN ? AND at >= ?", ids, since.UnixMilli()).
		Order("at ASC").
		Find(&out).Error
	return out, err
}

func (r *WorkerSnapshotRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	return r.q(ctx).Where("at < ?", cutoff.UnixMilli()).Delete(&models.WorkerSnapshot{}).Error
}
