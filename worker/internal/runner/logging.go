package runner

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Source values for JSON log lines (job log file).
const (
	LogSourceSystem    = "system"
	LogSourceContainer = "container"
	LogLevel           = slog.LevelDebug
)

// openJobLog returns a JSON slog.Logger that always writes to a file when logDir is set
// (job-{jobID}.log). When logDir is empty, logs are discarded; close is a no-op.
// Defer close after open so the file is synced and closed.
func openJobLog(logDir, jobID string) (jobLogger *slog.Logger, close func(), err error) {
	opts := &slog.HandlerOptions{Level: LogLevel}
	if logDir == "" {
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), func() {}, nil
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, err
	}
	f, err := os.Create(filepath.Join(logDir, "job-"+jobID+".log"))
	if err != nil {
		return nil, nil, err
	}
	jobLogger = slog.New(slog.NewJSONHandler(f, opts))
	close = func() {
		_ = f.Sync()
		_ = f.Close()
	}
	return jobLogger, close, nil
}
