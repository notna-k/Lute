// Package runs starts, retries and cancels builds and reads their logs, for every HTTP surface.
package runs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/queue"
	pb "github.com/lute/proto"
)

// Gateway reaches connected agents; *grpc.Server is the real one.
type Gateway interface {
	DispatchQueue(ctx context.Context, queue string)
	RequestJobLog(ctx context.Context, workerID string, req *pb.JobLogRequest) (*pb.JobLogResponse, error)
}

type Service struct {
	queue      *queue.Engine
	stats      *queue.Stats
	runs       *repos.RunRepository
	executions *repos.JobExecutionRepository
	gateway    Gateway
}

func New(q *queue.Engine, stats *queue.Stats, runs *repos.RunRepository, executions *repos.JobExecutionRepository, gw Gateway) *Service {
	return &Service{queue: q, stats: stats, runs: runs, executions: executions, gateway: gw}
}

// Job is what goes on the queue for a run.
type Job struct {
	Payload  json.RawMessage
	Selector map[string]string
	Opts     queue.EnqueueOpts
}

// Enqueue records run and queues its job. If the user already used run.IdempotencyKey,
// the earlier run is returned instead and created is false.
func (s *Service) Enqueue(ctx context.Context, run *models.Run, job Job) (_ *models.Run, created bool, _ error) {
	if run.IdempotencyKey != "" {
		existing, err := s.runs.GetByIdempotency(ctx, run.UserID, run.IdempotencyKey)
		if err == nil {
			return existing, false, nil
		}
		if !errors.Is(err, repos.ErrNotFound) {
			return nil, false, err
		}
	}

	run.JobID = uuid.New().String()
	if err := s.runs.Create(ctx, run); err != nil {
		return nil, false, err
	}
	meta := map[string]string{"user_id": run.UserID.Hex(), "run_id": run.ID.Hex()}
	if run.JobSlug != "" {
		meta["job_slug"] = run.JobSlug
	}
	err := s.push(ctx, &queue.Job{
		ID:       run.JobID,
		Queue:    run.Queue,
		Type:     run.Type,
		Payload:  job.Payload,
		Meta:     meta,
		Selector: job.Selector,
	}, job.Opts)
	if err != nil {
		return nil, false, err
	}
	return run, true, nil
}

// Retry queues a job again from its first attempt.
func (s *Service) Retry(ctx context.Context, jobID string) (*queue.Job, error) {
	job, err := s.queue.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	job.Attempts = 0
	job.Error = ""
	job.DoneAt = 0
	job.WorkerID = ""
	if err := s.push(ctx, job, queue.EnqueueOpts{MaxRetries: job.MaxRetries, TimeoutSec: job.TimeoutSec}); err != nil {
		return nil, err
	}
	return job, nil
}

// Cancel stops a job that has not started; see queue.ErrNotCancellable.
func (s *Service) Cancel(ctx context.Context, jobID string) error {
	return s.queue.CancelJob(ctx, jobID)
}

func (s *Service) push(ctx context.Context, job *queue.Job, opts queue.EnqueueOpts) error {
	if err := s.queue.Enqueue(ctx, job, opts); err != nil {
		return err
	}
	s.stats.RecordEnqueued(ctx, job.Queue)
	s.gateway.DispatchQueue(ctx, job.Queue)
	return nil
}

// Job returns a job's queue state, or repos.ErrNotFound once its slot is gone.
func (s *Service) Job(ctx context.Context, jobID string) (*queue.Job, error) {
	return s.queue.GetJob(ctx, jobID)
}

// Execution returns a finished job's record, or repos.ErrNotFound before it finishes.
func (s *Service) Execution(ctx context.Context, jobID string) (*models.JobExecution, error) {
	return s.executions.GetByJobID(ctx, jobID)
}

// Owned returns the run behind jobID if userID started it; a foreign run reads as missing.
func (s *Service) Owned(ctx context.Context, userID id.ID, jobID string) (*models.Run, error) {
	run, err := s.runs.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if run.UserID != userID {
		return nil, repos.ErrNotFound
	}
	return run, nil
}

func (s *Service) List(ctx context.Context, f repos.RunListFilter, offset, limit int64) ([]models.Run, int64, error) {
	return s.runs.List(ctx, f, offset, limit)
}

// History returns each slug's newest runs, newest first, and the execution records of
// those that finished, keyed by job id.
func (s *Service) History(ctx context.Context, userID id.ID, slugs []string, perSlug int) (map[string][]models.Run, map[string]*models.JobExecution, error) {
	bySlug, err := s.runs.ListByJobSlugs(ctx, userID, slugs, perSlug)
	if err != nil {
		return nil, nil, err
	}
	var jobIDs []string
	for _, runs := range bySlug {
		for i := range runs {
			jobIDs = append(jobIDs, runs[i].JobID)
		}
	}
	execs, err := s.executions.ListByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, nil, err
	}
	return bySlug, execs, nil
}
