package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lute/server/api/config"
	"github.com/lute/server/api/database"
	"github.com/lute/server/api/grpc"
	"github.com/lute/server/api/middleware"
	"github.com/lute/server/api/repository"
	"github.com/lute/server/api/router"
	"github.com/lute/server/api/websocket"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Firebase
	if cfg.Firebase.ProjectID != "" {
		if err := middleware.InitFirebase(cfg.Firebase.ProjectID); err != nil {
			log.Fatalf("Failed to initialize Firebase: %v", err)
		}
		log.Println("Firebase initialized successfully")
	} else {
		log.Println("Warning: FIREBASE_PROJECT_ID not set, Firebase authentication will not work")
	}

	// Initialize MongoDB
	db, err := database.NewMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Close(ctx); err != nil {
			log.Printf("Error closing MongoDB connection: %v", err)
		}
	}()

	// Initialize repositories
	machineRepo := repository.NewMachineRepository(db.Database)
	userRepo := repository.NewUserRepository(db.Database)
	agentRepo := repository.NewAgentRepository(db.Database)
	commandRepo := repository.NewCommandRepository(db.Database)

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize gRPC server
	grpcServer := grpc.NewServer(cfg, machineRepo, agentRepo, commandRepo)
	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Setup HTTP router
	r := router.SetupRouter(cfg, db, machineRepo, userRepo, agentRepo, commandRepo, hub)

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("HTTP server starting on %s:%s", cfg.Server.Host, cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop gRPC server
	grpcServer.Stop()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
