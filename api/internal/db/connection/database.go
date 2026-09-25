package connection

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/lute/api/internal/db/migrate"
)

type Database struct {
	DB *gorm.DB
}

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

func (d *Database) HealthCheck(ctx context.Context) error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

// Open connects to Postgres and runs migrations.
func Open(ctx context.Context, dsn string) (*Database, error) {
	if dsn == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := migrate.Run(gormDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Database{DB: gormDB}, nil
}
