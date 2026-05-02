package setup

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/middleware"
	"github.com/lute/api/internal/queue"
)

// Dependencies holds all initialized dependencies
type Dependencies struct {
	Config             *config.Config
	Database           *connection.SQLite
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

	queueEngine := queue.NewEngine(db.DB)
	queueScheduler := queue.NewScheduler(queueEngine, time.Second)
	statsAgg := queue.NewStatsAggregator(db.DB)

	reposInit := initializeRepositories(db)

	return &Dependencies{
		Config:             cfg,
		Database:           db,
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
	if d.Database != nil {
		if err := d.Database.Close(); err != nil {
			log.Printf("Error closing SQLite: %v", err)
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

func initializeDatabase(cfg *config.Config) (*connection.SQLite, error) {
	return connection.NewSQLite(context.Background(), cfg)
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

func initializeRepositories(db *connection.SQLite) *Repositories {
	return &Repositories{
		WorkerRepo:         repos.NewWorkerRepository(db.DB),
		UserRepo:           repos.NewUserRepository(db.DB),
		CommandRepo:        repos.NewCommandRepository(db.DB),
		UptimeSnapshotRepo: repos.NewUptimeSnapshotRepository(db.DB),
		WorkerSnapshotRepo: repos.NewWorkerSnapshotRepository(db.DB),
		JobExecutionRepo:   repos.NewJobExecutionRepository(db.DB),
		APIKeyRepo:         repos.NewAPIKeyRepository(db.DB),
		RunRepo:            repos.NewRunRepository(db.DB),
		WebhookRepo:        repos.NewWebhookDeliveryRepository(db.DB),
	}
}
