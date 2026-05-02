package connection

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite driver (pure Go, CGO-free)

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/db/schema"
)

// SQLite wraps the application sql.DB handle.
type SQLite struct {
	DB *sql.DB
}

// NewSQLite opens SQLite, applies schema, and returns a handle.
func NewSQLite(ctx context.Context, cfg *config.Config) (*SQLite, error) {
	busyMs := int(cfg.SQLite.BusyTimeout / time.Millisecond)
	if busyMs < 1 {
		busyMs = 5000
	}
	path := filepath.ToSlash(filepath.Clean(cfg.SQLite.Path))
	if strings.ContainsAny(path, "?&") {
		return nil, fmt.Errorf("sqlite path must not contain ? or &")
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)",
		path,
		busyMs,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if err := schema.Apply(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &SQLite{DB: db}, nil
}

func (s *SQLite) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *SQLite) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.DB.PingContext(ctx)
}
