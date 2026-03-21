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

func ParseMachineID(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}

type Server struct {
	pb.UnimplementedWorkerServiceServer
	config                 *config.Config
	machineRepo            *repos.MachineRepository
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
	machineRepo *repos.MachineRepository,
	jobExecRepo *repos.JobExecutionRepository,
	queueEngine *queue.Engine,
	statsAgg *queue.StatsAggregator,
	hub *websocket.Hub,
) *Server {
	return &Server{
		config:      cfg,
		machineRepo: machineRepo,
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
// The first message must carry the machine_id. After registration in the
// ConnectionManager, Run() takes over: it handles heartbeat pings, job
// assignments, and incoming results/pongs.
func (s *Server) Connect(stream pb.WorkerService_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("connect: failed to receive initial message: %w", err)
	}

	machineID := first.GetMachineId()
	if machineID == "" {
		return fmt.Errorf("connect: machine_id is required in the first message")
	}

	mid, err := ParseMachineID(machineID)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	machine, err := s.machineRepo.GetByID(stream.Context(), mid)
	if err != nil {
		return fmt.Errorf("connect: machine %s not found: %w", machineID, err)
	}
	if machine.Status == "dead" {
		return fmt.Errorf("connect: machine %s is dead; set status to pending to re-enable", machineID)
	}

	if machine.Status == "pending" {
		if err := s.machineRepo.UpdateStatus(stream.Context(), mid, "registered"); err != nil {
			log.Printf("Connect: failed to set machine %s to registered: %v", machineID, err)
		} else {
			machine.Status = "registered"
		}
	}

	log.Printf("Connect: worker %s connected", machineID)

	conn := s.ConnMgr.Register(machineID, stream)
	if s.OnConnectionRegistered != nil {
		s.OnConnectionRegistered()
	}
	defer func() {
		s.ConnMgr.Unregister(machineID)
		log.Printf("Connect: worker %s disconnected", machineID)
	}()

	conn.Run(s.handleJobResult, s.handleWorkerRegistration)
	return nil
}

func (s *Server) handleJobResult(machineID string, result *pb.JobResult) {
	ctx := context.Background()
	if result.Success {
		if err := s.queueEngine.Complete(ctx, result.JobId, result.ElapsedMs); err != nil {
			log.Printf("handleJobResult: complete %s: %v", result.JobId, err)
		}
		job, _ := s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordProcessed(ctx, job.Queue, result.ElapsedMs)
			s.broadcastJobEvent("completed", job)
		}
	} else {
		if err := s.queueEngine.Fail(ctx, result.JobId, result.Error); err != nil {
			log.Printf("handleJobResult: fail %s: %v", result.JobId, err)
		}
		job, _ := s.queueEngine.GetJob(ctx, result.JobId)
		if job != nil {
			s.statsAgg.RecordFailed(ctx, job.Queue)
			s.broadcastJobEvent("failed", job)
		}
	}

	s.persistExecution(ctx, machineID, result)
}

func (s *Server) persistExecution(ctx context.Context, machineID string, result *pb.JobResult) {
	if s.jobExecRepo == nil {
		return
	}

	job, _ := s.queueEngine.GetJob(ctx, result.JobId)

	exec := &models.JobExecution{
		JobID:            result.JobId,
		MachineID:        machineID,
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

func (s *Server) handleWorkerRegistration(machineID string, reg *pb.WorkerRegistration) {
	log.Printf("Worker %s registered: queues=%v concurrency=%d", machineID, reg.Queues, reg.Concurrency)
}

// DispatchJob attempts to assign a pending job to an available worker.
func (s *Server) DispatchJob(ctx context.Context, queueName string) bool {
	worker := s.ConnMgr.FindAvailableWorker(queueName)
	if worker == nil {
		connected := s.ConnMgr.ConnectedMachineIDs()
		log.Printf("DispatchJob queue=%s: no available worker (connected: %v)", queueName, connected)
		return false
	}

	job, err := s.queueEngine.Dequeue(ctx, queueName)
	if err != nil {
		log.Printf("DispatchJob queue=%s: dequeue error: %v", queueName, err)
		return false
	}
	if job == nil {
		log.Printf("DispatchJob queue=%s: no pending job in queue", queueName)
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
		log.Printf("DispatchJob: worker %s rejected assignment for job %s", worker.MachineID, job.ID)
		return false
	}

	log.Printf("DispatchJob: assigned job %s to worker %s", job.ID, worker.MachineID)
	s.broadcastJobEvent("started", job)
	return true
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
