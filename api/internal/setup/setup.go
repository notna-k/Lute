package setup

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/queue"
)

// Dependencies holds all initialized dependencies
type Dependencies struct {
	Config             *config.Config
	Database           *connection.MongoDB
	Redis              *redis.Client
	QueueEngine        *queue.Engine
	QueueScheduler     *queue.Scheduler
	StatsAggregator    *queue.StatsAggregator
	WorkerRepo         *repos.WorkerRepository
	UserRepo           *repos.UserRepository
	CommandRepo        *repos.CommandRepository
	UptimeSnapshotRepo *repos.UptimeSnapshotRepository
	WorkerSnapshotRepo *repos.WorkerSnapshotRepository
	JobExecutionRepo   *repos.JobExecutionRepository
	APIKeyRepo         *repos.APIKeyRepository
	RunRepo            *repos.RunRepository
	WebhookRepo        *repos.WebhookDeliveryRepository
}

// Initialize loads configuration and initializes all dependencies
func Initialize() (*Dependencies, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if err := initializeFirebase(cfg); err != nil {
		return nil, err
	}

	db, err := initializeDatabase(cfg)
	if err != nil {
		return nil, err
	}

	rdb, err := connection.NewRedisClient(cfg)
	if err != nil {
		return nil, err
	}

	queueEngine := queue.NewEngine(rdb)
	queueScheduler := queue.NewScheduler(queueEngine, time.Second)
	statsAgg := queue.NewStatsAggregator(rdb)

	reposInit := initializeRepositories(db)

	return &Dependencies{
		Config:             cfg,
		Database:           db,
		Redis:              rdb,
		QueueEngine:        queueEngine,
		QueueScheduler:     queueScheduler,
		StatsAggregator:    statsAgg,
		WorkerRepo:         reposInit.WorkerRepo,
		UserRepo:           reposInit.UserRepo,
		CommandRepo:        reposInit.CommandRepo,
		UptimeSnapshotRepo: reposInit.UptimeSnapshotRepo,
		WorkerSnapshotRepo: reposInit.WorkerSnapshotRepo,
		JobExecutionRepo:   reposInit.JobExecutionRepo,
		APIKeyRepo:         reposInit.APIKeyRepo,
		RunRepo:            reposInit.RunRepo,
		WebhookRepo:        reposInit.WebhookRepo,
	}, nil
}

// Close gracefully closes all dependencies
func (d *Dependencies) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Database.Close(ctx); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
	}
	if d.Redis != nil {
		if err := d.Redis.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func initializeFirebase(cfg *config.Config) error {
	if cfg.Firebase.ProjectID == "" {
		log.Println("Warning: FIREBASE_PROJECT_ID not set, Firebase authentication will not work")
		return nil
	}

	if err := middleware.InitFirebase(cfg.Firebase.ProjectID); err != nil {
		return err
	}

	log.Println("Firebase initialized successfully")
	return nil
}

func initializeDatabase(cfg *config.Config) (*connection.MongoDB, error) {
	db, err := connection.NewMongoDB(cfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Repositories holds all repository instances
type Repositories struct {
	WorkerRepo         *repos.WorkerRepository
	UserRepo           *repos.UserRepository
	CommandRepo        *repos.CommandRepository
	UptimeSnapshotRepo *repos.UptimeSnapshotRepository
	WorkerSnapshotRepo *repos.WorkerSnapshotRepository
	JobExecutionRepo   *repos.JobExecutionRepository
	APIKeyRepo         *repos.APIKeyRepository
	RunRepo            *repos.RunRepository
	WebhookRepo        *repos.WebhookDeliveryRepository
}

func initializeRepositories(db *connection.MongoDB) *Repositories {
	return &Repositories{
		WorkerRepo:         repos.NewWorkerRepository(db.Database),
		UserRepo:           repos.NewUserRepository(db.Database),
		CommandRepo:        repos.NewCommandRepository(db.Database),
		UptimeSnapshotRepo: repos.NewUptimeSnapshotRepository(db.Database),
		WorkerSnapshotRepo: repos.NewWorkerSnapshotRepository(db.Database),
		JobExecutionRepo:   repos.NewJobExecutionRepository(db.Database),
		APIKeyRepo:         repos.NewAPIKeyRepository(db.Database),
		RunRepo:            repos.NewRunRepository(db.Database),
		WebhookRepo:        repos.NewWebhookDeliveryRepository(db.Database),
	}
}
