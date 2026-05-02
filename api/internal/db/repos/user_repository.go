package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.BeforeCreate()
	_, err := r.db.ExecContext(ctx, `INSERT INTO users (id, created_at, updated_at, email, display_name, firebase_uid) VALUES (?,?,?,?,?,?)`,
		user.ID.Hex(), timeMilli(user.CreatedAt), timeMilli(user.UpdatedAt),
		user.Email, user.DisplayName, user.FirebaseUID)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, uid id.ID) (*models.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, created_at, updated_at, email, display_name, firebase_uid FROM users WHERE id = ?`, uid.Hex())
	return scanUser(row)
}

func (r *UserRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, created_at, updated_at, email, display_name, firebase_uid FROM users WHERE firebase_uid = ?`, firebaseUID)
	return scanUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, created_at, updated_at, email, display_name, firebase_uid FROM users WHERE email = ?`, email)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var ca, ua int64
	var idStr string
	if err := row.Scan(&idStr, &ca, &ua, &u.Email, &u.DisplayName, &u.FirebaseUID); err != nil {
		return nil, mapErr(err)
	}
	u.ID = id.ID(idStr)
	u.CreatedAt = time.UnixMilli(ca).UTC()
	u.UpdatedAt = time.UnixMilli(ua).UTC()
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, uid id.ID, user *models.User) error {
	user.BeforeUpdate()
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET email = ?, display_name = ?, firebase_uid = ?, updated_at = ? WHERE id = ?`,
		user.Email, user.DisplayName, user.FirebaseUID, timeMilli(user.UpdatedAt), uid.Hex())
	return err
}

func (r *UserRepository) Delete(ctx context.Context, uid id.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, uid.Hex())
	return err
}
