package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type APIKeyRepository struct {
	db *sql.DB
}

func NewAPIKeyRepository(db *sql.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) Create(ctx context.Context, k *models.APIKey) error {
	k.BeforeCreate()
	_, err := r.db.ExecContext(ctx, `INSERT INTO api_keys (
		id, created_at, updated_at, user_id, name, prefix, hash, last_used_at, revoked_at
	) VALUES (?,?,?,?,?,?,?,?,?)`,
		k.ID.Hex(), timeMilli(k.CreatedAt), timeMilli(k.UpdatedAt), k.UserID.Hex(),
		k.Name, k.Prefix, k.Hash, timePtrArg(k.LastUsedAt), timePtrArg(k.RevokedAt),
	)
	return err
}

func (r *APIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, user_id, name, prefix, hash, last_used_at, revoked_at
		FROM api_keys WHERE prefix = ? AND revoked_at IS NULL`, prefix)
	return scanAPIKey(row)
}

func scanAPIKey(row *sql.Row) (*models.APIKey, error) {
	var k models.APIKey
	var ca, ua int64
	var idStr, uid string
	var lastU, rev sql.NullInt64
	if err := row.Scan(&idStr, &ca, &ua, &uid, &k.Name, &k.Prefix, &k.Hash, &lastU, &rev); err != nil {
		return nil, mapErr(err)
	}
	k.ID = id.ID(idStr)
	k.CreatedAt = time.UnixMilli(ca).UTC()
	k.UpdatedAt = time.UnixMilli(ua).UTC()
	k.UserID = id.ID(uid)
	k.LastUsedAt = readTimePtr(lastU)
	k.RevokedAt = readTimePtr(rev)
	return &k, nil
}

func (r *APIKeyRepository) ListByUser(ctx context.Context, userID id.ID) ([]*models.APIKey, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, user_id, name, prefix, hash, last_used_at, revoked_at
		FROM api_keys WHERE user_id = ? ORDER BY created_at DESC`, userID.Hex())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*models.APIKey
	for rows.Next() {
		var k models.APIKey
		var ca, ua int64
		var idStr, uid string
		var lastU, rev sql.NullInt64
		if err := rows.Scan(&idStr, &ca, &ua, &uid, &k.Name, &k.Prefix, &k.Hash, &lastU, &rev); err != nil {
			return nil, err
		}
		k.ID = id.ID(idStr)
		k.CreatedAt = time.UnixMilli(ca).UTC()
		k.UpdatedAt = time.UnixMilli(ua).UTC()
		k.UserID = id.ID(uid)
		k.LastUsedAt = readTimePtr(lastU)
		k.RevokedAt = readTimePtr(rev)
		out = append(out, &k)
	}
	return out, rows.Err()
}

func (r *APIKeyRepository) Revoke(ctx context.Context, keyID, userID id.ID) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked_at = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		timeMilli(now), timeMilli(now), keyID.Hex(), userID.Hex())
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *APIKeyRepository) TouchUsed(ctx context.Context, keyID id.ID) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE api_keys SET last_used_at = ? WHERE id = ?`, timeMilli(now), keyID.Hex())
	return err
}
