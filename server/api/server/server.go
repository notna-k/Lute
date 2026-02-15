package server

import (
	"context"
	"log"
	"net/http"

	"github.com/lute/api/config"
	"github.com/lute/api/database"
	"github.com/lute/api/grpc"
	"github.com/lute/api/repository"
	"github.com/lute/api/router"
	"github.com/lute/api/websocket"
)

// Server holds HTTP and gRPC server instances
type Server struct {
	HTTP *http.Server
	GRPC *grpc.Server
	Hub  *websocket.Hub
}

// New creates and configures HTTP and gRPC servers
func New(
	cfg *config.Config,
	db *database.MongoDB,
	machineRepo *repository.MachineRepository,
	userRepo *repository.UserRepository,
	agentRepo *repository.AgentRepository,
	commandRepo *repository.CommandRepository,
) *Server {
	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize gRPC server
	grpcServer := grpc.NewServer(cfg, machineRepo, agentRepo, commandRepo)

	// Setup HTTP router
	r := router.SetupRouter(cfg, db, machineRepo, userRepo, agentRepo, commandRepo, hub)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Server{
		HTTP: httpServer,
		GRPC: grpcServer,
		Hub:  hub,
	}
}

// Start starts both HTTP and gRPC servers
func (s *Server) Start() error {
	// Start gRPC server
	go func() {
		if err := s.GRPC.Start(); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Start HTTP server
	go func() {
		log.Printf("HTTP server starting on %s", s.HTTP.Addr)
		if err := s.HTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down both servers
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	// Stop gRPC server
	s.GRPC.Stop()

	// Shutdown HTTP server
	if err := s.HTTP.Shutdown(ctx); err != nil {
		return err
	}

	log.Println("Server exited")
	return nil
}
