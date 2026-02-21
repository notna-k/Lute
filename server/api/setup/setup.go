package setup

import (
	"context"
	"log"
	"time"

	"github.com/lute/api/config"
	"github.com/lute/api/database"
	"github.com/lute/api/middleware"
	"github.com/lute/api/repository"
)

// Dependencies holds all initialized dependencies
type Dependencies struct {
	Config               *config.Config
	Database             *database.MongoDB
	MachineRepo          *repository.MachineRepository
	UserRepo             *repository.UserRepository
	CommandRepo          *repository.CommandRepository
	UptimeSnapshotRepo   *repository.UptimeSnapshotRepository
	MachineSnapshotRepo  *repository.MachineSnapshotRepository
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

	repos := initializeRepositories(db)

	return &Dependencies{
		Config:              cfg,
		Database:            db,
		MachineRepo:         repos.MachineRepo,
		UserRepo:            repos.UserRepo,
		CommandRepo:         repos.CommandRepo,
		UptimeSnapshotRepo:  repos.UptimeSnapshotRepo,
		MachineSnapshotRepo: repos.MachineSnapshotRepo,
	}, nil
}

// Close gracefully closes all dependencies
func (d *Dependencies) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Database.Close(ctx); err != nil {
		log.Printf("Error closing MongoDB connection: %v", err)
	}
}

// loadConfig loads application configuration
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// initializeFirebase initializes Firebase authentication
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

// initializeDatabase connects to MongoDB
func initializeDatabase(cfg *config.Config) (*database.MongoDB, error) {
	db, err := database.NewMongoDB(cfg)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Repositories holds all repository instances
type Repositories struct {
	MachineRepo        *repository.MachineRepository
	UserRepo           *repository.UserRepository
	CommandRepo        *repository.CommandRepository
	UptimeSnapshotRepo *repository.UptimeSnapshotRepository
	MachineSnapshotRepo *repository.MachineSnapshotRepository
}

// initializeRepositories creates all repository instances
func initializeRepositories(db *database.MongoDB) *Repositories {
	return &Repositories{
		MachineRepo:         repository.NewMachineRepository(db.Database),
		UserRepo:            repository.NewUserRepository(db.Database),
		CommandRepo:         repository.NewCommandRepository(db.Database),
		UptimeSnapshotRepo:  repository.NewUptimeSnapshotRepository(db.Database),
		MachineSnapshotRepo: repository.NewMachineSnapshotRepository(db.Database),
	}
}
