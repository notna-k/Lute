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

	pb "github.com/lute/agent/proto/agent"
	"github.com/lute/api/config"
	"github.com/lute/api/repository"
)

type Server struct {
	pb.UnimplementedAgentServiceServer
	config      *config.Config
	machineRepo *repository.MachineRepository
	agentRepo   *repository.AgentRepository
	commandRepo *repository.CommandRepository
	grpcServer  *grpc.Server
}

func NewServer(
	cfg *config.Config,
	machineRepo *repository.MachineRepository,
	agentRepo *repository.AgentRepository,
	commandRepo *repository.CommandRepository,
) *Server {
	return &Server{
		config:      cfg,
		machineRepo: machineRepo,
		agentRepo:   agentRepo,
		commandRepo: commandRepo,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.GRPC.Host, s.config.GRPC.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterAgentServiceServer(s.grpcServer, s)

	// Enable reflection for testing with grpcurl
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

// RegisterAgent — agent connects for the first time in normal (daemon) mode
func (s *Server) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	log.Printf("RegisterAgent: agent_id=%s machine_id=%s version=%s ip=%s",
		req.AgentId, req.MachineId, req.Version, req.IpAddress)

	// 1. Verify the machine exists
	machineID, err := primitive.ObjectIDFromHex(req.MachineId)
	if err != nil {
		return &pb.RegisterAgentResponse{
			Success: false,
			Message: fmt.Sprintf("invalid machine_id: %v", err),
		}, nil
	}

	machine, err := s.machineRepo.GetByID(ctx, machineID)
	if err != nil {
		return &pb.RegisterAgentResponse{
			Success: false,
			Message: fmt.Sprintf("machine not found: %v", err),
		}, nil
	}

	// 2. Update agent record
	if err := s.agentRepo.UpdateStatus(ctx, req.AgentId, "connected"); err != nil {
		log.Printf("RegisterAgent: failed to update agent status: %v", err)
	}

	// 3. Update machine status to running
	if err := s.machineRepo.UpdateStatus(ctx, machine.ID, "running"); err != nil {
		log.Printf("RegisterAgent: failed to update machine status: %v", err)
	}

	return &pb.RegisterAgentResponse{
		Success:       true,
		Message:       "Agent registered successfully",
		ServerVersion: "1.0.0",
	}, nil
}

// Heartbeat — periodic keep-alive with optional system metrics
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	// Update agent last_seen
	if err := s.agentRepo.UpdateLastSeen(ctx, req.AgentId); err != nil {
		log.Printf("Heartbeat: failed to update last_seen for %s: %v", req.AgentId, err)
	}

	// Update agent status
	if req.Status != "" {
		if err := s.agentRepo.UpdateStatus(ctx, req.AgentId, req.Status); err != nil {
			log.Printf("Heartbeat: failed to update status for %s: %v", req.AgentId, err)
		}
	}

	// Store metrics if provided
	if len(req.Metrics) > 0 {
		if err := s.agentRepo.UpdateMetrics(ctx, req.AgentId, req.Metrics); err != nil {
			log.Printf("Heartbeat: failed to update metrics for %s: %v", req.AgentId, err)
		}
	}

	return &pb.HeartbeatResponse{
		Success:   true,
		Timestamp: time.Now().Unix(),
	}, nil
}

// UpdateMachineStatus — agent reports a state change (running/stopped/error)
func (s *Server) UpdateMachineStatus(ctx context.Context, req *pb.UpdateMachineStatusRequest) (*pb.UpdateMachineStatusResponse, error) {
	log.Printf("UpdateMachineStatus: agent_id=%s machine_id=%s status=%s msg=%s",
		req.AgentId, req.MachineId, req.Status, req.Message)

	machineID, err := primitive.ObjectIDFromHex(req.MachineId)
	if err != nil {
		return &pb.UpdateMachineStatusResponse{
			Success: false,
			Message: fmt.Sprintf("invalid machine_id: %v", err),
		}, nil
	}

	if err := s.machineRepo.UpdateStatus(ctx, machineID, req.Status); err != nil {
		log.Printf("UpdateMachineStatus: DB error: %v", err)
		return &pb.UpdateMachineStatusResponse{
			Success: false,
			Message: fmt.Sprintf("failed to update status: %v", err),
		}, nil
	}

	// Also update agent status to match
	if err := s.agentRepo.UpdateStatus(ctx, req.AgentId, req.Status); err != nil {
		log.Printf("UpdateMachineStatus: failed to update agent status: %v", err)
	}

	return &pb.UpdateMachineStatusResponse{
		Success: true,
		Message: "Status updated",
	}, nil
}

// GetMachineConfig — agent polls for configuration + pending commands
func (s *Server) GetMachineConfig(ctx context.Context, req *pb.GetMachineConfigRequest) (*pb.GetMachineConfigResponse, error) {
	log.Printf("GetMachineConfig: agent_id=%s machine_id=%s", req.AgentId, req.MachineId)

	cfg := make(map[string]string)

	// Default config values
	cfg["heartbeat_interval"] = "30"
	cfg["log_level"] = "info"

	// Fetch pending commands for this agent
	if s.commandRepo != nil {
		pending, err := s.commandRepo.GetPendingByAgentID(ctx, req.AgentId)
		if err != nil {
			log.Printf("GetMachineConfig: failed to fetch pending commands: %v", err)
		} else if len(pending) > 0 {
			// Serialize pending commands into config
			type cmdInfo struct {
				ID      string   `json:"id"`
				Command string   `json:"command"`
				Args    []string `json:"args,omitempty"`
			}
			var cmds []cmdInfo
			for _, c := range pending {
				cmds = append(cmds, cmdInfo{
					ID:      c.ID.Hex(),
					Command: c.Command,
					Args:    c.Args,
				})
			}
			data, _ := json.Marshal(cmds)
			cfg["pending_commands"] = string(data)
			cfg["pending_commands_count"] = fmt.Sprintf("%d", len(cmds))
		}
	}

	return &pb.GetMachineConfigResponse{
		Success: true,
		Config:  cfg,
		Message: "Configuration retrieved",
	}, nil
}

// ExecuteCommand — agent reports a command pick-up or result
// env["stage"] = "start" → mark running, "done" → store result
func (s *Server) ExecuteCommand(ctx context.Context, req *pb.ExecuteCommandRequest) (*pb.ExecuteCommandResponse, error) {
	log.Printf("ExecuteCommand: agent_id=%s machine_id=%s command=%s stage=%s",
		req.AgentId, req.MachineId, req.Command, req.Env["stage"])

	if s.commandRepo == nil {
		return &pb.ExecuteCommandResponse{Success: true}, nil
	}

	cmdIDStr, hasCmdID := req.Env["command_id"]
	if !hasCmdID {
		return &pb.ExecuteCommandResponse{Success: true}, nil
	}

	cmdID, err := primitive.ObjectIDFromHex(cmdIDStr)
	if err != nil {
		return &pb.ExecuteCommandResponse{Success: false, Error: "invalid command_id"}, nil
	}

	stage := req.Env["stage"]
	switch stage {
	case "start":
		_ = s.commandRepo.MarkRunning(ctx, cmdID)
	case "done":
		exitCode := 0
		if ec, ok := req.Env["exit_code"]; ok {
			fmt.Sscanf(ec, "%d", &exitCode)
		}
		status := "completed"
		if exitCode != 0 {
			status = "failed"
		}
		_ = s.commandRepo.UpdateResult(ctx, cmdID, status, req.Env["output"], exitCode, req.Env["error"])
		log.Printf("Command %s finished: exit=%d", cmdIDStr, exitCode)
	}

	return &pb.ExecuteCommandResponse{
		Success:  true,
		Output:   "",
		ExitCode: 0,
	}, nil
}

// StreamLogs — server pushes events/logs to the agent (server-streaming)
func (s *Server) StreamLogs(req *pb.StreamLogsRequest, stream pb.AgentService_StreamLogsServer) error {
	log.Printf("StreamLogs: agent_id=%s machine_id=%s level=%s",
		req.AgentId, req.MachineId, req.Level)

	// Send an initial acknowledgement log message
	if err := stream.Send(&pb.LogMessage{
		Level:     "info",
		Message:   "Log stream connected",
		Timestamp: time.Now().Unix(),
		Source:    "server",
	}); err != nil {
		return err
	}

	// Keep stream open — poll for pending commands and push them as log events
	// This gives the agent a real-time channel for commands
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("StreamLogs: client %s disconnected", req.AgentId)
			return nil
		case <-ticker.C:
			// Check for pending commands and notify agent
			if s.commandRepo != nil {
				pending, err := s.commandRepo.GetPendingByAgentID(stream.Context(), req.AgentId)
				if err == nil && len(pending) > 0 {
					for _, cmd := range pending {
						msg := fmt.Sprintf("PENDING_CMD:%s:%s", cmd.ID.Hex(), cmd.Command)
						if err := stream.Send(&pb.LogMessage{
							Level:     "info",
							Message:   msg,
							Timestamp: time.Now().Unix(),
							Source:    "command_queue",
						}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
}
