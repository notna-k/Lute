package repos

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
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

// GetByUserIDAndLabels returns userID's workers whose labels contain every filter pair.
func (r *WorkerRepository) GetByUserIDAndLabels(ctx context.Context, userID id.ID, filter map[string]string) ([]*models.Worker, error) {
	workers, err := r.GetByUserID(ctx, userID)
	if err != nil || len(filter) == 0 {
		return workers, err
	}
	out := workers[:0]
	for _, w := range workers {
		if workerLabelsMatch(w.Labels, filter) {
			out = append(out, w)
		}
	}
	return out, nil
}

func workerLabelsMatch(labels, filter map[string]string) bool {
	for k, v := range filter {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func (r *WorkerRepository) UpdateFields(ctx context.Context, uid id.ID, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", uid.Hex()).Updates(fields).Error)
}

func (r *WorkerRepository) UpdateLabels(ctx context.Context, uid id.ID, labels map[string]string) error {
	return r.UpdateFields(ctx, uid, map[string]any{"labels": jsonText(labels)})
}

// MarkDead fails a worker that stopped answering pings, unless it has moved on meanwhile
// (a worker being deleted stays "deleting").
func (r *WorkerRepository) MarkDead(ctx context.Context, uid id.ID) error {
	return mapErr(r.q(ctx).Model(&models.Worker{}).
		Where("id = ? AND status IN ?", uid.Hex(), []enums.WorkerStatus{enums.WorkerRegistered, enums.WorkerAlive}).
		Updates(map[string]any{"status": enums.WorkerDead, "updated_at": time.Now().UTC().UnixMilli()}).Error)
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

func (r *WorkerRepository) UpdateLastSeen(ctx context.Context, workerID id.ID) error {
	nowMs := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(map[string]interface{}{
		"last_seen":  nowMs,
		"updated_at": nowMs,
	}).Error)
}

func (r *WorkerRepository) UpdateAgentInfo(ctx context.Context, workerID id.ID, version string, protocol int32, engine *models.Engine, peerIP string) error {
	var w models.Worker
	if err := r.q(ctx).Select("metadata").Where("id = ?", workerID.Hex()).First(&w).Error; err != nil {
		return mapErr(err)
	}
	if w.Metadata == nil {
		w.Metadata = map[string]any{}
	}
	if peerIP != "" {
		w.Metadata["ip"] = peerIP
	}
	nowMs := time.Now().UTC().UnixMilli()
	updates := map[string]any{
		"agent_version": version,
		"protocol":      protocol,
		"metadata":      jsonText(w.Metadata),
		"last_seen":     nowMs,
		"updated_at":    nowMs,
	}
	if engine != nil {
		updates["engine"] = jsonText(engine)
	}
	return mapErr(r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(updates).Error)
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

// UpdateHeartbeat marks a worker alive after a pong. Only a registered or alive worker
// becomes alive: a heartbeat racing a delete must not undo "deleting".
func (r *WorkerRepository) UpdateHeartbeat(ctx context.Context, workerID id.ID, metrics map[string]interface{}) error {
	nowMs := time.Now().UTC().UnixMilli()
	updates := map[string]any{
		"status": gorm.Expr("CASE WHEN status IN (?, ?) THEN ? ELSE status END",
			enums.WorkerRegistered, enums.WorkerAlive, enums.WorkerAlive),
		"heartbeat_retry": 0,
		"last_seen":       nowMs,
		"updated_at":      nowMs,
	}
	if len(metrics) > 0 {
		updates["metrics"] = jsonText(metrics)
	}
	res := r.q(ctx).Model(&models.Worker{}).Where("id = ?", workerID.Hex()).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
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

// jsonText encodes a value for a serializer:json column written through a map update,
// which bypasses the model's serializer.
func jsonText(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}
