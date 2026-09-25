package repos

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type RefreshTokenRepository struct {
	g *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{g: db}
}

func (r *RefreshTokenRepository) q(ctx context.Context) *gorm.DB {
	return r.g.WithContext(ctx)
}

func (r *RefreshTokenRepository) Create(ctx context.Context, t *models.RefreshToken) error {
	return mapErr(r.q(ctx).Create(t).Error)
}

// GetByHash includes used and revoked rows, so the caller can detect token reuse.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.q(ctx).Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

// MarkUsed claims an unused, unrevoked token; ErrNotFound means it was already used or revoked.
func (r *RefreshTokenRepository) MarkUsed(ctx context.Context, tokenID id.ID) error {
	now := time.Now().UTC().UnixMilli()
	res := r.q(ctx).Model(&models.RefreshToken{}).
		Where("id = ? AND used_at IS NULL AND revoked_at IS NULL", tokenID.Hex()).
		Updates(map[string]interface{}{"used_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID id.ID) error {
	now := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID.Hex()).
		Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error)
}
