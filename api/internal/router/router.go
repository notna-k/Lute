package router

import (
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/dashboard"
	"github.com/lute/api/internal/health"
	"github.com/lute/api/internal/jobs"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"

	"github.com/gin-gonic/gin"
)

// SetupRouter builds the gin.Engine with global middleware and all domain routes.
func SetupRouter(
	cfg *config.Config,
	db *connection.MongoDB,
	workerRepo *repos.WorkerRepository,
	userRepo *repos.UserRepository,
	commandRepo *repos.CommandRepository,
	uptimeSnapshotRepo *repos.UptimeSnapshotRepository,
	workerSnapshotRepo *repos.WorkerSnapshotRepository,
	hub *websocket.Hub,
	queueEngine *queue.Engine,
	statsAgg *queue.StatsAggregator,
	grpcServer *luteGrpc.Server,
	jobExecutionRepo *repos.JobExecutionRepository,
) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	healthHandler := health.NewHealthHandler(db)
	api := r.Group("/api")
	{
		health.SetupRoutes(api, healthHandler)
	}

	wsHandler := websocket.NewWebSocketHandler(hub, cfg)
	api.GET("/ws", middleware.OptionalAuthMiddleware(), wsHandler.HandleWebSocket)

	workerService := worker.NewWorkerService(workerRepo)
	workerHandler := worker.NewWorkerHandler(cfg.WorkerBinary.Dir, cfg, workerRepo, commandRepo, grpcServer.ConnMgr)
	dashboardHandler := dashboard.NewDashboardHandler(cfg, workerService, workerSnapshotRepo)

	jobHandler := jobs.NewJobHandler(queueEngine, statsAgg, grpcServer, jobExecutionRepo)
	executionsHandler := jobs.NewExecutionsHandler(jobExecutionRepo)
	queueHandler := jobs.NewQueueHandler(queueEngine, statsAgg)
	dlqHandler := jobs.NewDLQHandler(queueEngine, grpcServer)

	v1 := api.Group("/v1")
	{
		worker.SetupRoutes(v1, workerHandler, userRepo)
		dashboard.SetupRoutes(v1, dashboardHandler, userRepo)
		jobs.SetupRoutes(v1, jobHandler, queueHandler, dlqHandler, executionsHandler)
	}

	return r
}
