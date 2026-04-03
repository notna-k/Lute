package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/lute/proto"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/websocket"
)

func ParseWorkerID(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}

type Server struct {
	pb.UnimplementedWorkerServiceServer
	config                 *config.Config
	workerRepo             *repos.WorkerRepository
	jobExecRepo            *repos.JobExecutionRepository
	queueEngine            *queue.Engine
	statsAgg               *queue.StatsAggregator
	hub                    *websocket.Hub
	ConnMgr                *ConnectionManager
	grpcServer             *grpc.Server
	OnConnectionRegistered func()
}

func NewServer(
	cfg *config.Config,
	workerRepo *repos.WorkerRepository,
	jobExecRepo *repos.JobExecutionRepository,
	queueEngine *queue.Engine,
	statsAgg *queue.StatsAggregator,
	hub *websocket.Hub,
) *Server {
	return &Server{
		config:     cfg,
		workerRepo: workerRepo,
		jobExecRepo: jobExecRepo,
		queueEngine: queueEngine,
		statsAgg:    statsAgg,
		hub:         hub,
		ConnMgr:     NewConnectionManager(),
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.GRPC.Host, s.config.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterWorkerServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	log.Printf("gRPC server listening on %s", addr)

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve gRPC: %w", err)
	}
	return nil
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
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
		return fmt.Errorf("connect: worker %s not found: %w", workerID, err)
	}
	if w.Status == "dead" {
		return fmt.Errorf("connect: worker %s is dead; set status to pending to re-enable", workerID)
	}

	if w.Status == "pending" {
		if err := s.workerRepo.UpdateStatus(stream.Context(), wid, "registered"); err != nil {
			log.Printf("Connect: failed to set worker %s to registered: %v", workerID, err)
		} else {
			w.Status = "registered"
		}
	}

	log.Printf("Connect: worker %s connected", workerID)

	conn := s.ConnMgr.Register(workerID, stream)
	if s.OnConnectionRegistered != nil {
		s.OnConnectionRegistered()
	}
	defer func() {
		s.ConnMgr.Unregister(workerID)
		log.Printf("Connect: worker %s disconnected", workerID)
	}()

	conn.Run(s.handleJobResult, s.handleWorkerRegistration)
	return nil
}

func (s *Server) handleJobResult(workerID string, result *pb.JobResult) {
	ctx := context.Background()
	var job *queue.Job
	if result.Success {
		if err := s.queueEngine.Complete(ctx, result.JobId, result.ElapsedMs); err != nil {
			log.Printf("handleJobResult: complete %s: %v", result.JobId, err)
		}
		job, _ = s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordProcessed(ctx, job.Queue, result.ElapsedMs)
			s.broadcastJobEvent("completed", job)
		}
	} else {
		if err := s.queueEngine.Fail(ctx, result.JobId, result.Error); err != nil {
			log.Printf("handleJobResult: fail %s: %v", result.JobId, err)
		}
		job, _ = s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordFailed(ctx, job.Queue)
			s.broadcastJobEvent("failed", job)
		}
	}

	s.persistExecution(ctx, workerID, result)

	// Pull more pending work now that this worker has a free slot.
	if job != nil {
		s.DispatchQueue(ctx, job.Queue)
	}
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
		FinishedAt:       time.Now(),
	}
	if job != nil {
		exec.Queue = job.Queue
		exec.Type = job.Type
	}

	if err := s.jobExecRepo.Upsert(ctx, exec); err != nil {
		log.Printf("handleJobResult: persist execution %s: %v", result.JobId, err)
	}
}

func (s *Server) handleWorkerRegistration(workerID string, reg *pb.WorkerRegistration) {
	log.Printf("Worker %s registered: queues=%v concurrency=%d", workerID, reg.Queues, reg.Concurrency)
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
func (s *Server) DispatchJob(ctx context.Context, queueName string) bool {
	worker := s.ConnMgr.FindAvailableWorker(queueName)
	if worker == nil {
		return false
	}

	job, err := s.queueEngine.Dequeue(ctx, queueName)
	if err != nil {
		log.Printf("DispatchJob queue=%s: dequeue error: %v", queueName, err)
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
		log.Printf("DispatchJob: worker %s rejected assignment for job %s", worker.WorkerID, job.ID)
		return false
	}

	if err := s.queueEngine.SetWorkerID(ctx, job.ID, worker.WorkerID); err != nil {
		log.Printf("DispatchJob: set worker id for job %s: %v", job.ID, err)
	}

	log.Printf("DispatchJob: assigned job %s to worker %s", job.ID, worker.WorkerID)
	s.broadcastJobEvent("started", job)
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
