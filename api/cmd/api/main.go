package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/server"
	"github.com/lute/api/internal/setup"
)

func main() {
	if err := run(); err != nil {
		slog.Error("core stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	deps, err := setup.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer deps.Close()

	srv := server.New(deps)
	if err := srv.Start(); err != nil {
		return err
	}
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
