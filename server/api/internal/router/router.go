package router

import (
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/dashboard"
	"github.com/lute/api/internal/health"
	"github.com/lute/api/internal/jobs"
	"github.com/lute/api/internal/machines"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/queue"
	"github.com/lute/api/internal/worker"
	"github.com/lute/api/internal/websocket"

	"github.com/gin-gonic/gin"
)

// SetupRouter builds the gin.Engine with global middleware and all domain routes.
func SetupRouter(
	cfg *config.Config,
	db *connection.MongoDB,
	machineRepo *repos.MachineRepository,
	userRepo *repos.UserRepository,
	commandRepo *repos.CommandRepository,
	uptimeSnapshotRepo *repos.UptimeSnapshotRepository,
	machineSnapshotRepo *repos.MachineSnapshotRepository,
	hub *websocket.Hub,
	queueEngine *queue.Engine,
	statsAgg *queue.StatsAggregator,
	grpcServer *luteGrpc.Server,
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

	machineService := machines.NewMachineService(machineRepo)
	machineHandler := machines.NewMachineHandler(machineService)
	workerHandler := worker.NewWorkerHandler(cfg.WorkerBinary.Dir, cfg, machineRepo, commandRepo)
	dashboardHandler := dashboard.NewDashboardHandler(cfg, machineService, machineSnapshotRepo)

	jobHandler := jobs.NewJobHandler(queueEngine, statsAgg, grpcServer)
	queueHandler := jobs.NewQueueHandler(queueEngine, statsAgg)
	dlqHandler := jobs.NewDLQHandler(queueEngine)
	workersInfoHandler := jobs.NewWorkersInfoHandler(grpcServer.ConnMgr)

	v1 := api.Group("/v1")
	{
		machines.SetupRoutes(v1, machineHandler, userRepo)
		dashboard.SetupRoutes(v1, dashboardHandler, userRepo)
		worker.SetupRoutes(v1, workerHandler, userRepo)
		jobs.SetupRoutes(v1, jobHandler, queueHandler, dlqHandler, workersInfoHandler)
	}

	return r
}
