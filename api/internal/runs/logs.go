package runs

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lute/api/internal/db/enums"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	pb "github.com/lute/proto"
)

const (
	defaultLogLimit = 200
	maxLogLimit     = 500
	logRPCDeadline  = 30 * time.Second
)

var (
	// ErrNoLogs means no worker has run the job, so there is no log to read.
	ErrNoLogs = errors.New("no worker has run this job yet; logs appear once it starts")
	// ErrWorkerOffline means the worker holding the log is not connected; core keeps no copy.
	ErrWorkerOffline = errors.New("the worker holding this log is not connected")
	// ErrLogRead wraps a failure the worker reported while reading the log.
	ErrLogRead = errors.New("read log from worker")
)

// LogQuery pages through a job log: Cursor is the NextCursor of the previous page.
type LogQuery struct {
	Direction string
	Limit     int
	Cursor    int64
}

// ParseLogQuery reads ?direction (tail|head), ?limit (capped at 500) and ?cursor.
func ParseLogQuery(direction, limit, cursor string) (LogQuery, error) {
	q := LogQuery{Direction: direction, Limit: defaultLogLimit}
	if q.Direction == "" {
		q.Direction = "tail"
	}
	if q.Direction != "tail" && q.Direction != "head" {
		return q, errors.New("direction must be tail or head")
	}
	if limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil || n < 1 {
			return q, errors.New("limit must be a positive number")
		}
		q.Limit = min(n, maxLogLimit)
	}
	if cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return q, errors.New("cursor must be the next_cursor of a previous page")
		}
		q.Cursor = n
	}
	return q, nil
}

// LogPage is the log endpoints' response body.
type LogPage struct {
	Lines      []string `json:"lines"`
	Direction  string   `json:"direction"`
	HasMore    bool     `json:"has_more"`
	FileSize   int64    `json:"file_size"`
	NextCursor string   `json:"next_cursor,omitempty"`
	// Error is a problem the worker hit reading the file; the page is still valid.
	Error string `json:"error,omitempty"`
}

// Logs reads a page of a job's log from the worker that holds it.
func (s *Service) Logs(ctx context.Context, jobID string, q LogQuery) (*LogPage, error) {
	workerID, err := s.logWorker(ctx, jobID)
	if err != nil {
		return nil, err
	}

	dir := pb.LogReadDirection_LOG_READ_TAIL
	if q.Direction == "head" {
		dir = pb.LogReadDirection_LOG_READ_HEAD
	}
	rpcCtx, cancel := context.WithTimeout(ctx, logRPCDeadline)
	defer cancel()
	resp, err := s.gateway.RequestJobLog(rpcCtx, workerID, &pb.JobLogRequest{
		JobId:        jobID,
		Direction:    dir,
		Limit:        int32(q.Limit),
		AnchorOffset: q.Cursor,
	})
	if errors.Is(err, grpc.ErrNoConnection) {
		return nil, ErrWorkerOffline
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLogRead, err)
	}

	page := &LogPage{
		Lines:     resp.GetLines(),
		Direction: q.Direction,
		HasMore:   resp.GetHasMore(),
		FileSize:  resp.GetFileSize(),
		Error:     resp.GetError(),
	}
	if page.Lines == nil {
		page.Lines = []string{}
	}
	if resp.GetNextAnchor() != 0 || page.HasMore {
		page.NextCursor = strconv.FormatInt(resp.GetNextAnchor(), 10)
	}
	return page, nil
}

// logWorker finds the worker holding a job's log: the one running it, else the one that
// finished it, else the one it was dispatched to before its execution record landed.
func (s *Service) logWorker(ctx context.Context, jobID string) (string, error) {
	job, err := s.queue.GetJob(ctx, jobID)
	if err != nil && !errors.Is(err, repos.ErrNotFound) {
		return "", err
	}
	if job != nil && job.Status == enums.QueueJobRunning && job.WorkerID != "" {
		return job.WorkerID, nil
	}

	exec, err := s.executions.GetByJobID(ctx, jobID)
	switch {
	case err == nil && exec.WorkerID != "":
		return exec.WorkerID, nil
	case err != nil && !errors.Is(err, repos.ErrNotFound):
		return "", err
	}

	if job != nil && job.WorkerID != "" {
		return job.WorkerID, nil
	}
	if job == nil && exec == nil {
		return "", repos.ErrNotFound
	}
	return "", ErrNoLogs
}
