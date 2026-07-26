package server

import (
	"context"
	"log"
	"net/http"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/router"
	"github.com/lute/api/internal/webhooks"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"
)

type Server struct {
	HTTP              *http.Server
	GRPC              *grpc.Server
	Hub               *websocket.Hub
	HeartbeatChecker  *worker.HeartbeatChecker
	WorkerSnapshotJob *worker.WorkerSnapshotJob
	QueueScheduler    *queue.Scheduler
	WebhookDispatcher *webhooks.Dispatcher
	checkerCtx        context.Context
	checkerStop       context.CancelFunc
	snapshotJobCtx    context.Context
	snapshotJobCancel context.CancelFunc
	schedulerCtx      context.Context
	schedulerCancel   context.CancelFunc
	webhookCtx        context.Context
	webhookCancel     context.CancelFunc
}

// Deps aggregates the dependencies needed to construct a Server.
type Deps struct {
	Config             *config.Config
	Database           *connection.Database
	WorkerRepo         *repos.WorkerRepository
	UserRepo           *repos.UserRepository
	CommandRepo        *repos.CommandRepository
	UptimeSnapshotRepo *repos.UptimeSnapshotRepository
	WorkerSnapshotRepo *repos.WorkerSnapshotRepository
	JobExecutionRepo   *repos.JobExecutionRepository
	APIKeyRepo         *repos.APIKeyRepository
	RunRepo            *repos.RunRepository
	WebhookRepo        *repos.WebhookDeliveryRepository
	JobDefRepo         *repos.JobDefinitionRepository
	SettingRepo        *repos.SettingRepository
	QueueEngine        *queue.Engine
	QueueScheduler     *queue.Scheduler
	StatsAgg           *queue.StatsAggregator
	TokenService       *auth.TokenService
	AuthService        *auth.Service
}

func New(d Deps) *Server {
	hub := websocket.NewHub()
	go hub.Run()

	grpcServer := grpc.NewServer(d.Config, d.WorkerRepo, d.JobExecutionRepo, d.QueueEngine, d.StatsAgg, hub)

	emitter := webhooks.NewEmitter(d.RunRepo, d.WebhookRepo)
	grpcServer.WebhookEmitter = emitter

	r := router.SetupRouter(router.SetupRouterDeps{
		Config:             d.Config,
		DB:                 d.Database,
		WorkerRepo:         d.WorkerRepo,
		UserRepo:           d.UserRepo,
		CommandRepo:        d.CommandRepo,
		UptimeSnapshotRepo: d.UptimeSnapshotRepo,
		WorkerSnapshotRepo: d.WorkerSnapshotRepo,
		JobExecutionRepo:   d.JobExecutionRepo,
		APIKeyRepo:         d.APIKeyRepo,
		RunRepo:            d.RunRepo,
		JobDefRepo:         d.JobDefRepo,
		SettingRepo:        d.SettingRepo,
		Hub:                hub,
		QueueEngine:        d.QueueEngine,
		StatsAgg:           d.StatsAgg,
		GRPCServer:         grpcServer,
		TokenService:       d.TokenService,
		AuthService:        d.AuthService,
	})

	httpServer := &http.Server{
		Addr:         d.Config.Server.Host + ":" + d.Config.Server.Port,
		Handler:      r,
		ReadTimeout:  d.Config.Server.ReadTimeout,
		WriteTimeout: d.Config.Server.WriteTimeout,
		IdleTimeout:  d.Config.Server.IdleTimeout,
	}

	heartbeatChecker := worker.NewHeartbeatChecker(
		d.WorkerRepo,
		grpcServer.ConnMgr,
		d.Config.Heartbeat.CheckInterval,
		d.Config.Heartbeat.PingTimeout,
		d.Config.Heartbeat.MaxRetries,
	)
	grpcServer.OnConnectionRegistered = func() { heartbeatChecker.TriggerCheck() }

	if d.QueueScheduler != nil {
		d.QueueScheduler.SetOnJobsPromoted(func(ctx context.Context, queueNames []string) {
			for _, q := range queueNames {
				grpcServer.DispatchQueue(ctx, q)
			}
		})
	}

	workerSnapshotJob := worker.NewWorkerSnapshotJob(d.WorkerRepo, d.WorkerSnapshotRepo, d.Config.Metrics.SnapshotInterval)

	return &Server{
		HTTP:              httpServer,
		GRPC:              grpcServer,
		Hub:               hub,
		HeartbeatChecker:  heartbeatChecker,
		WorkerSnapshotJob: workerSnapshotJob,
		QueueScheduler:    d.QueueScheduler,
		WebhookDispatcher: webhooks.NewDispatcher(d.WebhookRepo),
	}
}

func (s *Server) Start() error {
	s.checkerCtx, s.checkerStop = context.WithCancel(context.Background())
	go s.HeartbeatChecker.Start(s.checkerCtx)

	s.snapshotJobCtx, s.snapshotJobCancel = context.WithCancel(context.Background())
	go s.WorkerSnapshotJob.Run(s.snapshotJobCtx)

	if s.QueueScheduler != nil {
		s.schedulerCtx, s.schedulerCancel = context.WithCancel(context.Background())
		go s.QueueScheduler.Run(s.schedulerCtx)
	}

	if s.WebhookDispatcher != nil {
		s.webhookCtx, s.webhookCancel = context.WithCancel(context.Background())
		go s.WebhookDispatcher.Run(s.webhookCtx)
	}

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
	if s.schedulerCancel != nil {
		s.schedulerCancel()
	}
	if s.webhookCancel != nil {
		s.webhookCancel()
	}

	s.GRPC.Stop()

	if err := s.HTTP.Shutdown(ctx); err != nil {
		return err
	}

	log.Println("Server exited")
	return nil
}
