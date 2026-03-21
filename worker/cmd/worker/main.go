package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lute/worker/internal/handler"
	"github.com/lute/worker/internal/heartbeat"
	"github.com/lute/worker/internal/setup"
	"github.com/lute/worker/internal/setup/types"
	"github.com/lute/worker/internal/utils"

	pb "github.com/lute/proto"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

type Flags struct {
	serverAddr  string
	apiURL      string
	machineID   string
	claimCode   string
	queues      string
	concurrency int
	jobLogsDir  string
	version     bool
	setupMode   bool
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	flags := parseFlags()

	if flags.version {
		fmt.Printf("lute-worker %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if flags.setupMode {
		setup.Run(flags.apiURL, Version, BuildTime, flags.claimCode)
		return
	}

	runWorker(flags)
}

func parseFlags() *Flags {
	f := &Flags{}
	flag.StringVar(&f.serverAddr, "server", "localhost:50051", "gRPC server address")
	flag.StringVar(&f.apiURL, "api", "http://localhost:8080", "HTTP API base URL")
	flag.StringVar(&f.machineID, "machine-id", "", "Machine ID (skip REST registration if provided)")
	flag.StringVar(&f.claimCode, "claim-code", "", "Claim code from UI to link this machine to your account")
	flag.StringVar(&f.queues, "queues", "default", "Comma-separated list of queues to process")
	flag.IntVar(&f.concurrency, "concurrency", 10, "Maximum concurrent jobs")
	flag.StringVar(&f.jobLogsDir, "job-logs-dir", "lute-logs", "Directory for per-job log files (relative to cwd)")
	flag.BoolVar(&f.version, "version", false, "Print version and exit")
	flag.BoolVar(&f.setupMode, "setup", false, "Run interactive setup")
	flag.Parse()
	return f
}

func runWorker(flags *Flags) {
	slog.Info("Lute Worker starting", "version", Version, "build", BuildTime)

	machineID := flags.machineID
	serverAddr := flags.serverAddr

	if machineID == "" {
		var grpcAddr string
		machineID, grpcAddr = registerViaREST(flags.apiURL)
		if grpcAddr != "" {
			serverAddr = grpcAddr
		}
	}

	queueList := strings.Split(flags.queues, ",")
	for i := range queueList {
		queueList[i] = strings.TrimSpace(queueList[i])
	}

	handler.JobLogsDir = flags.jobLogsDir

	if flags.jobLogsDir != "" {
		if err := os.MkdirAll(flags.jobLogsDir, 0755); err != nil {
			slog.Error("Cannot create job logs dir", "dir", flags.jobLogsDir, "err", err)
			os.Exit(1)
		}
	}

	slog.Info("worker config", "machine_id", machineID, "server", serverAddr, "queues", queueList, "concurrency", flags.concurrency, "job_logs_dir", flags.jobLogsDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		slog.Info("Shutting down worker...")
		cancel()
	}()

	connectLoop(ctx, serverAddr, machineID, queueList, int32(flags.concurrency), flags.jobLogsDir)
	slog.Info("Worker stopped")
}

func registerViaREST(apiURL string) (string, string) {
	hostname := utils.MustHostname()
	localIP := utils.GetLocalIP()

	body := types.SetupRequest{
		Name:     fmt.Sprintf("%s:%s", hostname, localIP),
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
		IP:       localIP,
		Version:  Version,
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"build_time": BuildTime,
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		slog.Error("Failed to marshal registration request", "err", err)
		os.Exit(1)
	}

	url := strings.TrimRight(apiURL, "/") + "/api/v1/worker/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Error("REST registration failed", "err", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read registration response", "err", err)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		slog.Error("Registration error", "status", resp.StatusCode, "body", string(respBody))
		os.Exit(1)
	}

	var result types.SetupResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		slog.Error("Failed to parse registration response", "err", err)
		os.Exit(1)
	}

	slog.Info("Registered", "machine_id", result.MachineID, "grpc", result.GRPCAddress)
	return result.MachineID, result.GRPCAddress
}

func connectLoop(ctx context.Context, serverAddr, machineID string, queues []string, concurrency int32, jobLogsDir string) {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		err := runStream(ctx, serverAddr, machineID, queues, concurrency, jobLogsDir)
		if ctx.Err() != nil {
			return
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

func runStream(ctx context.Context, serverAddr, machineID string, queues []string, concurrency int32, jobLogsDir string) error {
	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client := pb.NewWorkerServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	// Send initial message with machine_id.
	if err := stream.Send(&pb.WorkerMessage{MachineId: machineID}); err != nil {
		return fmt.Errorf("send initial: %w", err)
	}

	// Send worker registration with queues and concurrency.
	if err := stream.Send(&pb.WorkerMessage{
		MachineId: machineID,
		Payload: &pb.WorkerMessage_Register{
			Register: &pb.WorkerRegistration{
				WorkerId:    machineID,
				Queues:      queues,
				Concurrency: concurrency,
			},
		},
	}); err != nil {
		return fmt.Errorf("send registration: %w", err)
	}

	slog.Info("Connected", "server", serverAddr)

	var sendMu sync.Mutex
	draining := false

	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		if ping := msg.GetHeartbeatPing(); ping != nil {
			slog.Debug("Heartbeat ping received")
			pongMsg := heartbeat.PongMessage(machineID)
			sendMu.Lock()
			err := stream.Send(pongMsg)
			sendMu.Unlock()
			if err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		}

		if assign := msg.GetAssign(); assign != nil {
			if draining {
				sendMu.Lock()
				_ = stream.Send(&pb.WorkerMessage{
					MachineId: machineID,
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

			go func(a *pb.JobAssignment) {
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
					MachineId: machineID,
					Payload:   &pb.WorkerMessage_Result{Result: result},
				})
				sendMu.Unlock()
				if sendErr != nil {
					slog.Error("Failed to send job result", "job_id", a.JobId, "err", sendErr)
				}
			}(assign)
		}

		if msg.GetDrain() != nil {
			slog.Info("Drain signal received, finishing in-flight jobs")
			draining = true
		}
	}
}
