package connection

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	glebSQLite "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/migrate"
)

// Database wraps a shared GORM handle for SQLite or PostgreSQL.
type Database struct {
	Driver string
	DB     *gorm.DB
}

// SQL exposes the pooled *sql.DB for health checks and metrics integrations.
func (d *Database) SQL() (*sql.DB, error) {
	if d == nil || d.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return d.DB.DB()
}

// Close releases the connection pool.
func (d *Database) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// HealthCheck verifies the database is reachable.
func (d *Database) HealthCheck(ctx context.Context) error {
	sqlDB, err := d.SQL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

// Open connects to the configured driver, runs migrations, and returns a handle.
func Open(ctx context.Context, cfg *config.Config) (*Database, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "sqlite"
	}

	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		if strings.TrimSpace(cfg.Database.Postgres.DSN) == "" {
			return nil, fmt.Errorf("POSTGRES_DSN is required when DB_DRIVER=postgres")
		}
		dialector = postgres.Open(cfg.Database.Postgres.DSN)
	case "sqlite":
		busyMs := int(cfg.Database.SQLite.BusyTimeout / time.Millisecond)
		if busyMs < 1 {
			busyMs = 5000
		}
		path := filepath.ToSlash(filepath.Clean(cfg.Database.SQLite.Path))
		if strings.ContainsAny(path, "?&") {
			return nil, fmt.Errorf("sqlite path must not contain ? or &")
		}
		dsn := fmt.Sprintf(
			"file:%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
			path,
			busyMs,
		)
		dialector = glebSQLite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q (use sqlite or postgres)", driver)
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	if driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
	} else {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(5)
	}

	if err := migrate.Run(gormDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Database{Driver: driver, DB: gormDB}, nil
}
