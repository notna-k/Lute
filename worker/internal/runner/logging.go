package runner

import (
	"bytes"
	"context"
	"log/slog"
	"os"
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

// openJobLog returns a JSON logger writing to path.
func openJobLog(path string) (jobLogger *slog.Logger, closeLog func(), err error) {
	f, err := os.Create(path) //nolint:gosec // path is built by joblog.Path from a validated id
	if err != nil {
		return nil, nil, err
	}
	closeLog = func() {
		_ = f.Sync()
		_ = f.Close()
	}
	return slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})), closeLog, nil
}

// lineLogWriter emits one job log record per line written to it.
type lineLogWriter struct {
	jobLogger *slog.Logger
	source    string
	buf       []byte
}

func (w *lineLogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		w.jobLogger.Info(line, slog.String("source", w.source))
	}
	return len(p), nil
}

func (w *lineLogWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	w.jobLogger.Info(string(w.buf), slog.String("source", w.source))
	w.buf = nil
}
