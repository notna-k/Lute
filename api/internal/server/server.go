package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/jobdefs"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/router"
	"github.com/lute/api/internal/webhooks"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"
)

type Server struct {
	HTTP              *http.Server
	httpListener      net.Listener
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
	JobDefSyncer       *jobdefs.Syncer
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
		JobDefSyncer:       d.JobDefSyncer,
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
		d.QueueScheduler.SetOnLeasesExpired(grpcServer.HandleExpiredLeases)
	}

	workerSnapshotJob := worker.NewWorkerSnapshotJob(d.WorkerRepo, d.WorkerSnapshotRepo, d.Config.Metrics.SnapshotInterval)

	return &Server{
		HTTP:              httpServer,
		GRPC:              grpcServer,
		Hub:               hub,
		HeartbeatChecker:  heartbeatChecker,
		WorkerSnapshotJob: workerSnapshotJob,
		QueueScheduler:    d.QueueScheduler,
		WebhookDispatcher: webhooks.NewDispatcher(d.WebhookRepo, d.Config.Webhooks.PollInterval),
	}
}

// HTTPAddr is the address the HTTP server is listening on, or "" before Start.
func (s *Server) HTTPAddr() string {
	if s.httpListener == nil {
		return ""
	}
	return s.httpListener.Addr().String()
}

// GRPCAddr is the address the gRPC server is listening on, or "" before Start.
func (s *Server) GRPCAddr() string {
	return s.GRPC.Addr()
}

// Start binds both listeners, then serves and starts the background jobs. A port
// conflict is returned to the caller: binding in the foreground is what makes that
// possible, and what keeps a failure from killing the process from a goroutine.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.HTTP.Addr, err)
	}
	s.httpListener = lis

	if err := s.GRPC.Listen(); err != nil {
		_ = lis.Close()
		s.httpListener = nil
		return err
	}

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
		if err := s.GRPC.Serve(); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	go func() {
		log.Printf("HTTP server listening on %s", s.HTTPAddr())
		if err := s.HTTP.Serve(s.httpListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped: %v", err)
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

	s.GRPC.StopContext(ctx)

	if err := s.HTTP.Shutdown(ctx); err != nil {
		return err
	}

	log.Println("Server exited")
	return nil
}
