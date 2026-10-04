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

// GetByPrefix returns the usable key with this prefix: not revoked, and for an account
// key, still owned by an existing user.
func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	var k models.APIKey
	err := r.q(ctx).
		Where("prefix = ? AND revoked_at IS NULL", prefix).
		Where("(scope = ? OR EXISTS (SELECT 1 FROM users WHERE users.id = api_keys.user_id))", models.KeyScopeService).
		First(&k).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &k, nil
}

func (r *APIKeyRepository) GetByID(ctx context.Context, keyID id.ID) (*models.APIKey, error) {
	var k models.APIKey
	if err := r.q(ctx).Where("id = ?", keyID.Hex()).First(&k).Error; err != nil {
		return nil, mapErr(err)
	}
	return &k, nil
}

// ListByUser returns userID's account keys, newest first.
func (r *APIKeyRepository) ListByUser(ctx context.Context, userID id.ID) ([]*models.APIKey, error) {
	var out []*models.APIKey
	err := r.q(ctx).Where("scope = ? AND user_id = ?", models.KeyScopeAccount, userID.Hex()).
		Order("created_at DESC").Find(&out).Error
	return out, err
}

// ListService returns every service key, newest first.
func (r *APIKeyRepository) ListService(ctx context.Context) ([]*models.APIKey, error) {
	var out []*models.APIKey
	err := r.q(ctx).Where("scope = ?", models.KeyScopeService).Order("created_at DESC").Find(&out).Error
	return out, err
}

// Names maps each key id to its name; unknown ids are left out.
func (r *APIKeyRepository) Names(ctx context.Context, ids []id.ID) (map[id.ID]string, error) {
	out := make(map[id.ID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []models.APIKey
	if err := r.q(ctx).Select("id", "name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, k := range rows {
		out[k.ID] = k.Name
	}
	return out, nil
}

// Revoke revokes keyID for userID: their own account key, or any service key.
func (r *APIKeyRepository) Revoke(ctx context.Context, keyID, userID id.ID) error {
	nowMs := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.APIKey{}).
		Where("id = ? AND revoked_at IS NULL", keyID.Hex()).
		Where("(scope = ? OR user_id = ?)", models.KeyScopeService, userID.Hex()).
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
