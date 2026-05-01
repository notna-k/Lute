package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/lute/worker/internal/handler"
	"github.com/lute/worker/internal/heartbeat"
	"github.com/lute/worker/internal/joblog"
	"github.com/lute/worker/internal/setup"

	pb "github.com/lute/proto"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

const (
	defaultAPIURL      = "http://localhost:8080"
	defaultGRPCAddr    = "localhost:50051"
	defaultQueues      = "default"
	defaultConcurrency = 10
	defaultJobLogsDir = "lute-job-logs"
)

type runFlags struct {
	serverAddr  string
	workerID    string
	queues      string
	concurrency int
	jobLogsDir  string
}

type setupFlags struct {
	apiURL    string
	claimCode string
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

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
		// Backward compatibility: if first arg is a flag (-foo / --foo),
		// fall back to the legacy flat flag parser. This keeps existing
		// install scripts and background auto-starts working.
		if strings.HasPrefix(cmd, "-") {
			legacyMain(os.Args[1:])
			return
		}
		fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `lute-worker %s — Lute compute agent

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

// ---------- run ----------

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	f := &runFlags{}
	fs.StringVar(&f.serverAddr, "server", defaultGRPCAddr, "gRPC server address (host:port)")
	fs.StringVar(&f.workerID, "worker-id", "", "Worker ID (required; obtain via `lute-worker setup`)")
	fs.StringVar(&f.queues, "queues", defaultQueues, "Comma-separated list of queues to process")
	fs.IntVar(&f.concurrency, "concurrency", defaultConcurrency, "Maximum concurrent jobs")
	fs.StringVar(&f.jobLogsDir, "job-logs-dir", defaultJobLogsDir, "Directory for per-job log files")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: lute-worker run [flags]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if f.workerID == "" {
		fmt.Fprintln(os.Stderr, "error: --worker-id is required. Run `lute-worker setup --claim-code <CODE>` first.")
		os.Exit(2)
	}

	runWorker(f)
}

// ---------- setup ----------

func cmdSetup(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	f := &setupFlags{}
	fs.StringVar(&f.apiURL, "api", defaultAPIURL, "HTTP API origin (scheme+host; registration POSTs to /api/public/v1/workers/bootstrap/register)")
	fs.StringVar(&f.claimCode, "claim-code", "", "Claim code from the Add Worker dialog in the Lute UI (required)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: lute-worker setup --claim-code <CODE> [--api URL]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if f.claimCode == "" {
		fmt.Fprintln(os.Stderr, "error: --claim-code is required.")
		fmt.Fprintln(os.Stderr, "Open the Add Worker dialog in the Lute UI (while logged in) and copy the full command.")
		os.Exit(2)
	}

	setup.Run(f.apiURL, Version, BuildTime, f.claimCode)
}

// ---------- logs ----------

func cmdLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	follow := false
	n := 100
	fs.BoolVar(&follow, "follow", false, "Follow the daemon log as it grows (like tail -f)")
	fs.BoolVar(&follow, "f", false, "Alias for -follow")
	fs.IntVar(&n, "n", 100, "Number of lines to show from the end of the file")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: lute-worker logs [-n N] [-f]")
		fmt.Fprintf(fs.Output(), "Reads %s — the same file `lute-worker setup` attaches the background worker's stdout/stderr to.\n", setup.DaemonLogPath())
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	path := setup.DaemonLogPath()
	if err := showLog(path, n, follow); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func showLog(path string, lines int, follow bool) error {
	if lines < 0 {
		lines = 0
	}
	if lines > 0 {
		res := joblog.ReadTail(path, lines, 0)
		if res.Err != "" {
			return errors.New(res.Err)
		}
		for _, line := range res.Lines {
			fmt.Println(line)
		}
		if !follow {
			return nil
		}
		return followFrom(path, res.FileSize)
	}
	return followFrom(path, 0)
}

func followFrom(path string, startOffset int64) error {
	for {
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		if startOffset > 0 {
			if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
				_ = f.Close()
				return err
			}
		}
		if err := tailLoop(f); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
		startOffset = 0
	}
}

func tailLoop(f *os.File) error {
	buf := make([]byte, 4096)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)
	for {
		select {
		case <-sigs:
			return nil
		default:
		}
		n, err := f.Read(buf)
		if n > 0 {
			_, _ = os.Stdout.Write(buf[:n])
		}
		if err == io.EOF {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
	}
}

// ---------- legacy flat-flag entry point (backward compatibility) ----------

type legacyFlags struct {
	serverAddr  string
	apiURL      string
	workerID    string
	claimCode   string
	queues      string
	concurrency int
	jobLogsDir  string
	version     bool
	setupMode   bool
}

func legacyMain(args []string) {
	fs := flag.NewFlagSet("lute-worker", flag.ExitOnError)
	f := &legacyFlags{}
	fs.StringVar(&f.serverAddr, "server", defaultGRPCAddr, "gRPC server address")
	fs.StringVar(&f.apiURL, "api", defaultAPIURL, "HTTP API origin (scheme+host; setup uses /api/public/v1/workers/bootstrap/register)")
	fs.StringVar(&f.workerID, "worker-id", "", "Worker ID (skip REST registration if provided)")
	fs.StringVar(&f.claimCode, "claim-code", "", "Claim code from UI to link this worker to your account")
	fs.StringVar(&f.queues, "queues", defaultQueues, "Comma-separated list of queues to process")
	fs.IntVar(&f.concurrency, "concurrency", defaultConcurrency, "Maximum concurrent jobs")
	fs.StringVar(&f.jobLogsDir, "job-logs-dir", defaultJobLogsDir, "Directory for per-job log files")
	fs.BoolVar(&f.version, "version", false, "Print version and exit")
	fs.BoolVar(&f.setupMode, "setup", false, "Run interactive setup")
	_ = fs.Parse(args)

	if f.version {
		fmt.Printf("lute-worker %s (built %s)\n", Version, BuildTime)
		return
	}

	if f.setupMode {
		if f.claimCode == "" {
			fmt.Fprintln(os.Stderr, "error: --claim-code is required for setup.")
			os.Exit(2)
		}
		setup.Run(f.apiURL, Version, BuildTime, f.claimCode)
		return
	}

	if f.workerID == "" {
		fmt.Fprintln(os.Stderr, "error: --worker-id is required to run the agent.")
		fmt.Fprintln(os.Stderr, "Run `lute-worker setup --claim-code <CODE>` first (copy the command from the Add Worker dialog).")
		os.Exit(2)
	}

	runWorker(&runFlags{
		serverAddr:  f.serverAddr,
		workerID:    f.workerID,
		queues:      f.queues,
		concurrency: f.concurrency,
		jobLogsDir:  f.jobLogsDir,
	})
}

// ---------- worker main loop (shared) ----------

func runWorker(f *runFlags) {
	slog.Info("Lute Worker starting", "version", Version, "build", BuildTime)

	queueList := strings.Split(f.queues, ",")
	for i := range queueList {
		queueList[i] = strings.TrimSpace(queueList[i])
	}

	handler.JobLogsDir = f.jobLogsDir

	if f.jobLogsDir != "" {
		if err := os.MkdirAll(f.jobLogsDir, 0755); err != nil {
			slog.Error("Cannot create job logs directory", "dir", f.jobLogsDir, "err", err)
			os.Exit(1)
		}
	}

	slog.Info("worker config",
		"worker_id", f.workerID,
		"server", f.serverAddr,
		"queues", queueList,
		"concurrency", f.concurrency,
		"job_logs_dir", f.jobLogsDir,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		slog.Info("Shutting down worker...")
		cancel()
	}()

	connectLoop(ctx, f.serverAddr, f.workerID, queueList, int32(f.concurrency), f.jobLogsDir)
	slog.Info("Worker stopped")
}

// errShutdown is returned by runStream when the server has asked the worker to
// exit its process (via DrainSignal.shutdown). connectLoop treats it as a
// terminal, non-retryable condition.
var errShutdown = errors.New("server requested worker shutdown")

func connectLoop(ctx context.Context, serverAddr, workerID string, queues []string, concurrency int32, jobLogsDir string) {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		err := runStream(ctx, serverAddr, workerID, queues, concurrency, jobLogsDir)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errShutdown) {
			slog.Info("Worker shutdown acknowledged, exiting")
			return
		}
		if st, ok := status.FromError(err); ok {
			switch st.Code() {
			case codes.NotFound:
				slog.Info("Server reports this worker no longer exists; exiting", "msg", st.Message())
				return
			case codes.FailedPrecondition:
				slog.Info("Server rejected connection with a non-retryable condition; exiting", "msg", st.Message())
				return
			}
		}

		slog.Warn("Stream disconnected, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func runStream(ctx context.Context, serverAddr, workerID string, queues []string, concurrency int32, jobLogsDir string) error {
	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewWorkerServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	if err := stream.Send(&pb.WorkerMessage{WorkerId: workerID}); err != nil {
		return fmt.Errorf("send initial: %w", err)
	}

	if err := stream.Send(&pb.WorkerMessage{
		WorkerId: workerID,
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{
				Queues:      queues,
				Concurrency: concurrency,
			},
		},
	}); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	slog.Info("Connected", "server", serverAddr)

	var sendMu sync.Mutex
	var jobsWG sync.WaitGroup
	draining := false
	shutdownRequested := false

	for {
		msg, err := stream.Recv()
		if err != nil {
			if shutdownRequested {
				jobsWG.Wait()
				return errShutdown
			}
			return fmt.Errorf("recv: %w", err)
		}

		if ping := msg.GetHeartbeatPing(); ping != nil {
			slog.Debug("Heartbeat ping received")
			pongMsg := heartbeat.PongMessage(workerID)
			sendMu.Lock()
			err := stream.Send(pongMsg)
			sendMu.Unlock()
			if err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		}

		if logReq := msg.GetJobLogRequest(); logReq != nil {
			req := logReq
			go func() {
				resp := buildJobLogResponse(jobLogsDir, req)
				sendMu.Lock()
				sendErr := stream.Send(&pb.WorkerMessage{
					WorkerId: workerID,
					Payload:  &pb.WorkerMessage_JobLogResponse{JobLogResponse: resp},
				})
				sendMu.Unlock()
				if sendErr != nil {
					slog.Error("Failed to send job log response", "request_id", req.RequestId, "err", sendErr)
				}
			}()
		}

		if assign := msg.GetAssign(); assign != nil {
			if draining {
				sendMu.Lock()
				_ = stream.Send(&pb.WorkerMessage{
					WorkerId: workerID,
					Payload: &pb.WorkerMessage_Result{
						Result: &pb.JobResult{
							JobId:   assign.JobId,
							Success: false,
							Error:   "worker is draining",
						},
					},
				})
				sendMu.Unlock()
				continue
			}

			jobsWG.Add(1)
			go func(a *pb.JobAssignment) {
				defer jobsWG.Done()
				start := time.Now()
				jobErr := handler.Execute(ctx, a.JobId, a.Type, a.Payload, a.TimeoutSec)
				elapsed := time.Since(start).Milliseconds()

				result := &pb.JobResult{
					JobId:     a.JobId,
					Success:   jobErr == nil,
					ElapsedMs: elapsed,
				}
				if jobLogsDir != "" {
					result.LogFile = "job-" + a.JobId + ".log"
				}
				if jobErr != nil {
					result.Error = jobErr.Error()
					slog.Warn("Job failed", "job_id", a.JobId, "err", jobErr)
				} else {
					slog.Info("Job completed", "job_id", a.JobId, "elapsed_ms", elapsed)
				}

				sendMu.Lock()
				sendErr := stream.Send(&pb.WorkerMessage{
					WorkerId: workerID,
					Payload:  &pb.WorkerMessage_Result{Result: result},
				})
				sendMu.Unlock()
				if sendErr != nil {
					slog.Error("Failed to send job result", "job_id", a.JobId, "err", sendErr)
				}
			}(assign)
		}

		if drain := msg.GetDrain(); drain != nil {
			draining = true
			if drain.GetShutdown() {
				if !shutdownRequested {
					shutdownRequested = true
					slog.Info("Shutdown signal received, finishing in-flight jobs then exiting")
					go func() {
						jobsWG.Wait()
						_ = stream.CloseSend()
					}()
				}
			} else {
				slog.Info("Drain signal received, finishing in-flight jobs")
			}
		}
	}
}

func buildJobLogResponse(jobLogsDir string, req *pb.JobLogRequest) *pb.JobLogResponse {
	resp := &pb.JobLogResponse{
		RequestId: req.RequestId,
	}
	if jobLogsDir == "" {
		resp.Error = "job logs directory not configured on worker"
		return resp
	}
	if req.JobId == "" {
		resp.Error = "job_id is required"
		return resp
	}
	path := filepath.Join(jobLogsDir, "job-"+req.JobId+".log")
	limit := int(req.Limit)
	var r joblog.Result
	switch req.GetDirection() {
	case pb.LogReadDirection_LOG_READ_HEAD:
		r = joblog.ReadHead(path, limit, req.AnchorOffset)
	default:
		r = joblog.ReadTail(path, limit, req.AnchorOffset)
	}
	resp.Lines = r.Lines
	resp.NextAnchor = r.NextAnchor
	resp.FileSize = r.FileSize
	resp.HasMore = r.HasMore
	resp.Error = r.Err
	return resp
}
