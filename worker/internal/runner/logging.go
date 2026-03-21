package runner

import (
	"context"
	"fmt"
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

// logSystem writes the same record to the process default logger (e.g. stderr) and to jobLogger (job file).
func logSystem(jobLogger *slog.Logger, level slog.Level, msg string, args ...any) {
	a := append([]any{slog.String("source", LogSourceSystem)}, args...)
	slog.Log(context.Background(), level, msg, a...)
	jobLogger.Log(context.Background(), level, msg, a...)
}

// ValidateJobLogDir returns an error if dir is not an existing directory.
func ValidateJobLogDir(dir string) error {
	if dir == "" {
		return nil
	}
	st, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("job log directory does not exist: %s", dir)
		}
		return fmt.Errorf("job log directory: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("job log path is not a directory: %s", dir)
	}
	return nil
}

// openJobLog returns a JSON slog.Logger that always writes to a file when logDir is set
// (job-{jobID}.log). When logDir is empty, logs are discarded; close is a no-op.
// Defer close after open so the file is synced and closed.
func openJobLog(logDir, jobID string) (jobLogger *slog.Logger, close func(), err error) {
	opts := &slog.HandlerOptions{Level: LogLevel}
	if logDir == "" {
		return slog.New(slog.NewJSONHandler(io.Discard, opts)), func() {}, nil
	}
	if err := ValidateJobLogDir(logDir); err != nil {
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
