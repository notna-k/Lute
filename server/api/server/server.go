package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/lute/api/config"
	"github.com/lute/api/database"
	"github.com/lute/api/grpc"
	"github.com/lute/api/repository"
	"github.com/lute/api/router"
	"github.com/lute/api/services"
	"github.com/lute/api/websocket"
)

type Server struct {
	HTTP               *http.Server
	GRPC               *grpc.Server
	Hub                *websocket.Hub
	HeartbeatChecker   *services.HeartbeatChecker
	MachineSnapshotJob *services.MachineSnapshotJob
	checkerCtx         context.Context
	checkerStop        context.CancelFunc
	snapshotJobCtx     context.Context
	snapshotJobCancel  context.CancelFunc
}

func New(
	cfg *config.Config,
	db *database.MongoDB,
	machineRepo *repository.MachineRepository,
	userRepo *repository.UserRepository,
	commandRepo *repository.CommandRepository,
	uptimeSnapshotRepo *repository.UptimeSnapshotRepository,
	machineSnapshotRepo *repository.MachineSnapshotRepository,
) *Server {
	hub := websocket.NewHub()
	go hub.Run()

	grpcServer := grpc.NewServer(cfg, machineRepo)

	r := router.SetupRouter(cfg, db, machineRepo, userRepo, commandRepo, uptimeSnapshotRepo, machineSnapshotRepo, hub)

	httpServer := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	heartbeatChecker := services.NewHeartbeatChecker(
		machineRepo,
		grpcServer.ConnMgr,
		cfg.Heartbeat.CheckInterval,
		cfg.Heartbeat.PingTimeout,
		cfg.Heartbeat.MaxRetries,
	)
	grpcServer.OnConnectionRegistered = func() { heartbeatChecker.TriggerCheck() }

	machineSnapshotJob := services.NewMachineSnapshotJob(machineRepo, machineSnapshotRepo, 100*time.Millisecond)

	return &Server{
		HTTP:               httpServer,
		GRPC:               grpcServer,
		Hub:                hub,
		HeartbeatChecker:   heartbeatChecker,
		MachineSnapshotJob: machineSnapshotJob,
	}
}

func (s *Server) Start() error {
	s.checkerCtx, s.checkerStop = context.WithCancel(context.Background())
	go s.HeartbeatChecker.Start(s.checkerCtx)

	s.snapshotJobCtx, s.snapshotJobCancel = context.WithCancel(context.Background())
	go s.MachineSnapshotJob.Run(s.snapshotJobCtx)

	go func() {
		if err := s.GRPC.Start(); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	go func() {
		log.Printf("HTTP server starting on %s", s.HTTP.Addr)
		if err := s.HTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down server...")

	if s.checkerStop != nil {
		s.checkerStop()
	}
	if s.snapshotJobCancel != nil {
		s.snapshotJobCancel()
	}

	s.GRPC.Stop()

	if err := s.HTTP.Shutdown(ctx); err != nil {
		return err
	}

	log.Println("Server exited")
	return nil
}
