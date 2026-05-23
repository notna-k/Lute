package repos

import (
	"context"
	"time"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/types"
	"gorm.io/gorm"
)

type WorkerRepository struct {
	g *gorm.DB
}

func NewWorkerRepository(db *gorm.DB) *WorkerRepository {
	return &WorkerRepository{g: db}
}

func (r *WorkerRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *WorkerRepository) Create(ctx context.Context, w *models.Worker) error {
	return mapErr(r.q(ctx).Create(w).Error)
}

func (r *WorkerRepository) GetByID(ctx context.Context, uid id.ID) (*models.Worker, error) {
	var w models.Worker
	if err := r.q(ctx).Where("id = ?", uid.Hex()).First(&w).Error; err != nil {
		return nil, mapErr(err)
	}
	return &w, nil
}

func (r *WorkerRepository) GetByUserID(ctx context.Context, userID id.ID) ([]*models.Worker, error) {
	var out []*models.Worker
	err := r.q(ctx).
		Where("user_id = ? OR user_id IS NULL", userID.Hex()).
		Order("created_at ASC").
		Find(&out).Error
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WorkerRepository) Update(ctx context.Context, uid id.ID, w *models.Worker) error {
	w.ID = uid
	return mapErr(r.q(ctx).Save(w).Error)
}

func (r *WorkerRepository) Delete(ctx context.Context, uid id.ID) error {
	return mapErr(r.q(ctx).Where("id = ?", uid.Hex()).Delete(&models.Worker{}).Error)
}

func (r *WorkerRepository) UpdateStatus(ctx context.Context, uid id.ID, status enums.WorkerStatus) error {
	nowMs := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", uid.Hex()).Updates(map[string]interface{}{
		"status":     status,
		"updated_at": nowMs,
	}).Error)
}

func (r *WorkerRepository) FindByAgentID(ctx context.Context, agentID string) (*models.Worker, error) {
	_ = agentID
	return nil, ErrNotFound
}

func (r *WorkerRepository) GetByUserIDAndIP(ctx context.Context, userID id.ID, ip string) ([]*models.Worker, error) {
	if ip == "" {
		return nil, nil
	}
	var out []*models.Worker
	err := r.q(ctx).Where("user_id = ? AND agent_ip = ?", userID.Hex(), ip).Find(&out).Error
	return out, err
}

func (r *WorkerRepository) UpdateLastSeen(ctx context.Context, workerID id.ID) error {
	nowMs := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(map[string]interface{}{
		"last_seen":  nowMs,
		"updated_at": nowMs,
	}).Error)
}

func (r *WorkerRepository) UpdateMetrics(ctx context.Context, workerID id.ID, metrics map[string]interface{}) error {
	var w models.Worker
	if err := r.q(ctx).Where("id = ?", workerID.Hex()).First(&w).Error; err != nil {
		return mapErr(err)
	}
	w.Metrics = metrics
	return mapErr(r.q(ctx).Save(&w).Error)
}

func (r *WorkerRepository) UpdateAgentInfo(ctx context.Context, workerID id.ID, ipAddress, version string) error {
	nowMs := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(map[string]interface{}{
		"agent_ip":       ipAddress,
		"agent_version":  version,
		"last_seen":      nowMs,
		"updated_at":     nowMs,
	}).Error)
}

func (r *WorkerRepository) ListByStatus(ctx context.Context, status enums.WorkerStatus) ([]*models.Worker, error) {
	var out []*models.Worker
	err := r.q(ctx).Where("status = ?", status).Find(&out).Error
	return out, err
}

func (r *WorkerRepository) UpdateStatusAndLastSeen(ctx context.Context, workerID id.ID, status enums.WorkerStatus) error {
	nowMs := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(map[string]interface{}{
		"status":     status,
		"last_seen":  nowMs,
		"updated_at": nowMs,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WorkerRepository) UpdateHeartbeat(ctx context.Context, workerID id.ID, metrics map[string]interface{}) error {
	var w models.Worker
	if err := r.q(ctx).Where("id = ?", workerID.Hex()).First(&w).Error; err != nil {
		return mapErr(err)
	}
	w.Status = enums.WorkerAlive
	w.HeartbeatRetry = 0
	ls := types.NewMilliTime(time.Now())
	w.LastSeen = &ls
	if len(metrics) > 0 {
		w.Metrics = metrics
	}
	return mapErr(r.q(ctx).Save(&w).Error)
}

func (r *WorkerRepository) IncrementHeartbeatRetry(ctx context.Context, workerID id.ID) (int, error) {
	nowMs := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(map[string]interface{}{
		"heartbeat_retry": gorm.Expr("heartbeat_retry + ?", 1),
		"updated_at":      nowMs,
	})
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, ErrNotFound
	}
	var w models.Worker
	if err := r.q(ctx).Where("id = ?", workerID.Hex()).First(&w).Error; err != nil {
		return 0, mapErr(err)
	}
	return w.HeartbeatRetry, nil
}

func (r *WorkerRepository) ListMonitored(ctx context.Context) ([]*models.Worker, error) {
	var out []*models.Worker
	err := r.q(ctx).Where("status IN ?", []enums.WorkerStatus{enums.WorkerAlive, enums.WorkerRegistered}).Find(&out).Error
	return out, err
}

// CountByUserIDAndStatusResult is one row from AggregateCountsByUserID.
type CountByUserIDAndStatusResult struct {
	UserID id.ID
	Alive  int
	Dead   int
	Total  int
}

func (r *WorkerRepository) AggregateCountsByUserID(ctx context.Context) ([]CountByUserIDAndStatusResult, error) {
	type row struct {
		UserIDStr string `gorm:"column:user_id"`
		Alive     int64  `gorm:"column:alive"`
		Dead      int64  `gorm:"column:dead"`
		Total     int64  `gorm:"column:total"`
	}
	var raw []row
	tx := r.q(ctx).Model(&models.Worker{}).
		Select(`user_id, SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS alive,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS dead,
			COUNT(*) AS total`, enums.WorkerAlive, enums.WorkerDead).
		Where("user_id IS NOT NULL AND user_id <> ''").
		Group("user_id")
	if err := tx.Scan(&raw).Error; err != nil {
		return nil, err
	}
	out := make([]CountByUserIDAndStatusResult, 0, len(raw))
	for _, rw := range raw {
		out = append(out, CountByUserIDAndStatusResult{
			UserID: id.ID(rw.UserIDStr),
			Alive:  int(rw.Alive),
			Dead:   int(rw.Dead),
			Total:  int(rw.Total),
		})
	}
	return out, nil
}
