package router

import (
	luteGrpc "github.com/lute/api/grpc"
	"github.com/lute/api/config"
	"github.com/lute/api/database"
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
	"github.com/lute/api/queue"
	"github.com/lute/api/repository"
	"github.com/lute/api/services"
	"github.com/lute/api/websocket"

	"github.com/gin-gonic/gin"
)

func SetupRouter(
	cfg *config.Config,
	db *database.MongoDB,
	machineRepo *repository.MachineRepository,
	userRepo *repository.UserRepository,
	commandRepo *repository.CommandRepository,
	uptimeSnapshotRepo *repository.UptimeSnapshotRepository,
	machineSnapshotRepo *repository.MachineSnapshotRepository,
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

	healthHandler := handlers.NewHealthHandler(db)
	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.HealthCheck)
		api.GET("/ready", healthHandler.Readiness)
	}

	wsHandler := handlers.NewWebSocketHandler(hub, cfg)
	api.GET("/ws", middleware.OptionalAuthMiddleware(), wsHandler.HandleWebSocket)

	machineService := services.NewMachineService(machineRepo)

	machineHandler := handlers.NewMachineHandler(machineService)
	workerHandler := handlers.NewWorkerHandler(cfg.WorkerBinary.Dir, cfg, machineRepo, commandRepo)
	dashboardHandler := handlers.NewDashboardHandler(cfg, machineService, machineSnapshotRepo)

	jobHandler := handlers.NewJobHandler(queueEngine, statsAgg, grpcServer)
	queueHandler := handlers.NewQueueHandler(queueEngine, statsAgg)
	dlqHandler := handlers.NewDLQHandler(queueEngine)
	workersInfoHandler := handlers.NewWorkersInfoHandler(grpcServer.ConnMgr)

	v1 := api.Group("/v1")
	{
		SetupMachineRoutes(v1, machineHandler, userRepo)
		SetupDashboardRoutes(v1, dashboardHandler, userRepo)
		SetupWorkerRoutes(v1, workerHandler, userRepo)
		SetupJobRoutes(v1, jobHandler, queueHandler, dlqHandler, workersInfoHandler)
	}

	return r
}
