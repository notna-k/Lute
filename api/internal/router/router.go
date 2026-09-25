package router

import (
	"github.com/gin-gonic/gin"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/dashboard"
	luteGrpc "github.com/lute/api/internal/grpc"
	"github.com/lute/api/internal/health"
	"github.com/lute/api/internal/jobdefs"
	"github.com/lute/api/internal/jobs"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/publicapi"
	"github.com/lute/api/internal/settings"
	"github.com/lute/api/internal/setup"
	"github.com/lute/api/internal/ui"
	"github.com/lute/api/internal/websocket"
	"github.com/lute/api/internal/worker"
)

// New builds the HTTP handler: the panel API under /api/v1, the API-key API under
// /api/public/v1, and the embedded UI for everything else.
func New(d *setup.Deps, hub *websocket.Hub, grpcServer *luteGrpc.Server) *gin.Engine {
	cfg := d.Config
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(middleware.Logger(), middleware.Recovery(), middleware.CORS(cfg.Server.AllowedOrigins))

	api := r.Group("/api")
	health.SetupRoutes(api, health.NewHealthHandler(d.Database))
	// The WebSocket handler authenticates itself: a browser sends the token as a
	// subprotocol, which JWTAuthMiddleware would reject.
	api.GET("/ws", websocket.NewWebSocketHandler(hub, cfg, d.Tokens).HandleWebSocket)

	workerHandler := worker.NewWorkerHandler(cfg.WorkerBinary.Dir, cfg, d.Workers, d.Commands, grpcServer.ConnMgr, grpcServer)
	authedMW := middleware.JWTAuthMiddleware(d.Tokens)

	v1 := api.Group("/v1")
	auth.SetupRoutes(v1, auth.NewHandler(d.Auth, d.Users, auth.DefaultCookieConfig(cfg.Auth.CookieSecure)), authedMW)
	worker.MountJWT(v1, workerHandler, authedMW)
	dashboard.SetupRoutes(v1, dashboard.NewDashboardHandler(cfg, d.Workers, d.WorkerSnapshots), authedMW)

	authed := v1.Group("", authedMW)
	jobs.SetupRoutes(authed,
		jobs.NewJobHandler(d.Queue, d.Stats, grpcServer, d.JobExecutions),
		jobs.NewQueueHandler(d.Queue, d.Stats),
		jobs.NewDLQHandler(d.Queue, grpcServer),
		jobs.NewExecutionsHandler(d.JobExecutions),
	)
	jobdefs.SetupRoutes(authed, jobdefs.NewHandler(d.JobDefs, d.JobDefSyncer, d.Runs, d.JobExecutions, d.Settings, d.Queue, d.Stats, grpcServer))
	publicapi.SetupAPIKeyRoutes(authed, publicapi.NewAPIKeysHandler(d.APIKeys))
	settings.SetupRoutes(authed, settings.NewHandler(d.Settings))

	runsHandler := publicapi.NewRunsHandler(d.Queue, d.Stats, grpcServer, d.Runs, d.JobExecutions)
	publicapi.SetupPublicRoutes(api.Group("/public/v1"), d.APIKeys, runsHandler, workerHandler)

	ui.Register(r)
	return r
}
