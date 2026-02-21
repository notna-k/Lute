package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lute/api/server"
	"github.com/lute/api/setup"
)

func main() {
	// Initialize all dependencies
	deps, err := setup.Initialize()
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer deps.Close()

	// Create and start servers
	srv := server.New(
		deps.Config,
		deps.Database,
		deps.MachineRepo,
		deps.UserRepo,
		deps.CommandRepo,
		deps.UptimeSnapshotRepo,
		deps.MachineSnapshotRepo,
	)

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Wait for interrupt signal
	waitForShutdown()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}

// waitForShutdown waits for interrupt signal
func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
