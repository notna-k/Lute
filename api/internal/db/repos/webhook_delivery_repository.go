package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type WebhookDeliveryRepository struct {
	db *sql.DB
}

func NewWebhookDeliveryRepository(db *sql.DB) *WebhookDeliveryRepository {
	return &WebhookDeliveryRepository{db: db}
}

func (r *WebhookDeliveryRepository) Create(ctx context.Context, d *models.WebhookDelivery) error {
	d.BeforeCreate()
	_, err := r.db.ExecContext(ctx, `INSERT INTO webhook_deliveries (
		id, created_at, updated_at, run_id, job_id, user_id, event, url, payload, signature,
		signed_timestamp, status, attempts, max_attempts, next_retry_at, last_error, response_status, delivered_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		d.ID.Hex(), timeMilli(d.CreatedAt), timeMilli(d.UpdatedAt), d.RunID.Hex(), d.JobID, d.UserID.Hex(),
		d.Event, d.URL, d.Payload, d.Signature, d.SignedTimestamp, d.Status, d.Attempts, d.MaxAttempts,
		timeMilli(d.NextRetryAt), d.LastError, d.ResponseStatus, timePtrArg(d.DeliveredAt),
	)
	return err
}

func (r *WebhookDeliveryRepository) ClaimDue(ctx context.Context, limit int64) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}
	now := time.Now()
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, run_id, job_id, user_id, event, url, payload, signature,
			signed_timestamp, status, attempts, max_attempts, next_retry_at, last_error, response_status, delivered_at
		FROM webhook_deliveries
		WHERE status = 'pending' AND next_retry_at <= ?
		ORDER BY next_retry_at ASC
		LIMIT ?`, timeMilli(now), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	candidates, err := scanWebhookRows(rows)
	if err != nil {
		return nil, err
	}

	claimed := make([]models.WebhookDelivery, 0, len(candidates))
	for _, d := range candidates {
		res, err := r.db.ExecContext(ctx, `
			UPDATE webhook_deliveries SET status = 'in_flight', updated_at = ?
			WHERE id = ? AND status = 'pending'`, timeMilli(now), d.ID.Hex())
		if err != nil {
			continue
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			continue
		}
		d.Status = "in_flight"
		claimed = append(claimed, d)
	}
	return claimed, nil
}

func scanWebhookRows(rows *sql.Rows) ([]models.WebhookDelivery, error) {
	var out []models.WebhookDelivery
	for rows.Next() {
		var d models.WebhookDelivery
		var ca, ua, nra int64
		var idStr, rid, jid, uid string
		var del sql.NullInt64
		if err := rows.Scan(
			&idStr, &ca, &ua, &rid, &jid, &uid, &d.Event, &d.URL, &d.Payload, &d.Signature,
			&d.SignedTimestamp, &d.Status, &d.Attempts, &d.MaxAttempts, &nra, &d.LastError, &d.ResponseStatus, &del,
		); err != nil {
			return nil, err
		}
		d.ID = id.ID(idStr)
		d.CreatedAt = time.UnixMilli(ca).UTC()
		d.UpdatedAt = time.UnixMilli(ua).UTC()
		d.RunID = id.ID(rid)
		d.JobID = jid
		d.UserID = id.ID(uid)
		d.NextRetryAt = time.UnixMilli(nra).UTC()
		d.DeliveredAt = readTimePtr(del)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *WebhookDeliveryRepository) MarkDelivered(ctx context.Context, delID id.ID, attempts, status int) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_deliveries SET status = 'delivered', attempts = ?, response_status = ?,
			delivered_at = ?, updated_at = ? WHERE id = ?`,
		attempts, status, timeMilli(now), timeMilli(now), delID.Hex())
	return err
}

func (r *WebhookDeliveryRepository) MarkRetry(
	ctx context.Context,
	delID id.ID,
	attempts, maxAttempts int,
	lastErr string,
	responseStatus int,
) error {
	now := time.Now()
	if attempts >= maxAttempts {
		_, err := r.db.ExecContext(ctx, `
			UPDATE webhook_deliveries SET status = 'failed', attempts = ?, response_status = ?,
				last_error = ?, updated_at = ? WHERE id = ?`,
			attempts, responseStatus, lastErr, timeMilli(now), delID.Hex())
		return err
	}
	delay := time.Duration(1<<uint(attempts)) * 30 * time.Second
	if delay > 30*time.Minute {
		delay = 30 * time.Minute
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE webhook_deliveries SET status = 'pending', attempts = ?, response_status = ?,
			last_error = ?, next_retry_at = ?, updated_at = ? WHERE id = ?`,
		attempts, responseStatus, lastErr, timeMilli(now.Add(delay)), timeMilli(now), delID.Hex())
	return err
}

func (r *WebhookDeliveryRepository) ListByJobID(ctx context.Context, jobID string) ([]models.WebhookDelivery, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, run_id, job_id, user_id, event, url, payload, signature,
			signed_timestamp, status, attempts, max_attempts, next_retry_at, last_error, response_status, delivered_at
		FROM webhook_deliveries WHERE job_id = ? ORDER BY created_at DESC`, jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out, err := scanWebhookRows(rows)
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.WebhookDelivery{}
	}
	return out, nil
}
