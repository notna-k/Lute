package repos

import (
	"context"
	"time"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

type WebhookDeliveryRepository struct {
	g *gorm.DB
}

func NewWebhookDeliveryRepository(db *gorm.DB) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{g: db}
}

func (r *WebhookDeliveryRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *WebhookDeliveryRepository) Create(ctx context.Context, d *models.WebhookDelivery) error {
	return mapErr(r.q(ctx).Create(d).Error)
}

func (r *WebhookDeliveryRepository) ClaimDue(ctx context.Context, limit int64) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}
	nowMs := time.Now().UTC().UnixMilli()
	var candidates []models.WebhookDelivery
	if err := r.q(ctx).
		Where("status = ? AND next_retry_at <= ?", enums.WebhookDeliveryPending, nowMs).
		Order("next_retry_at ASC").
		Limit(int(limit)).
		Find(&candidates).Error; err != nil {
		return nil, err
	}

	claimed := make([]models.WebhookDelivery, 0, len(candidates))
	for _, d := range candidates {
		res := r.q(ctx).Model(&models.WebhookDelivery{}).
			Where("id = ? AND status = ?", d.ID.Hex(), enums.WebhookDeliveryPending).
			Updates(map[string]interface{}{
				"status":     enums.WebhookDeliveryInFlight,
				"updated_at": nowMs,
			})
		if res.Error != nil {
			continue
		}
		if res.RowsAffected == 0 {
			continue
		}
		d.Status = enums.WebhookDeliveryInFlight
		claimed = append(claimed, d)
	}
	return claimed, nil
}

func (r *WebhookDeliveryRepository) MarkDelivered(ctx context.Context, delID id.ID, attempts, status int) error {
	nowMs := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.WebhookDelivery{}).Where("id = ?", delID.Hex()).
		Updates(map[string]interface{}{
			"status":          enums.WebhookDeliveryDelivered,
			"attempts":        attempts,
			"response_status": status,
			"delivered_at":    nowMs,
			"updated_at":      nowMs,
		}).Error)
}

func (r *WebhookDeliveryRepository) MarkRetry(
	ctx context.Context,
	delID id.ID,
	attempts, maxAttempts int,
	lastErr string,
	responseStatus int,
) error {
	now := time.Now().UTC()
	nowMs := now.UnixMilli()
	if attempts >= maxAttempts {
		return mapErr(r.q(ctx).Model(&models.WebhookDelivery{}).Where("id = ?", delID.Hex()).
			Updates(map[string]interface{}{
				"status":          enums.WebhookDeliveryFailed,
				"attempts":        attempts,
				"response_status": responseStatus,
				"last_error":      lastErr,
				"updated_at":      nowMs,
			}).Error)
	}
	delay := time.Duration(1<<uint(attempts)) * 30 * time.Second
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	nextMs := now.Add(delay).UnixMilli()
	return mapErr(r.q(ctx).Model(&models.WebhookDelivery{}).Where("id = ?", delID.Hex()).
		Updates(map[string]interface{}{
			"status":          enums.WebhookDeliveryPending,
			"attempts":        attempts,
			"response_status": responseStatus,
			"last_error":      lastErr,
			"next_retry_at":   nextMs,
			"updated_at":      nowMs,
		}).Error)
}

func (r *WebhookDeliveryRepository) ListByJobID(ctx context.Context, jobID string) ([]models.WebhookDelivery, error) {
	var out []models.WebhookDelivery
	err := r.q(ctx).Where("job_id = ?", jobID).Order("created_at DESC").Find(&out).Error
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.WebhookDelivery{}
	}
	return out, nil
}
