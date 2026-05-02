package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type WorkerRepository struct {
	db *sql.DB
}

func NewWorkerRepository(db *sql.DB) *WorkerRepository {
	return &WorkerRepository{db: db}
}

func (r *WorkerRepository) Create(ctx context.Context, w *models.Worker) error {
	w.BeforeCreate()
	meta, err := marshalIfaceMap(w.Metadata)
	if err != nil {
		return err
	}
	metrics, err := marshalIfaceMap(w.Metrics)
	if err != nil {
		return err
	}
	ls := sql.NullInt64{}
	if !w.LastSeen.IsZero() {
		ls = sql.NullInt64{Int64: timeMilli(w.LastSeen), Valid: true}
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO workers (
		id, created_at, updated_at, user_id, name, description, status, metadata, agent_ip, agent_version, last_seen, metrics, heartbeat_retry
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.ID.Hex(), timeMilli(w.CreatedAt), timeMilli(w.UpdatedAt), idArg(w.UserID),
		w.Name, w.Description, w.Status, meta, w.AgentIP, w.AgentVersion, ls, metrics, w.HeartbeatRetry,
	)
	return err
}

func (r *WorkerRepository) GetByID(ctx context.Context, uid id.ID) (*models.Worker, error) {
	row := r.db.QueryRowContext(ctx, workerSelectBase+` WHERE id = ?`, uid.Hex())
	return scanWorker(row)
}

const workerSelectBase = `SELECT id, created_at, updated_at, user_id, name, description, status, metadata, agent_ip, agent_version, last_seen, metrics, heartbeat_retry FROM workers`

func scanWorker(row *sql.Row) (*models.Worker, error) {
	var w models.Worker
	var ca, ua int64
	var idStr string
	var userID sql.NullString
	var meta, metrics sql.NullString
	var ls sql.NullInt64
	if err := row.Scan(
		&idStr, &ca, &ua, &userID, &w.Name, &w.Description, &w.Status,
		&meta, &w.AgentIP, &w.AgentVersion, &ls, &metrics, &w.HeartbeatRetry,
	); err != nil {
		return nil, mapErr(err)
	}
	w.ID = id.ID(idStr)
	w.CreatedAt = time.UnixMilli(ca).UTC()
	w.UpdatedAt = time.UnixMilli(ua).UTC()
	w.UserID = scanID(userID)
	var err error
	w.Metadata, err = unmarshalIfaceMap(meta)
	if err != nil {
		return nil, err
	}
	w.Metrics, err = unmarshalIfaceMap(metrics)
	if err != nil {
		return nil, err
	}
	w.LastSeen = readTime(ls)
	return &w, nil
}

func (r *WorkerRepository) GetByUserID(ctx context.Context, userID id.ID) ([]*models.Worker, error) {
	rows, err := r.db.QueryContext(ctx, workerSelectBase+` WHERE user_id = ? OR user_id IS NULL ORDER BY created_at ASC`, userID.Hex())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanWorkerRows(rows)
}

func scanWorkerRows(rows *sql.Rows) ([]*models.Worker, error) {
	var out []*models.Worker
	for rows.Next() {
		var w models.Worker
		var ca, ua int64
		var idStr string
		var userID sql.NullString
		var meta, metrics sql.NullString
		var ls sql.NullInt64
		if err := rows.Scan(
			&idStr, &ca, &ua, &userID, &w.Name, &w.Description, &w.Status,
			&meta, &w.AgentIP, &w.AgentVersion, &ls, &metrics, &w.HeartbeatRetry,
		); err != nil {
			return nil, err
		}
		w.ID = id.ID(idStr)
		w.CreatedAt = time.UnixMilli(ca).UTC()
		w.UpdatedAt = time.UnixMilli(ua).UTC()
		w.UserID = scanID(userID)
		var err error
		w.Metadata, err = unmarshalIfaceMap(meta)
		if err != nil {
			return nil, err
		}
		w.Metrics, err = unmarshalIfaceMap(metrics)
		if err != nil {
			return nil, err
		}
		w.LastSeen = readTime(ls)
		out = append(out, &w)
	}
	return out, rows.Err()
}

func (r *WorkerRepository) Update(ctx context.Context, uid id.ID, w *models.Worker) error {
	w.BeforeUpdate()
	meta, err := marshalIfaceMap(w.Metadata)
	if err != nil {
		return err
	}
	metrics, err := marshalIfaceMap(w.Metrics)
	if err != nil {
		return err
	}
	ls := sql.NullInt64{}
	if !w.LastSeen.IsZero() {
		ls = sql.NullInt64{Int64: timeMilli(w.LastSeen), Valid: true}
	}
	_, err = r.db.ExecContext(ctx, `UPDATE workers SET
		created_at = ?, updated_at = ?, user_id = ?, name = ?, description = ?, status = ?,
		metadata = ?, agent_ip = ?, agent_version = ?, last_seen = ?, metrics = ?, heartbeat_retry = ?
		WHERE id = ?`,
		timeMilli(w.CreatedAt), timeMilli(w.UpdatedAt), idArg(w.UserID),
		w.Name, w.Description, w.Status, meta, w.AgentIP, w.AgentVersion, ls, metrics, w.HeartbeatRetry, uid.Hex(),
	)
	return err
}

func (r *WorkerRepository) Delete(ctx context.Context, uid id.ID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM workers WHERE id = ?`, uid.Hex())
	return err
}

func (r *WorkerRepository) UpdateStatus(ctx context.Context, uid id.ID, status string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET status = ?, updated_at = ? WHERE id = ?`, status, timeMilli(now), uid.Hex())
	return err
}

func (r *WorkerRepository) FindByAgentID(ctx context.Context, agentID string) (*models.Worker, error) {
	_ = agentID
	return nil, ErrNotFound
}

func (r *WorkerRepository) GetByUserIDAndIP(ctx context.Context, userID id.ID, ip string) ([]*models.Worker, error) {
	if ip == "" {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, workerSelectBase+` WHERE user_id = ? AND agent_ip = ?`, userID.Hex(), ip)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanWorkerRows(rows)
}

func (r *WorkerRepository) UpdateLastSeen(ctx context.Context, workerID id.ID) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET last_seen = ?, updated_at = ? WHERE id = ?`,
		timeMilli(now), timeMilli(now), workerID.Hex())
	return err
}

func (r *WorkerRepository) UpdateMetrics(ctx context.Context, workerID id.ID, metrics map[string]interface{}) error {
	mj, err := marshalIfaceMap(metrics)
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = r.db.ExecContext(ctx, `UPDATE workers SET metrics = ?, updated_at = ? WHERE id = ?`, mj, timeMilli(now), workerID.Hex())
	return err
}

func (r *WorkerRepository) UpdateAgentInfo(ctx context.Context, workerID id.ID, ipAddress, version string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE workers SET agent_ip = ?, agent_version = ?, last_seen = ?, updated_at = ? WHERE id = ?`,
		ipAddress, version, timeMilli(now), timeMilli(now), workerID.Hex())
	return err
}

func (r *WorkerRepository) ListByStatus(ctx context.Context, status string) ([]*models.Worker, error) {
	rows, err := r.db.QueryContext(ctx, workerSelectBase+` WHERE status = ?`, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanWorkerRows(rows)
}

func (r *WorkerRepository) UpdateStatusAndLastSeen(ctx context.Context, workerID id.ID, status string) error {
	now := time.Now()
	res, err := r.db.ExecContext(ctx, `UPDATE workers SET status = ?, last_seen = ?, updated_at = ? WHERE id = ?`,
		status, timeMilli(now), timeMilli(now), workerID.Hex())
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

func (r *WorkerRepository) UpdateHeartbeat(ctx context.Context, workerID id.ID, metrics map[string]interface{}) error {
	now := time.Now()
	setMetrics := len(metrics) > 0
	var res sql.Result
	var err error
	if setMetrics {
		mj, mErr := marshalIfaceMap(metrics)
		if mErr != nil {
			return mErr
		}
		res, err = r.db.ExecContext(ctx, `UPDATE workers SET status = ?, heartbeat_retry = 0, last_seen = ?, updated_at = ?, metrics = ? WHERE id = ?`,
			"alive", timeMilli(now), timeMilli(now), mj, workerID.Hex())
	} else {
		res, err = r.db.ExecContext(ctx, `UPDATE workers SET status = ?, heartbeat_retry = 0, last_seen = ?, updated_at = ? WHERE id = ?`,
			"alive", timeMilli(now), timeMilli(now), workerID.Hex())
	}
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

func (r *WorkerRepository) IncrementHeartbeatRetry(ctx context.Context, workerID id.ID) (int, error) {
	now := time.Now()
	var retry int
	err := r.db.QueryRowContext(ctx, `UPDATE workers SET heartbeat_retry = heartbeat_retry + 1, updated_at = ? WHERE id = ? RETURNING heartbeat_retry`,
		timeMilli(now), workerID.Hex()).Scan(&retry)
	if err != nil {
		return 0, mapErr(err)
	}
	return retry, nil
}

func (r *WorkerRepository) ListMonitored(ctx context.Context) ([]*models.Worker, error) {
	rows, err := r.db.QueryContext(ctx, workerSelectBase+` WHERE status IN ('alive', 'registered')`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanWorkerRows(rows)
}

// CountByUserIDAndStatusResult is one row from AggregateCountsByUserID.
type CountByUserIDAndStatusResult struct {
	UserID id.ID
	Alive  int
	Dead   int
	Total  int
}

func (r *WorkerRepository) AggregateCountsByUserID(ctx context.Context) ([]CountByUserIDAndStatusResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id,
			SUM(CASE WHEN status = 'alive' THEN 1 ELSE 0 END) AS alive,
			SUM(CASE WHEN status = 'dead' THEN 1 ELSE 0 END) AS dead,
			COUNT(*) AS total
		FROM workers
		WHERE user_id IS NOT NULL
		GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CountByUserIDAndStatusResult
	for rows.Next() {
		var row CountByUserIDAndStatusResult
		var uid sql.NullString
		if err := rows.Scan(&uid, &row.Alive, &row.Dead, &row.Total); err != nil {
			return nil, err
		}
		row.UserID = scanID(uid)
		out = append(out, row)
	}
	return out, rows.Err()
}
