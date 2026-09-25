package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lute/worker/internal/agent"
	"github.com/lute/worker/internal/setup"
)

// Set at build time via -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "run":
		cmdRun(args)
	case "setup":
		cmdSetup(args)
	case "logs":
		cmdLogs(args)
	case "version", "--version", "-v":
		fmt.Printf("lute-worker %s (built %s)\n", Version, BuildTime)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `lute-worker %s — Lute compute agent

Usage:
  lute-worker <command> [flags]

Commands:
  run        Run the worker agent (connects to the gRPC server and processes jobs)
  setup      Register this host with the Lute server and start the agent
  logs       Show or follow the daemon log (stdout/stderr from setup background worker)
  version    Print version information
  help       Show this help

Examples:
  # First-time registration using a claim code copied from the UI
  lute-worker setup --claim-code ABCDEFGHJKLMNPQRSTUV

  # Start an already-registered agent
  lute-worker run --server api.lute.local:50051 --worker-id 6521...

  # Follow daemon log (same file setup redirects the agent to)
  lute-worker logs -f

  # Last 200 lines of the daemon log
  lute-worker logs -n 200

Use "lute-worker <command> -h" for flags specific to a command.
`, Version)
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	server := fs.String("server", "localhost:50051", "gRPC server address (host:port)")
	workerID := fs.String("worker-id", "", "Worker ID (required; obtain via `lute-worker setup`)")
	queues := fs.String("queues", "default", "Comma-separated list of queues to process")
	concurrency := fs.Int("concurrency", 10, "Maximum concurrent jobs")
	jobLogsDir := fs.String("job-logs-dir", "lute-job-logs", "Directory for per-job log files")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "Usage: lute-worker run [flags]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if *workerID == "" {
		fatal(2, "--worker-id is required. Run `lute-worker setup --claim-code <CODE>` first.")
	}
	if *concurrency < 1 || *concurrency > 1<<16 {
		fatal(2, "--concurrency must be between 1 and 65536")
	}
	if *jobLogsDir != "" {
		if err := os.MkdirAll(*jobLogsDir, 0o755); err != nil {
			fatal(1, fmt.Sprintf("cannot create job logs directory: %v", err))
		}
	}

	cfg := agent.Config{
		ServerAddr:  *server,
		WorkerID:    *workerID,
		Queues:      splitList(*queues),
		Concurrency: int32(*concurrency),
		JobLogsDir:  *jobLogsDir,
	}
	slog.Info("Lute Worker starting", "version", Version, "build", BuildTime)
	slog.Info("worker config", "worker_id", cfg.WorkerID, "server", cfg.ServerAddr, "queues", cfg.Queues,
		"concurrency", cfg.Concurrency, "job_logs_dir", cfg.JobLogsDir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	agent.Run(ctx, cfg)
	slog.Info("Worker stopped")
}

func cmdSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	apiURL := fs.String("api", "http://localhost:8080", "HTTP API origin (scheme+host)")
	claimCode := fs.String("claim-code", "", "Claim code from the Add Worker dialog in the Lute UI (required)")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "Usage: lute-worker setup --claim-code <CODE> [--api URL]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if *claimCode == "" {
		fatal(2, "--claim-code is required.\nOpen the Add Worker dialog in the Lute UI (while logged in) and copy the full command.")
	}
	err := setup.Run(setup.Options{APIURL: *apiURL, ClaimCode: *claimCode, Version: Version, BuildTime: BuildTime})
	if err != nil {
		fatal(1, err.Error())
	}
}

func cmdLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	var follow bool
	fs.BoolVar(&follow, "follow", false, "Follow the daemon log as it grows (like tail -f)")
	fs.BoolVar(&follow, "f", false, "Alias for -follow")
	lines := fs.Int("n", 100, "Number of lines to show from the end of the file")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "Usage: lute-worker logs [-n N] [-f]")
		_, _ = fmt.Fprintf(fs.Output(), "Reads %s, where `lute-worker setup` sends the background agent's output.\n", setup.DaemonLogPath())
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if err := showLog(setup.DaemonLogPath(), *lines, follow); err != nil {
		fatal(1, err.Error())
	}
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func fatal(code int, msg string) {
	_, _ = fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(code)
}
