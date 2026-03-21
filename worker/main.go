package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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

	"github.com/lute/worker/handler"
	"github.com/lute/worker/heartbeat"
	"github.com/lute/worker/setup"
	"github.com/lute/worker/setup/types"
	"github.com/lute/worker/utils"

	pb "github.com/lute/worker/proto/worker"
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
	log.Printf("Lute Worker %s starting (build: %s)", Version, BuildTime)

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
			log.Fatalf("Cannot create job logs dir %q: %v", flags.jobLogsDir, err)
		}
	}

	log.Printf("  Machine ID:   %s", machineID)
	log.Printf("  Server:       %s", serverAddr)
	log.Printf("  Queues:       %v", queueList)
	log.Printf("  Concurrency:  %d", flags.concurrency)
	if flags.jobLogsDir != "" {
		log.Printf("  Job Logs Dir: %s", flags.jobLogsDir)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Shutting down worker...")
		cancel()
	}()

	connectLoop(ctx, serverAddr, machineID, queueList, int32(flags.concurrency), flags.jobLogsDir)
	log.Println("Worker stopped")
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
		log.Fatalf("Failed to marshal registration request: %v", err)
	}

	url := strings.TrimRight(apiURL, "/") + "/api/v1/worker/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Fatalf("REST registration failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read registration response: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Fatalf("Registration error %d: %s", resp.StatusCode, string(respBody))
	}

	var result types.SetupResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		log.Fatalf("Failed to parse registration response: %v", err)
	}

	log.Printf("Registered: machine_id=%s grpc=%s", result.MachineID, result.GRPCAddress)
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

		log.Printf("Stream disconnected: %v — reconnecting in %s", err, backoff)
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

	log.Printf("Connected to %s", serverAddr)

	var sendMu sync.Mutex
	draining := false

	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		if ping := msg.GetHeartbeatPing(); ping != nil {
			log.Printf("Heartbeat ping received")
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
					result.LogFile = "logs-" + a.JobId
					result.ExecutionLogFile = "execution-" + a.JobId
				}
				if jobErr != nil {
					result.Error = jobErr.Error()
					log.Printf("Job %s failed: %v", a.JobId, jobErr)
				} else {
					log.Printf("Job %s completed in %d ms", a.JobId, elapsed)
				}

				sendMu.Lock()
				sendErr := stream.Send(&pb.WorkerMessage{
					MachineId: machineID,
					Payload:   &pb.WorkerMessage_Result{Result: result},
				})
				sendMu.Unlock()
				if sendErr != nil {
					log.Printf("Failed to send job result for %s: %v", a.JobId, sendErr)
				}
			}(assign)
		}

		if msg.GetDrain() != nil {
			log.Println("Drain signal received, finishing in-flight jobs")
			draining = true
		}
	}
}
