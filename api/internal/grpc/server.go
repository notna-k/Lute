package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/id"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/db/types"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/websocket"
	pb "github.com/lute/proto"
)

func ParseWorkerID(hex string) (id.ID, error) {
	return id.FromHex(hex)
}

// WebhookEmitter is the narrow interface the gRPC layer uses to fire run events.
// It is satisfied by *webhooks.Emitter and can be nil to disable emission.
type WebhookEmitter interface {
	Emit(ctx context.Context, jobID, event string, payload map[string]interface{})
}

type Server struct {
	pb.UnimplementedWorkerServiceServer
	config                 *config.Config
	workerRepo             *repos.WorkerRepository
	jobExecRepo            *repos.JobExecutionRepository
	queueEngine            *queue.Engine
	statsAgg               *queue.Stats
	hub                    *websocket.Hub
	ConnMgr                *ConnectionManager
	grpcServer             *grpc.Server
	listener               net.Listener
	OnConnectionRegistered func()
	WebhookEmitter         WebhookEmitter
}

func NewServer(
	cfg *config.Config,
	workerRepo *repos.WorkerRepository,
	jobExecRepo *repos.JobExecutionRepository,
	queueEngine *queue.Engine,
	statsAgg *queue.Stats,
	hub *websocket.Hub,
) *Server {
	return &Server{
		config:      cfg,
		workerRepo:  workerRepo,
		jobExecRepo: jobExecRepo,
		queueEngine: queueEngine,
		statsAgg:    statsAgg,
		hub:         hub,
		ConnMgr:     NewConnectionManager(),
	}
}

// Listen binds the configured address and registers the service, without serving yet.
// Splitting this from Serve lets a caller fail fast on a port conflict — and lets it
// ask for port 0 and then read the assigned port off Addr.
func (s *Server) Listen() error {
	if s.listener != nil {
		return nil
	}
	addr := fmt.Sprintf("%s:%s", s.config.GRPC.Host, s.config.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterWorkerServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	s.listener = lis
	return nil
}

// Addr is the address the server is listening on, or "" before Listen.
func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// Serve blocks serving the listener Listen bound. It returns nil after Stop.
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("serve: Listen was not called")
	}
	slog.Info("gRPC server listening", "addr", s.Addr())
	if err := s.grpcServer.Serve(s.listener); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}
	return nil
}

// gracefulStopTimeout bounds how long a shutdown waits for in-flight RPCs.
const gracefulStopTimeout = 5 * time.Second

// Stop shuts the server down, giving in-flight RPCs until the context's
// deadline (or gracefulStopTimeout, whichever is sooner) to finish.
//
// A worker's Connect stream lives for as long as the worker does, so a plain
// GracefulStop never returns: it waits for streams that only end when the other side
// hangs up. Waiting a little and then closing them is what lets core exit on a signal
// instead of hanging until something kills it.
func (s *Server) Stop(ctx context.Context) {
	if s.grpcServer == nil {
		return
	}

	timeout := gracefulStopTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("gRPC streams still open after grace period, closing them", "grace", timeout)
		s.grpcServer.Stop()
		<-done
	}
}

// Connect handles the bidirectional stream opened by a worker.
// The first message must carry worker_id.
func (s *Server) Connect(stream pb.WorkerService_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("connect: failed to receive initial message: %w", err)
	}

	workerID := first.GetWorkerId()
	if workerID == "" {
		return fmt.Errorf("connect: worker_id is required in the first message")
	}

	wid, err := ParseWorkerID(workerID)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	w, err := s.workerRepo.GetByID(stream.Context(), wid)
	if err != nil {
		if errors.Is(err, repos.ErrNotFound) {
			return status.Errorf(codes.NotFound, "worker %s not found (it may have been deleted)", workerID)
		}
		return fmt.Errorf("connect: worker %s lookup failed: %w", workerID, err)
	}
	if w.Status == "dead" {
		return status.Errorf(codes.FailedPrecondition, "worker %s is dead; set status to pending to re-enable", workerID)
	}

	if w.Status == "pending" {
		if err := s.workerRepo.UpdateStatus(stream.Context(), wid, "registered"); err != nil {
			slog.Error("mark worker registered", "worker_id", workerID, "err", err)
		} else {
			w.Status = "registered"
		}
	}

	slog.Info("worker connected", "worker_id", workerID)

	conn := s.ConnMgr.Register(workerID, stream)
	conn.Labels = w.Labels // seed in-memory labels from DB at connect time
	if s.OnConnectionRegistered != nil {
		s.OnConnectionRegistered()
	}
	defer func() {
		s.ConnMgr.Unregister(workerID)
		slog.Info("worker disconnected", "worker_id", workerID)
	}()

	conn.Run(s.handleJobResult, s.handleWorkerRegistration)
	return nil
}

func (s *Server) handleJobResult(workerID string, result *pb.JobResult) {
	ctx := context.Background()
	var job *queue.Job
	if result.Success {
		if err := s.queueEngine.Complete(ctx, result.JobId, result.ElapsedMs); err != nil {
			if errors.Is(err, queue.ErrJobNotRunning) {
				// The reaper already requeued this attempt; recording it would contradict the retry.
				slog.Warn("ignoring late success for reaped job", "job_id", result.JobId, "worker_id", workerID)
				return
			}
			slog.Error("complete job", "job_id", result.JobId, "err", err)
		}
		job, _ = s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordProcessed(ctx, job.Queue, result.ElapsedMs)
			s.broadcastJobEvent("completed", job)
		}
		s.emitWebhook(ctx, result.JobId, "run.completed", map[string]interface{}{
			"success":    true,
			"elapsed_ms": result.ElapsedMs,
			"worker_id":  workerID,
		})
	} else {
		if err := s.queueEngine.Fail(ctx, result.JobId, result.Error); err != nil {
			if errors.Is(err, queue.ErrJobNotRunning) {
				slog.Warn("ignoring late failure for reaped job", "job_id", result.JobId, "worker_id", workerID)
				return
			}
			slog.Error("fail job", "job_id", result.JobId, "err", err)
		}
		job, _ = s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordFailed(ctx, job.Queue)
			s.broadcastJobEvent("failed", job)
		}
		// Only fire the public webhook once the job is truly dead (DLQ), not on
		// per-attempt retries. Internal retries are an implementation detail.
		if job != nil && job.Status == "dead" {
			s.emitWebhook(ctx, result.JobId, "run.failed", map[string]interface{}{
				"success":   false,
				"error":     result.Error,
				"attempts":  job.Attempts,
				"worker_id": workerID,
			})
		}
	}

	s.persistExecution(ctx, workerID, result)

	// Pull more pending work now that this worker has a free slot.
	if job != nil {
		s.DispatchQueue(ctx, job.Queue)
	}
}

// HandleExpiredLeases fails builds the sweep gave up on, so they are retried or dead-lettered
// like any other failure instead of showing as running forever.
func (s *Server) HandleExpiredLeases(ctx context.Context, leases []queue.ExpiredLease) {
	queues := make(map[string]struct{}, len(leases))
	for _, lease := range leases {
		reason := fmt.Sprintf("worker %s stopped reporting before the job finished", lease.WorkerID)
		if lease.WorkerID == "" {
			reason = "the assigned worker stopped reporting before the job finished"
		}
		slog.Warn("failing job with expired lease", "job_id", lease.JobID, "reason", reason)

		if err := s.queueEngine.Fail(ctx, lease.JobID, reason); err != nil {
			if errors.Is(err, queue.ErrJobNotRunning) {
				continue // the worker's own result landed between the claim and here
			}
			slog.Error("fail job with expired lease", "job_id", lease.JobID, "err", err)
			continue
		}

		job, _ := s.queueEngine.GetJob(ctx, lease.JobID)
		if job != nil {
			s.statsAgg.RecordFailed(ctx, job.Queue)
			s.broadcastJobEvent("failed", job)
		}
		if job != nil && job.Status == "dead" {
			s.persistExecution(ctx, lease.WorkerID, &pb.JobResult{
				JobId:   lease.JobID,
				Success: false,
				Error:   reason,
			})
			s.emitWebhook(ctx, lease.JobID, "run.failed", map[string]interface{}{
				"success":   false,
				"error":     reason,
				"attempts":  job.Attempts,
				"worker_id": lease.WorkerID,
			})
		}
		queues[lease.Queue] = struct{}{}
	}

	// A reaped job frees capacity on whatever worker is still connected.
	for q := range queues {
		s.DispatchQueue(ctx, q)
	}
}

func (s *Server) emitWebhook(ctx context.Context, jobID, event string, payload map[string]interface{}) {
	if s.WebhookEmitter == nil {
		return
	}
	s.WebhookEmitter.Emit(ctx, jobID, event, payload)
}

func (s *Server) persistExecution(ctx context.Context, workerID string, result *pb.JobResult) {
	if s.jobExecRepo == nil {
		return
	}

	job, _ := s.queueEngine.GetJob(ctx, result.JobId)

	exec := &models.JobExecution{
		JobID:            result.JobId,
		WorkerID:         workerID,
		Success:          result.Success,
		Error:            result.Error,
		ElapsedMs:        result.ElapsedMs,
		LogFile:          result.LogFile,
		ExecutionLogFile: result.ExecutionLogFile,
		FinishedAt:       types.NewMilliTime(time.Now()),
	}
	if job != nil {
		exec.Queue = job.Queue
		exec.Type = job.Type
	}

	if err := s.jobExecRepo.Upsert(ctx, exec); err != nil {
		slog.Error("persist job execution", "job_id", result.JobId, "err", err)
	}
}

func (s *Server) handleWorkerRegistration(workerID string, reg *pb.WorkerRegistration) {
	slog.Info("worker registered", "worker_id", workerID, "queues", reg.Queues, "concurrency", reg.Concurrency)
	ctx := context.Background()
	for _, q := range reg.Queues {
		s.DispatchQueue(ctx, q)
	}
}

// DispatchQueue assigns pending jobs from the queue to available workers until
// no worker can take work or the queue is empty.
func (s *Server) DispatchQueue(ctx context.Context, queueName string) {
	for s.DispatchJob(ctx, queueName) {
	}
}

// DispatchJob attempts to assign a pending job to an available worker.
// It peeks the next job first to read its selector, then finds a matching worker,
// and only dequeues once a match is confirmed.
func (s *Server) DispatchJob(ctx context.Context, queueName string) bool {
	peeked, err := s.queueEngine.PeekNextReadyJob(ctx, queueName)
	if err != nil {
		slog.Error("peek next job", "queue", queueName, "err", err)
		return false
	}
	if peeked == nil {
		return false
	}

	worker := s.ConnMgr.FindAvailableWorker(queueName, peeked.Selector)
	if worker == nil {
		if len(peeked.Selector) > 0 {
			slog.Debug("no eligible worker for job", "queue", queueName, "selector", peeked.Selector)
		}
		return false
	}

	job, err := s.queueEngine.Dequeue(ctx, queueName)
	if err != nil {
		slog.Error("dequeue job", "queue", queueName, "err", err)
		return false
	}
	if job == nil {
		return false
	}

	assignment := &pb.JobAssignment{
		JobId:      job.ID,
		Queue:      job.Queue,
		Type:       job.Type,
		Payload:    job.Payload,
		TimeoutSec: int32(job.TimeoutSec),
	}

	if !worker.AssignJob(assignment) {
		_ = s.queueEngine.Fail(ctx, job.ID, "worker rejected assignment")
		slog.Warn("worker rejected job assignment", "worker_id", worker.WorkerID, "job_id", job.ID)
		return false
	}

	if err := s.queueEngine.SetWorkerID(ctx, job.ID, worker.WorkerID); err != nil {
		slog.Error("record job worker", "job_id", job.ID, "err", err)
	}

	slog.Info("assigned job", "job_id", job.ID, "worker_id", worker.WorkerID)
	s.broadcastJobEvent("started", job)
	s.emitWebhook(ctx, job.ID, "run.started", map[string]interface{}{
		"worker_id": worker.WorkerID,
		"attempts":  job.Attempts,
	})
	return true
}

// RequestJobLog asks a connected worker to read a chunk of a job log file.
func (s *Server) RequestJobLog(ctx context.Context, workerID string, req *pb.JobLogRequest) (*pb.JobLogResponse, error) {
	conn := s.ConnMgr.Get(workerID)
	if conn == nil {
		return nil, ErrNoConnection
	}
	return conn.RequestJobLog(ctx, req)
}

func (s *Server) broadcastJobEvent(eventType string, job *queue.Job) {
	if s.hub == nil {
		return
	}
	data, _ := json.Marshal(map[string]interface{}{
		"type": "job_" + eventType,
		"job":  job,
	})
	s.hub.Broadcast(data)
}
