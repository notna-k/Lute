package repos

import (
	"context"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"gorm.io/gorm"
)

type APIKeyRepository struct {
	g *gorm.DB
}

func NewAPIKeyRepository(db *gorm.DB) *APIKeyRepository {
	return &APIKeyRepository{g: db}
}

func (r *APIKeyRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *APIKeyRepository) Create(ctx context.Context, k *models.APIKey) error {
	return mapErr(r.q(ctx).Create(k).Error)
}

func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	var k models.APIKey
	if err := r.q(ctx).Where("prefix = ?", prefix).Where("revoked_at IS NULL").First(&k).Error; err != nil {
		return nil, mapErr(err)
	}
	return &k, nil
}

func (r *APIKeyRepository) ListByUser(ctx context.Context, userID id.ID) ([]*models.APIKey, error) {
	var out []*models.APIKey
	err := r.q(ctx).Where("user_id = ?", userID.Hex()).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *APIKeyRepository) Revoke(ctx context.Context, keyID, userID id.ID) error {
	nowMs := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.APIKey{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", keyID.Hex(), userID.Hex()).
		Updates(map[string]interface{}{
			"revoked_at": nowMs,
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

func (r *APIKeyRepository) TouchUsed(ctx context.Context, keyID id.ID) error {
	return mapErr(r.q(ctx).Model(&models.APIKey{}).Where("id = ?", keyID.Hex()).
		Update("last_used_at", time.Now().UTC().UnixMilli()).Error)
}
