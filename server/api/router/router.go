package router

import (
	"github.com/lute/api/config"
	"github.com/lute/api/database"
	"github.com/lute/api/handlers"
	"github.com/lute/api/middleware"
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
) *gin.Engine {
	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()

	// Global middleware
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS())

	// Health check endpoints (no auth required)
	healthHandler := handlers.NewHealthHandler(db)
	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.HealthCheck)
		api.GET("/ready", healthHandler.Readiness)
	}

	// WebSocket endpoint
	wsHandler := handlers.NewWebSocketHandler(hub, cfg)
	api.GET("/ws", middleware.OptionalAuthMiddleware(), wsHandler.HandleWebSocket)

	// Initialize services
	machineService := services.NewMachineService(machineRepo)

	// Initialize handlers
	machineHandler := handlers.NewMachineHandler(machineService)
	agentHandler := handlers.NewAgentHandler(cfg.AgentBinary.Dir, cfg, machineRepo, commandRepo)
	dashboardHandler := handlers.NewDashboardHandler(machineService, machineSnapshotRepo)

	// Protected API routes
	v1 := api.Group("/v1")
	{
		// Machine routes (with dedicated router)
		SetupMachineRoutes(v1, machineHandler, userRepo)

		// Dashboard routes (stats, uptime)
		SetupDashboardRoutes(v1, dashboardHandler, userRepo)

		// Agent binary distribution routes
		SetupAgentRoutes(v1, agentHandler, userRepo)
	}

	return r
}
