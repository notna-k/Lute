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
	"github.com/lute/api/models"
	"github.com/lute/api/repository"
)

type Server struct {
	pb.UnimplementedAgentServiceServer
	config      *config.Config
	machineRepo *repository.MachineRepository
	commandRepo *repository.CommandRepository
	grpcServer  *grpc.Server
}

func NewServer(
	cfg *config.Config,
	machineRepo *repository.MachineRepository,
	commandRepo *repository.CommandRepository,
) *Server {
	return &Server{
		config:      cfg,
		machineRepo: machineRepo,
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

// RegisterAgent — agent connects for the first time
// If machine_id is provided, updates existing machine
// If machine_id is empty, creates a new machine
func (s *Server) RegisterAgent(ctx context.Context, req *pb.RegisterAgentRequest) (*pb.RegisterAgentResponse, error) {
	log.Printf("RegisterAgent: machine_id=%s version=%s ip=%s hostname=%s",
		req.MachineId, req.Version, req.IpAddress, req.Hostname)

	var machine *models.Machine
	var machineID primitive.ObjectID
	var err error

	// Check if machine_id is provided
	if req.MachineId != "" {
		// Existing machine - verify it exists
		machineID, err = primitive.ObjectIDFromHex(req.MachineId)
		if err != nil {
			return &pb.RegisterAgentResponse{
				Success: false,
				Message: fmt.Sprintf("invalid machine_id: %v", err),
			}, nil
		}

		machine, err = s.machineRepo.GetByID(ctx, machineID)
		if err != nil {
			return &pb.RegisterAgentResponse{
				Success: false,
				Message: fmt.Sprintf("machine not found: %v", err),
			}, nil
		}
	} else {
		// New machine - create it
		if req.IpAddress == "" {
			return &pb.RegisterAgentResponse{
				Success: false,
				Message: "ip_address is required for new machine registration",
			}, nil
		}

		// Build metadata from request
		metadata := make(map[string]interface{})
		for k, v := range req.Metadata {
			metadata[k] = v
		}
		metadata["ip"] = req.IpAddress

		// Create new machine
		machine = &models.Machine{
			Name:        fmt.Sprintf("%s:%s", req.Hostname, req.IpAddress),
			Description: fmt.Sprintf("Auto-registered agent from %s", req.IpAddress),
			Status:      "alive",
			Metadata:    metadata,
		}

		if err := s.machineRepo.Create(ctx, machine); err != nil {
			log.Printf("RegisterAgent: failed to create machine: %v", err)
			return &pb.RegisterAgentResponse{
				Success: false,
				Message: fmt.Sprintf("failed to create machine: %v", err),
			}, nil
		}

		machineID = machine.ID
		log.Printf("RegisterAgent: created new machine %s", machineID.Hex())
	}

	return &pb.RegisterAgentResponse{
		Success:       true,
		Message:       "Agent registered successfully",
		ServerVersion: "1.0.0",
		MachineId:     machineID.Hex(),
	}, nil
}

// Heartbeat — periodic keep-alive with optional system metrics
func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	// Parse machine ID from machine_id field
	machineID, err := primitive.ObjectIDFromHex(req.MachineId)
	if err != nil {
		log.Printf("Heartbeat: invalid machine_id %s: %v", req.MachineId, err)
		return &pb.HeartbeatResponse{
			Success:   false,
			Timestamp: time.Now().Unix(),
		}, nil
	}

	// Always set status to "alive" and update last_seen when we receive a heartbeat
	if err := s.machineRepo.UpdateStatusAndLastSeen(ctx, machineID, "alive"); err != nil {
		log.Printf("Heartbeat: failed to update status to alive for machine %s: %v", req.MachineId, err)
		return err
	}

	// Store metrics if provided
	if len(req.Metrics) > 0 {
		if err := s.machineRepo.UpdateMetrics(ctx, machineID, req.Metrics); err != nil {
			log.Printf("Heartbeat: failed to update metrics for machine %s: %v", req.MachineId, err)
		}
	}

	return &pb.HeartbeatResponse{
		Success:   true,
		Timestamp: time.Now().Unix(),
	}, nil
}

// UpdateMachineStatus — agent reports a state change (running/stopped/error)
func (s *Server) UpdateMachineStatus(ctx context.Context, req *pb.UpdateMachineStatusRequest) (*pb.UpdateMachineStatusResponse, error) {
	log.Printf("UpdateMachineStatus: machine_id=%s status=%s msg=%s",
		req.MachineId, req.Status, req.Message)

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

	return &pb.UpdateMachineStatusResponse{
		Success: true,
		Message: "Status updated",
	}, nil
}

// GetMachineConfig — agent polls for configuration + pending commands
func (s *Server) GetMachineConfig(ctx context.Context, req *pb.GetMachineConfigRequest) (*pb.GetMachineConfigResponse, error) {
	log.Printf("GetMachineConfig: machine_id=%s", req.MachineId)

	cfg := make(map[string]string)

	// Default config values
	cfg["heartbeat_interval"] = "30"
	cfg["log_level"] = "info"

	// Parse machine ID
	machineID, err := primitive.ObjectIDFromHex(req.MachineId)
	if err != nil {
		log.Printf("GetMachineConfig: invalid machine_id: %v", err)
		return &pb.GetMachineConfigResponse{
			Success: true,
			Config:  cfg,
			Message: "Configuration retrieved (invalid machine_id)",
		}, nil
	}

	// Fetch pending commands for this machine
	if s.commandRepo != nil {
		pending, err := s.commandRepo.GetPendingByMachineID(ctx, machineID)
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
	log.Printf("ExecuteCommand: machine_id=%s command=%s stage=%s",
		req.MachineId, req.Command, req.Env["stage"])

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
	log.Printf("StreamLogs: machine_id=%s level=%s",
		req.MachineId, req.Level)

	// Send an initial acknowledgement log message
	if err := stream.Send(&pb.LogMessage{
		Level:     "info",
		Message:   "Log stream connected",
		Timestamp: time.Now().Unix(),
		Source:    "server",
	}); err != nil {
		return err
	}

	// Parse machine ID
	machineID, err := primitive.ObjectIDFromHex(req.MachineId)
	if err != nil {
		log.Printf("StreamLogs: invalid machine_id: %v", err)
		return fmt.Errorf("invalid machine_id: %w", err)
	}

	// Keep stream open — poll for pending commands and push them as log events
	// This gives the agent a real-time channel for commands
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			log.Printf("StreamLogs: client %s disconnected", req.MachineId)
			return nil
		case <-ticker.C:
			// Check for pending commands and notify agent
			if s.commandRepo != nil {
				pending, err := s.commandRepo.GetPendingByMachineID(stream.Context(), machineID)
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
