package repos

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type RegistrationTokenRepository struct {
	g *gorm.DB
}

func NewRegistrationTokenRepository(db *gorm.DB) *RegistrationTokenRepository {
	return &RegistrationTokenRepository{g: db}
}

func (r *RegistrationTokenRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *RegistrationTokenRepository) Create(ctx context.Context, t *models.RegistrationToken) error {
	return mapErr(r.q(ctx).Create(t).Error)
}

// GetActiveByHash finds an unrevoked token.
func (r *RegistrationTokenRepository) GetActiveByHash(ctx context.Context, hash string) (*models.RegistrationToken, error) {
	var t models.RegistrationToken
	if err := r.q(ctx).Where("token_hash = ? AND revoked_at IS NULL", hash).First(&t).Error; err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

func (r *RegistrationTokenRepository) ExistsByHash(ctx context.Context, hash string) (bool, error) {
	var n int64
	err := r.q(ctx).Model(&models.RegistrationToken{}).Where("token_hash = ?", hash).Count(&n).Error
	return n > 0, err
}

// List returns every token, newest first. Tokens are shared: any operator may enrol workers.
func (r *RegistrationTokenRepository) List(ctx context.Context) ([]*models.RegistrationToken, error) {
	var out []*models.RegistrationToken
	err := r.q(ctx).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (r *RegistrationTokenRepository) Revoke(ctx context.Context, tokenID id.ID) error {
	nowMs := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.RegistrationToken{}).
		Where("id = ? AND revoked_at IS NULL", tokenID.Hex()).
		Updates(map[string]any{"revoked_at": nowMs, "updated_at": nowMs})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RegistrationTokenRepository) TouchUsed(ctx context.Context, tokenID id.ID) error {
	return mapErr(r.q(ctx).Model(&models.RegistrationToken{}).Where("id = ?", tokenID.Hex()).
		Update("last_used_at", time.Now().UTC().UnixMilli()).Error)
}
