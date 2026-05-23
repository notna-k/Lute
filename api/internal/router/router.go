package router

import (
	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/dashboard"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/health"
	"github.com/lute/api/internal/jobs"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/publicapi"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/ui"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"

	"github.com/gin-gonic/gin"
)

// SetupRouterDeps bundles router dependencies so the signature does not grow
// unbounded as new domains are added.
type SetupRouterDeps struct {
	Config             *config.Config
	DB                 *connection.Database
	WorkerRepo         *repos.WorkerRepository
	UserRepo           *repos.UserRepository
	CommandRepo        *repos.CommandRepository
	UptimeSnapshotRepo *repos.UptimeSnapshotRepository
	WorkerSnapshotRepo *repos.WorkerSnapshotRepository
	JobExecutionRepo   *repos.JobExecutionRepository
	APIKeyRepo         *repos.APIKeyRepository
	RunRepo            *repos.RunRepository
	Hub                *websocket.Hub
	QueueEngine        *queue.Engine
	StatsAgg           *queue.StatsAggregator
	GRPCServer         *luteGrpc.Server
	TokenService       *auth.TokenService
	AuthService        *auth.Service
}

// SetupRouter builds the gin.Engine with global middleware and all domain routes.
func SetupRouter(d SetupRouterDeps) *gin.Engine {
	gin.SetMode(d.Config.Server.Mode)

	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	healthHandler := health.NewHealthHandler(d.DB)
	api := r.Group("/api")
	{
		health.SetupRoutes(api, healthHandler)
	}

	wsHandler := websocket.NewWebSocketHandler(d.Hub, d.Config)
	api.GET("/ws", middleware.OptionalAuthMiddleware(), wsHandler.HandleWebSocket)

	workerService := worker.NewWorkerService(d.WorkerRepo)
	workerHandler := worker.NewWorkerHandler(d.Config.WorkerBinary.Dir, d.Config, d.WorkerRepo, d.CommandRepo, d.GRPCServer.ConnMgr)
	dashboardHandler := dashboard.NewDashboardHandler(d.Config, workerService, d.WorkerSnapshotRepo)
	apiKeysHandler := publicapi.NewAPIKeysHandler(d.APIKeyRepo)

	jobHandler := jobs.NewJobHandler(d.QueueEngine, d.StatsAgg, d.GRPCServer, d.JobExecutionRepo)
	executionsHandler := jobs.NewExecutionsHandler(d.JobExecutionRepo)
	queueHandler := jobs.NewQueueHandler(d.QueueEngine, d.StatsAgg)
	dlqHandler := jobs.NewDLQHandler(d.QueueEngine, d.GRPCServer)

	authedMW := middleware.JWTAuthMiddleware(d.TokenService)
	authHandler := auth.NewHandler(d.AuthService, d.UserRepo, auth.DefaultCookieConfig(d.Config.Auth.CookieSecure))

	v1 := api.Group("/v1")
	{
		auth.SetupRoutes(v1, authHandler, authedMW)
		worker.SetupRoutes(v1, workerHandler, authedMW)
		dashboard.SetupRoutes(v1, dashboardHandler, authedMW)

		authed := v1.Group("")
		authed.Use(authedMW)
		{
			jobs.SetupRoutes(authed, jobHandler, queueHandler, dlqHandler, executionsHandler)
			publicapi.SetupAPIKeyRoutes(authed, apiKeysHandler)
		}
	}

	runsHandler := publicapi.NewRunsHandler(d.QueueEngine, d.StatsAgg, d.GRPCServer, d.RunRepo, d.JobExecutionRepo)
	pub := api.Group("/public/v1")
	publicapi.SetupPublicRoutes(pub, d.APIKeyRepo, runsHandler, workerHandler)

	ui.Register(r)
	return r
}
