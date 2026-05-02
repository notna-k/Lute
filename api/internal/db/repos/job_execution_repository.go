package repos

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
)

type JobExecutionRepository struct {
	db *sql.DB
}

func NewJobExecutionRepository(db *sql.DB) *JobExecutionRepository {
	return &JobExecutionRepository{db: db}
}

func (r *JobExecutionRepository) Upsert(ctx context.Context, exec *models.JobExecution) error {
	now := time.Now()
	exec.UpdatedAt = now
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = now
	}
	if exec.ID.IsZero() {
		exec.ID = id.New()
	}
	succ := 0
	if exec.Success {
		succ = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO job_executions (
			id, created_at, updated_at, job_id, worker_id, queue, type, success, error,
			elapsed_ms, log_file, execution_log_file, finished_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(job_id) DO UPDATE SET
			updated_at = excluded.updated_at,
			worker_id = excluded.worker_id,
			queue = excluded.queue,
			type = excluded.type,
			success = excluded.success,
			error = excluded.error,
			elapsed_ms = excluded.elapsed_ms,
			log_file = excluded.log_file,
			execution_log_file = excluded.execution_log_file,
			finished_at = excluded.finished_at
	`,
		exec.ID.Hex(), timeMilli(exec.CreatedAt), timeMilli(exec.UpdatedAt), exec.JobID, exec.WorkerID,
		exec.Queue, exec.Type, succ, exec.Error, exec.ElapsedMs, exec.LogFile, exec.ExecutionLogFile, timeMilli(exec.FinishedAt),
	)
	return err
}

func (r *JobExecutionRepository) GetByJobID(ctx context.Context, jobID string) (*models.JobExecution, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, job_id, worker_id, queue, type, success, error,
			elapsed_ms, log_file, execution_log_file, finished_at
		FROM job_executions WHERE job_id = ?`, jobID)
	return scanJobExecution(row)
}

func scanJobExecution(row *sql.Row) (*models.JobExecution, error) {
	var e models.JobExecution
	var ca, ua, fin int64
	var idStr string
	var succ int
	if err := row.Scan(&idStr, &ca, &ua, &e.JobID, &e.WorkerID, &e.Queue, &e.Type, &succ, &e.Error,
		&e.ElapsedMs, &e.LogFile, &e.ExecutionLogFile, &fin); err != nil {
		return nil, mapErr(err)
	}
	e.ID = id.ID(idStr)
	e.CreatedAt = time.UnixMilli(ca).UTC()
	e.UpdatedAt = time.UnixMilli(ua).UTC()
	e.Success = succ != 0
	e.FinishedAt = time.UnixMilli(fin).UTC()
	return &e, nil
}

type JobExecutionListFilter struct {
	Queue  string
	Type   string
	Status string // "", "success", or "failed"
}

func (r *JobExecutionRepository) List(ctx context.Context, filter JobExecutionListFilter, offset, limit int64, sortDesc bool) ([]models.JobExecution, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	where, args := jobExecWhereArgs(filter)
	countQ := `SELECT COUNT(*) FROM job_executions` + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count job executions: %w", err)
	}

	order := "ASC"
	if sortDesc {
		order = "DESC"
	}
	q := fmt.Sprintf(`
		SELECT id, created_at, updated_at, job_id, worker_id, queue, type, success, error,
			elapsed_ms, log_file, execution_log_file, finished_at
		FROM job_executions%s ORDER BY finished_at %s LIMIT ? OFFSET ?`, where, order)
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("find job executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []models.JobExecution
	for rows.Next() {
		var e models.JobExecution
		var ca, ua, fin int64
		var idStr string
		var succ int
		if err := rows.Scan(&idStr, &ca, &ua, &e.JobID, &e.WorkerID, &e.Queue, &e.Type, &succ, &e.Error,
			&e.ElapsedMs, &e.LogFile, &e.ExecutionLogFile, &fin); err != nil {
			return nil, 0, err
		}
		e.ID = id.ID(idStr)
		e.CreatedAt = time.UnixMilli(ca).UTC()
		e.UpdatedAt = time.UnixMilli(ua).UTC()
		e.Success = succ != 0
		e.FinishedAt = time.UnixMilli(fin).UTC()
		out = append(out, e)
	}
	if out == nil {
		out = []models.JobExecution{}
	}
	return out, total, rows.Err()
}

func jobExecWhereArgs(f JobExecutionListFilter) (where string, args []any) {
	var conds []string
	if f.Queue != "" {
		conds = append(conds, "queue = ?")
		args = append(args, f.Queue)
	}
	if f.Type != "" {
		conds = append(conds, "type = ?")
		args = append(args, f.Type)
	}
	switch f.Status {
	case "success":
		conds = append(conds, "success = 1")
	case "failed":
		conds = append(conds, "success = 0")
	}
	if len(conds) == 0 {
		return "", args
	}
	w := " WHERE "
	for i, c := range conds {
		if i > 0 {
			w += " AND "
		}
		w += c
	}
	return w, args
}

func (r *JobExecutionRepository) DistinctQueuesAndTypes(ctx context.Context) (queues, types []string, err error) {
	rawQ, err := r.distinctCol(ctx, "queue")
	if err != nil {
		return nil, nil, err
	}
	rawT, err := r.distinctCol(ctx, "type")
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(rawQ)
	sort.Strings(rawT)
	return rawQ, rawT, nil
}

func (r *JobExecutionRepository) distinctCol(ctx context.Context, col string) ([]string, error) {
	q := fmt.Sprintf(`SELECT DISTINCT %s FROM job_executions`, col)
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	seen := make(map[string]struct{})
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out, rows.Err()
}
