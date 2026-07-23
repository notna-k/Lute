package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lute/api/internal/server"
	"github.com/lute/api/internal/setup"
)

func main() {
	deps, err := setup.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer deps.Close()

	srv := server.New(server.Deps{
		Config:             deps.Config,
		Database:           deps.Database,
		WorkerRepo:         deps.WorkerRepo,
		UserRepo:           deps.UserRepo,
		CommandRepo:        deps.CommandRepo,
		UptimeSnapshotRepo: deps.UptimeSnapshotRepo,
		WorkerSnapshotRepo: deps.WorkerSnapshotRepo,
		JobExecutionRepo:   deps.JobExecutionRepo,
		APIKeyRepo:         deps.APIKeyRepo,
		RunRepo:            deps.RunRepo,
		WebhookRepo:        deps.WebhookRepo,
		JobDefRepo:         deps.JobDefRepo,
		QueueEngine:        deps.QueueEngine,
		QueueScheduler:     deps.QueueScheduler,
		StatsAgg:           deps.StatsAggregator,
		TokenService:       deps.TokenService,
		AuthService:        deps.AuthService,
	})

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	waitForShutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
