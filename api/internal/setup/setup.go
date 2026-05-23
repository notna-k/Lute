package setup

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/queue"
)

// Dependencies holds all initialized dependencies
type Dependencies struct {
	Config             *config.Config
	Database           *connection.Database
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
	RefreshTokenRepo   *repos.RefreshTokenRepository
	TokenService       *auth.TokenService
	AuthService        *auth.Service
}

// Initialize loads configuration and initializes all dependencies
func Initialize() (*Dependencies, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	db, err := initializeDatabase(cfg)
	if err != nil {
		return nil, err
	}
	jobQ := repos.NewJobQueueRepository(db.DB)
	queueEngine := queue.NewEngine(jobQ)
	queueScheduler := queue.NewScheduler(queueEngine, time.Second)
	statsAgg := queue.NewStatsAggregator(repos.NewQueueStatsRepository(db.DB))

	reposInit := initializeRepositories(db)

	tokenSvc, err := auth.NewTokenService(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, cfg.Auth.Issuer)
	if err != nil {
		return nil, err
	}
	authSvc := auth.NewService(reposInit.UserRepo, reposInit.RefreshTokenRepo, tokenSvc)

	if err := seedAdminUser(context.Background(), cfg, reposInit.UserRepo); err != nil {
		return nil, err
	}

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
		RefreshTokenRepo:   reposInit.RefreshTokenRepo,
		TokenService:       tokenSvc,
		AuthService:        authSvc,
	}, nil
}

// Close gracefully closes all dependencies
func (d *Dependencies) Close() {
	if d.Database != nil {
		if err := d.Database.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
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

func initializeDatabase(cfg *config.Config) (*connection.Database, error) {
	return connection.Open(context.Background(), cfg)
}

// seedAdminUser creates the bootstrap admin if ADMIN_EMAIL / ADMIN_PASSWORD are set
// and no user with that email exists yet. Password is bcrypt-hashed before insert.
func seedAdminUser(ctx context.Context, cfg *config.Config, users *repos.UserRepository) error {
	email := strings.ToLower(strings.TrimSpace(cfg.Auth.AdminEmail))
	password := cfg.Auth.AdminPassword
	if email == "" || password == "" {
		log.Println("auth: ADMIN_EMAIL / ADMIN_PASSWORD not set — no admin user seeded")
		return nil
	}
	if _, err := users.GetByEmail(ctx, email); err == nil {
		log.Printf("auth: admin user %s already exists, skipping seed", email)
		return nil
	} else if !errors.Is(err, repos.ErrNotFound) {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	u := &models.User{
		Email:        email,
		DisplayName:  "Admin",
		PasswordHash: hash,
	}
	if err := users.Create(ctx, u); err != nil {
		return err
	}
	log.Printf("auth: seeded admin user %s", email)
	return nil
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
	RefreshTokenRepo   *repos.RefreshTokenRepository
}

func initializeRepositories(db *connection.Database) *Repositories {
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
		RefreshTokenRepo:   repos.NewRefreshTokenRepository(db.DB),
	}
}
