// Package setup builds the repositories and services core runs on.
package setup

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/lute/api/internal/auth"
	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/connection"
	"github.com/lute/api/internal/db/models"
	"github.com/lute/api/internal/db/repos"
	"github.com/lute/api/internal/jobdefs"
	"github.com/lute/api/internal/queue"
)

type Deps struct {
	Config   *config.Config
	Database *connection.Database

	Queue *queue.Engine
	Stats *queue.Stats

	Workers         *repos.WorkerRepository
	WorkerSnapshots *repos.WorkerSnapshotRepository
	Commands        *repos.CommandRepository
	Users           *repos.UserRepository
	APIKeys         *repos.APIKeyRepository
	JobExecutions   *repos.JobExecutionRepository
	Runs            *repos.RunRepository
	Webhooks        *repos.WebhookDeliveryRepository
	JobDefs         *repos.JobDefinitionRepository
	Settings        *repos.SettingRepository

	JobDefSyncer *jobdefs.Syncer
	Tokens       *auth.TokenService
	Auth         *auth.Service
}

// New opens the database, seeds the admin and syncs job definitions; the e2e harness boots through it too.
func New(ctx context.Context, cfg *config.Config) (*Deps, error) {
	db, err := connection.Open(ctx, cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	d, err := build(ctx, cfg, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return d, nil
}

func build(ctx context.Context, cfg *config.Config, db *connection.Database) (*Deps, error) {
	g := db.DB
	tokens, err := auth.NewTokenService(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL, cfg.Auth.Issuer)
	if err != nil {
		return nil, err
	}
	d := &Deps{
		Config:   cfg,
		Database: db,
		Queue: queue.NewEngine(g, queue.Timings{
			LeaseGrace:   cfg.Queue.LeaseGrace,
			ReclaimAfter: cfg.Queue.ReclaimAfter,
		}),
		Stats:           queue.NewStats(g),
		Workers:         repos.NewWorkerRepository(g),
		WorkerSnapshots: repos.NewWorkerSnapshotRepository(g),
		Commands:        repos.NewCommandRepository(g),
		Users:           repos.NewUserRepository(g),
		APIKeys:         repos.NewAPIKeyRepository(g),
		JobExecutions:   repos.NewJobExecutionRepository(g),
		Runs:            repos.NewRunRepository(g),
		Webhooks:        repos.NewWebhookDeliveryRepository(g),
		JobDefs:         repos.NewJobDefinitionRepository(g),
		Settings:        repos.NewSettingRepository(g),
		Tokens:          tokens,
	}
	d.Auth = auth.NewService(d.Users, repos.NewRefreshTokenRepository(g), tokens)
	d.JobDefSyncer = jobdefs.NewSyncer(d.JobDefs, d.Settings, cfg.JobDefs.Dir)

	if err := seedAdminUser(ctx, cfg, d.Users); err != nil {
		return nil, err
	}
	if _, err := d.JobDefSyncer.Sync(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Deps) Close() {
	if err := d.Database.Close(); err != nil {
		slog.Error("close database", "err", err)
	}
}

// seedAdminUser creates the bootstrap admin from ADMIN_EMAIL / ADMIN_PASSWORD unless it exists.
func seedAdminUser(ctx context.Context, cfg *config.Config, users *repos.UserRepository) error {
	email := strings.ToLower(strings.TrimSpace(cfg.Auth.AdminEmail))
	password := cfg.Auth.AdminPassword
	if email == "" || password == "" {
		slog.Warn("ADMIN_EMAIL / ADMIN_PASSWORD not set, no admin user seeded")
		return nil
	}
	if _, err := users.GetByEmail(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, repos.ErrNotFound) {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := users.Create(ctx, &models.User{Email: email, DisplayName: "Admin", PasswordHash: hash}); err != nil {
		return err
	}
	slog.Info("seeded admin user", "email", email)
	return nil
}
