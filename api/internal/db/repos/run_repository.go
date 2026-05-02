package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type RunRepository struct {
	db *sql.DB
}

func NewRunRepository(db *sql.DB) *RunRepository {
	return &RunRepository{db: db}
}

func (r *RunRepository) Create(ctx context.Context, run *models.Run) error {
	run.BeforeCreate()
	evJ, err := marshalStringSlice(run.WebhookEvents)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO runs (
		id, created_at, updated_at, job_id, user_id, api_key_id, queue, type,
		idempotency_key, webhook_url, webhook_secret, webhook_events
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		run.ID.Hex(), timeMilli(run.CreatedAt), timeMilli(run.UpdatedAt), run.JobID, run.UserID.Hex(),
		idArg(run.APIKeyID), run.Queue, run.Type, nullIfEmpty(run.IdempotencyKey), run.WebhookURL, run.WebhookSecret, evJ,
	)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *RunRepository) GetByJobID(ctx context.Context, jobID string) (*models.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, job_id, user_id, api_key_id, queue, type,
			idempotency_key, webhook_url, webhook_secret, webhook_events
		FROM runs WHERE job_id = ?`, jobID)
	return scanRun(row)
}

func scanRun(row *sql.Row) (*models.Run, error) {
	var run models.Run
	var ca, ua int64
	var idStr, userIDStr string
	var akid sql.NullString
	var idem sql.NullString
	var evJ sql.NullString
	if err := row.Scan(
		&idStr, &ca, &ua, &run.JobID, &userIDStr, &akid, &run.Queue, &run.Type,
		&idem, &run.WebhookURL, &run.WebhookSecret, &evJ,
	); err != nil {
		return nil, mapErr(err)
	}
	run.ID = id.ID(idStr)
	run.CreatedAt = time.UnixMilli(ca).UTC()
	run.UpdatedAt = time.UnixMilli(ua).UTC()
	run.UserID = id.ID(userIDStr)
	run.APIKeyID = scanID(akid)
	if idem.Valid {
		run.IdempotencyKey = idem.String
	}
	var err error
	run.WebhookEvents, err = unmarshalStringSlice(evJ)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *RunRepository) GetByIdempotency(ctx context.Context, userID id.ID, key string) (*models.Run, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, job_id, user_id, api_key_id, queue, type,
			idempotency_key, webhook_url, webhook_secret, webhook_events
		FROM runs WHERE user_id = ? AND idempotency_key = ?`, userID.Hex(), key)
	return scanRun(row)
}

type RunListFilter struct {
	UserID id.ID
	Queue  string
	Type   string
}

func (r *RunRepository) List(ctx context.Context, f RunListFilter, offset, limit int64) ([]models.Run, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	args := []any{f.UserID.Hex()}
	where := ` WHERE user_id = ?`
	if f.Queue != "" {
		where += ` AND queue = ?`
		args = append(args, f.Queue)
	}
	if f.Type != "" {
		where += ` AND type = ?`
		args = append(args, f.Type)
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `
		SELECT id, created_at, updated_at, job_id, user_id, api_key_id, queue, type,
			idempotency_key, webhook_url, webhook_secret, webhook_events
		FROM runs` + where + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var runs []models.Run
	for rows.Next() {
		var run models.Run
		var ca, ua int64
		var idStr, userIDStr string
		var akid sql.NullString
		var idem sql.NullString
		var evJ sql.NullString
		if err := rows.Scan(
			&idStr, &ca, &ua, &run.JobID, &userIDStr, &akid, &run.Queue, &run.Type,
			&idem, &run.WebhookURL, &run.WebhookSecret, &evJ,
		); err != nil {
			return nil, 0, err
		}
		run.ID = id.ID(idStr)
		run.CreatedAt = time.UnixMilli(ca).UTC()
		run.UpdatedAt = time.UnixMilli(ua).UTC()
		run.UserID = id.ID(userIDStr)
		run.APIKeyID = scanID(akid)
		if idem.Valid {
			run.IdempotencyKey = idem.String
		}
		run.WebhookEvents, err = unmarshalStringSlice(evJ)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	if runs == nil {
		runs = []models.Run{}
	}
	return runs, total, rows.Err()
}
