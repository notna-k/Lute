package runner

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/lute/worker/internal/joblog"
)

// Values of the "source" attribute on job log lines.
const (
	sourceSystem    = "system"
	sourceContainer = "container"
)

// logSystem writes a record to both the agent's own log and the job log.
func logSystem(jobLogger *slog.Logger, level slog.Level, msg string, args ...any) {
	a := append([]any{slog.String("source", sourceSystem)}, args...)
	slog.Log(context.Background(), level, msg, a...)
	jobLogger.Log(context.Background(), level, msg, a...)
}

// openJobLog returns a JSON logger writing to the job's log file in logDir, or discarding when logDir is empty.
func openJobLog(logDir, jobID string) (jobLogger *slog.Logger, closeLog func(), err error) {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	if logDir == "" {
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), func() {}, nil
	}
	path, err := joblog.Path(logDir, jobID)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	closeLog = func() {
		_ = f.Sync()
		_ = f.Close()
	}
	return slog.New(slog.NewJSONHandler(f, opts)), closeLog, nil
}
