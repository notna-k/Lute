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

// GetByHash returns the row whose token_hash matches. Includes used/revoked rows so the caller
// can detect token-reuse attacks.
func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	if err := r.q(ctx).Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

// MarkUsed sets used_at=now on the row, but only if it is still unused and not revoked.
// Returns ErrNotFound if the row was already used / revoked (so the caller can treat that as reuse).
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

// RevokeFamily revokes every (non-revoked) row in the given family. Used both on logout
// and on token-reuse detection (theft).
func (r *RefreshTokenRepository) RevokeFamily(ctx context.Context, familyID id.ID) error {
	now := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID.Hex()).
		Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error)
}

// RevokeAllForUser revokes every active family for a user (e.g. on password change).
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID id.ID) error {
	now := time.Now().UTC().UnixMilli()
	return mapErr(r.q(ctx).Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID.Hex()).
		Updates(map[string]interface{}{"revoked_at": now, "updated_at": now}).Error)
}

// DeleteExpired prunes rows whose expires_at has passed by more than the grace window.
// Safe to run periodically.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, graceBefore time.Time) error {
	cutoff := graceBefore.UTC().UnixMilli()
	return mapErr(r.q(ctx).Where("expires_at < ?", cutoff).Delete(&models.RefreshToken{}).Error)
}
