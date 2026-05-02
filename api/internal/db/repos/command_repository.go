package repos

import (
	"context"
	"database/sql"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type CommandRepository struct {
	db *sql.DB
}

func NewCommandRepository(db *sql.DB) *CommandRepository {
	return &CommandRepository{db: db}
}

func (r *CommandRepository) Create(ctx context.Context, cmd *models.Command) error {
	cmd.BeforeCreate()
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	argsJ, err := marshalStringSlice(cmd.Args)
	if err != nil {
		return err
	}
	envJ, err := marshalStringMap(cmd.Env)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO commands (
		id, created_at, updated_at, worker_id, command, args, env, status, output, exit_code, error
	) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		cmd.ID.Hex(), timeMilli(cmd.CreatedAt), timeMilli(cmd.UpdatedAt), cmd.WorkerID.Hex(),
		cmd.Command, argsJ, envJ, cmd.Status, cmd.Output, cmd.ExitCode, cmd.Error,
	)
	return err
}

func (r *CommandRepository) GetByID(ctx context.Context, cid id.ID) (*models.Command, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, worker_id, command, args, env, status, output, exit_code, error
		FROM commands WHERE id = ?`, cid.Hex())
	return scanCommand(row)
}

func scanCommand(row *sql.Row) (*models.Command, error) {
	var c models.Command
	var ca, ua int64
	var idStr, wid string
	var argsJ, envJ sql.NullString
	if err := row.Scan(&idStr, &ca, &ua, &wid, &c.Command, &argsJ, &envJ, &c.Status, &c.Output, &c.ExitCode, &c.Error); err != nil {
		return nil, mapErr(err)
	}
	c.ID = id.ID(idStr)
	c.CreatedAt = time.UnixMilli(ca).UTC()
	c.UpdatedAt = time.UnixMilli(ua).UTC()
	c.WorkerID = id.ID(wid)
	var err error
	c.Args, err = unmarshalStringSlice(argsJ)
	if err != nil {
		return nil, err
	}
	c.Env, err = unmarshalStringMap(envJ)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CommandRepository) GetPendingByWorkerID(ctx context.Context, workerID id.ID) ([]*models.Command, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, worker_id, command, args, env, status, output, exit_code, error
		FROM commands WHERE worker_id = ? AND status = 'pending' ORDER BY created_at ASC`, workerID.Hex())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCommandRows(rows)
}

func (r *CommandRepository) GetByWorkerID(ctx context.Context, workerID id.ID, limit int64) ([]*models.Command, error) {
	q := `
		SELECT id, created_at, updated_at, worker_id, command, args, env, status, output, exit_code, error
		FROM commands WHERE worker_id = ? ORDER BY created_at DESC`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, q+` LIMIT ?`, workerID.Hex(), limit)
	} else {
		rows, err = r.db.QueryContext(ctx, q, workerID.Hex())
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanCommandRows(rows)
}

func scanCommandRows(rows *sql.Rows) ([]*models.Command, error) {
	var out []*models.Command
	for rows.Next() {
		var c models.Command
		var ca, ua int64
		var idStr, wid string
		var argsJ, envJ sql.NullString
		if err := rows.Scan(&idStr, &ca, &ua, &wid, &c.Command, &argsJ, &envJ, &c.Status, &c.Output, &c.ExitCode, &c.Error); err != nil {
			return nil, err
		}
		c.ID = id.ID(idStr)
		c.CreatedAt = time.UnixMilli(ca).UTC()
		c.UpdatedAt = time.UnixMilli(ua).UTC()
		c.WorkerID = id.ID(wid)
		var err error
		c.Args, err = unmarshalStringSlice(argsJ)
		if err != nil {
			return nil, err
		}
		c.Env, err = unmarshalStringMap(envJ)
		if err != nil {
			return nil, err
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

func (r *CommandRepository) UpdateResult(ctx context.Context, cid id.ID, status, output string, exitCode int, errMsg string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE commands SET status = ?, output = ?, exit_code = ?, error = ?, updated_at = ? WHERE id = ?`,
		status, output, exitCode, errMsg, timeMilli(now), cid.Hex())
	return err
}

func (r *CommandRepository) MarkRunning(ctx context.Context, cid id.ID) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `UPDATE commands SET status = ?, updated_at = ? WHERE id = ?`,
		"running", timeMilli(now), cid.Hex())
	return err
}
