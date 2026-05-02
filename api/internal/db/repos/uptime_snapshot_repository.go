package repos

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type UptimeSnapshotRepository struct {
	db *sql.DB
}

func NewUptimeSnapshotRepository(db *sql.DB) *UptimeSnapshotRepository {
	return &UptimeSnapshotRepository{db: db}
}

func (r *UptimeSnapshotRepository) Insert(ctx context.Context, userID id.ID, at time.Time, alive, dead, total int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO uptime_snapshots (user_id, at, alive, dead, total) VALUES (?,?,?,?,?)`,
		userID.Hex(), timeMilli(at), alive, dead, total)
	return err
}

func (r *UptimeSnapshotRepository) GetByUserID(ctx context.Context, userID id.ID, since time.Time) ([]*models.UptimeSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, at, alive, dead, total FROM uptime_snapshots
		WHERE user_id = ? AND at >= ? ORDER BY at ASC`, userID.Hex(), timeMilli(since))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*models.UptimeSnapshot
	for rows.Next() {
		var s models.UptimeSnapshot
		var uid string
		var atMs int64
		if err := rows.Scan(&uid, &atMs, &s.Alive, &s.Dead, &s.Total); err != nil {
			return nil, err
		}
		s.UserID = id.ID(uid)
		s.At = time.UnixMilli(atMs).UTC()
		out = append(out, &s)
	}
	return out, rows.Err()
}

// PruneOlderThan deletes uptime snapshots with timestamp before cutoff (retention).
func (r *UptimeSnapshotRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM uptime_snapshots WHERE at < ?`, timeMilli(cutoff))
	return err
}

type WorkerSnapshotRepository struct {
	db *sql.DB
}

func NewWorkerSnapshotRepository(db *sql.DB) *WorkerSnapshotRepository {
	return &WorkerSnapshotRepository{db: db}
}

func (r *WorkerSnapshotRepository) Insert(ctx context.Context, workerID id.ID, at time.Time, metrics map[string]interface{}) error {
	mj, err := marshalIfaceMap(metrics)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO worker_snapshots (worker_id, at, metrics) VALUES (?,?,?)`,
		workerID.Hex(), timeMilli(at), mj)
	return err
}

func (r *WorkerSnapshotRepository) GetByWorkerID(ctx context.Context, workerID id.ID, since time.Time) ([]*models.WorkerSnapshot, error) {
	return r.GetByWorkerIDs(ctx, []id.ID{workerID}, since)
}

func (r *WorkerSnapshotRepository) GetByWorkerIDs(ctx context.Context, workerIDs []id.ID, since time.Time) ([]*models.WorkerSnapshot, error) {
	if len(workerIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(workerIDs))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, 0, len(workerIDs)+1)
	for _, wid := range workerIDs {
		args = append(args, wid.Hex())
	}
	args = append(args, timeMilli(since))
	q := `SELECT worker_id, at, metrics FROM worker_snapshots WHERE worker_id IN (` + placeholders + `) AND at >= ? ORDER BY at ASC`
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*models.WorkerSnapshot
	for rows.Next() {
		var s models.WorkerSnapshot
		var wid string
		var atMs int64
		var mj sql.NullString
		if err := rows.Scan(&wid, &atMs, &mj); err != nil {
			return nil, err
		}
		s.WorkerID = id.ID(wid)
		s.At = time.UnixMilli(atMs).UTC()
		var uerr error
		s.Metrics, uerr = unmarshalIfaceMap(mj)
		if uerr != nil {
			return nil, uerr
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

// PruneOlderThan deletes worker snapshots with timestamp before cutoff (retention).
func (r *WorkerSnapshotRepository) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM worker_snapshots WHERE at < ?`, timeMilli(cutoff))
	return err
}
