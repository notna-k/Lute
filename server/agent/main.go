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
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lute/agent/metrics"
	"github.com/lute/agent/setup"
	"github.com/lute/agent/setup/types"
	"github.com/lute/agent/utils"

	pb "github.com/lute/agent/proto/agent"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

type Flags struct {
	serverAddr string
	apiURL     string
	machineID  string
	version    bool
	setupMode  bool
}

func main() {
	flags := parseFlags()

	if flags.version {
		fmt.Printf("lute-agent %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	if flags.setupMode {
		setup.Run(flags.apiURL, Version, BuildTime)
		return
	}

	runAgent(flags)
}

func parseFlags() *Flags {
	f := &Flags{}
	flag.StringVar(&f.serverAddr, "server", "localhost:50051", "gRPC server address")
	flag.StringVar(&f.apiURL, "api", "http://localhost:8080", "HTTP API base URL")
	flag.StringVar(&f.machineID, "machine-id", "", "Machine ID (skip REST registration if provided)")
	flag.BoolVar(&f.version, "version", false, "Print version and exit")
	flag.BoolVar(&f.setupMode, "setup", false, "Run interactive setup")
	flag.Parse()
	return f
}

func runAgent(flags *Flags) {
	log.Printf("Lute Agent %s starting (build: %s)", Version, BuildTime)

	machineID := flags.machineID
	serverAddr := flags.serverAddr

	// If no machine-id, register via REST and obtain one.
	if machineID == "" {
		var grpcAddr string
		machineID, grpcAddr = registerViaREST(flags.apiURL)
		if grpcAddr != "" {
			serverAddr = grpcAddr
		}
	}

	log.Printf("  Machine ID: %s", machineID)
	log.Printf("  Server:     %s", serverAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Shutting down agent...")
		cancel()
	}()

	// Persistent connection loop with reconnection.
	connectLoop(ctx, serverAddr, machineID)
	log.Println("Agent stopped")
}

// registerViaREST calls POST /api/v1/agent/register and returns
// (machine_id, grpc_address).
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

	url := strings.TrimRight(apiURL, "/") + "/api/v1/agent/register"
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

// connectLoop keeps the bidirectional stream alive, reconnecting with
// exponential backoff on failure.
func connectLoop(ctx context.Context, serverAddr, machineID string) {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return
		}

		err := runStream(ctx, serverAddr, machineID)
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

// runStream opens a single Connect stream and processes heartbeat pings
// until the stream breaks or the context is cancelled.
func runStream(ctx context.Context, serverAddr, machineID string) error {
	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	// Send initial message with machine_id.
	if err := stream.Send(&pb.AgentMessage{MachineId: machineID}); err != nil {
		return fmt.Errorf("send initial: %w", err)
	}

	log.Printf("Connected to %s", serverAddr)

	// Reset backoff on successful connect (caller handles backoff).
	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		if ping := msg.GetHeartbeatPing(); ping != nil {
			pong := &pb.HeartbeatPong{
				Status:    "running",
				Metrics:   metrics.Collect(),
				Timestamp: time.Now().Unix(),
			}
			if err := stream.Send(&pb.AgentMessage{
				MachineId: machineID,
				Payload:   &pb.AgentMessage_HeartbeatPong{HeartbeatPong: pong},
			}); err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		}
	}
}
