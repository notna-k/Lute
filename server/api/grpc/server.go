package grpc

import (
	"fmt"
	"log"
	"net"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/lute/agent/proto/agent"
	"github.com/lute/api/config"
	"github.com/lute/api/repository"
)

func ParseMachineID(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}

type Server struct {
	pb.UnimplementedAgentServiceServer
	config                 *config.Config
	machineRepo            *repository.MachineRepository
	ConnMgr                *ConnectionManager
	grpcServer             *grpc.Server
	OnConnectionRegistered func() // called when a new agent stream is registered (e.g. to trigger heartbeat check)
}

func NewServer(
	cfg *config.Config,
	machineRepo *repository.MachineRepository,
) *Server {
	return &Server{
		config:      cfg,
		machineRepo: machineRepo,
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
	pb.RegisterAgentServiceServer(s.grpcServer, s)
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

// Connect handles the bidirectional stream opened by an agent.
// The first message must carry the machine_id. After registration in the
// ConnectionManager, Run() takes over: it waits for ping requests from the
// HeartbeatChecker, writes them to the stream, reads pongs back, and
// forwards the results.
func (s *Server) Connect(stream pb.AgentService_ConnectServer) error {
	// Read the first message to identify the machine.
	first, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("connect: failed to receive initial message: %w", err)
	}

	machineID := first.GetMachineId()
	if machineID == "" {
		return fmt.Errorf("connect: machine_id is required in the first message")
	}

	// Validate the machine exists and is not dead.
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

	// So the heartbeat checker picks up this machine, ensure it's monitored.
	if machine.Status == "pending" {
		if err := s.machineRepo.UpdateStatus(stream.Context(), mid, "registered"); err != nil {
			log.Printf("Connect: failed to set machine %s to registered: %v", machineID, err)
		} else {
			machine.Status = "registered"
		}
	}

	log.Printf("Connect: machine %s connected", machineID)

	conn := s.ConnMgr.Register(machineID, stream)
	if s.OnConnectionRegistered != nil {
		s.OnConnectionRegistered()
	}
	defer func() {
		s.ConnMgr.Unregister(machineID)
		log.Printf("Connect: machine %s disconnected", machineID)
	}()

	// Run blocks until the stream closes or an error occurs.
	conn.Run()
	return nil
}
