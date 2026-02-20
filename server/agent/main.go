package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	agentclient "github.com/lute/agent/client"
	"github.com/lute/agent/config"
	"github.com/lute/agent/heartbeat"
	"github.com/lute/agent/logs"
	"github.com/lute/agent/setup"
	"github.com/lute/agent/utils"

	pb "github.com/lute/agent/proto/agent"
)

var (
	// Version is set at build time via -ldflags
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	flags := parseFlags()

	if flags.version {
		displayVersion()
		os.Exit(0)
	}

	if flags.setup {
		setup.Run(flags.apiURL, Version, BuildTime)
		return
	}

	runAgent(flags)
}

// Flags holds all command-line flags
type Flags struct {
	serverAddr         string
	apiURL             string
	machineID          string
	heartbeatInterval  time.Duration
	configPollInterval time.Duration
	version            bool
	setup              bool
}

// parseFlags parses and returns command-line flags
func parseFlags() *Flags {
	flags := &Flags{}

	flag.StringVar(&flags.serverAddr, "server", "localhost:50051", "gRPC server address")
	flag.StringVar(&flags.apiURL, "api", "http://localhost:8080", "HTTP API base URL")
	flag.StringVar(&flags.machineID, "machine-id", "", "Machine ID to register with")
	flag.DurationVar(&flags.heartbeatInterval, "heartbeat", 30*time.Second, "Heartbeat interval")
	flag.DurationVar(&flags.configPollInterval, "config-poll", 60*time.Second, "Config poll interval")
	flag.BoolVar(&flags.version, "version", false, "Print version and exit")
	flag.BoolVar(&flags.setup, "setup", false, "Run interactive setup: register this VM with the server")
	flag.Parse()

	return flags
}

// displayVersion prints version information
func displayVersion() {
	fmt.Printf("lute-agent %s (built %s)\n", Version, BuildTime)
}

// runAgent runs the agent in daemon mode
func runAgent(flags *Flags) {
	validateFlags(flags)

	logStartupInfo(flags)

	grpcClient := connectToServer(flags.serverAddr)
	defer grpcClient.Close()

	agentServiceClient := pb.NewAgentServiceClient(grpcClient)

	// Register and get the machine_id (might be newly created)
	machineID := registerAgent(agentServiceClient, flags.machineID)
	
	agentclient.UpdateStatus(agentServiceClient, machineID, "running", "Agent started")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startBackgroundTasks(ctx, agentServiceClient, machineID, flags)

	log.Println("Shutting down agent...")
	agentclient.UpdateStatus(agentServiceClient, machineID, "stopped", "Agent shutting down")
	heartbeat.SendFinal(context.Background(), agentServiceClient, machineID)

	cancel()
	log.Println("Agent stopped")
}

// validateFlags validates required flags
func validateFlags(flags *Flags) {
	// machine_id is now optional - if not provided, agent will auto-register
}

// logStartupInfo logs agent startup information
func logStartupInfo(flags *Flags) {
	log.Printf("Lute Agent %s starting (build: %s)", Version, BuildTime)
	log.Printf("  Server:     %s", flags.serverAddr)
	if flags.machineID != "" {
		log.Printf("  Machine ID: %s", flags.machineID)
	} else {
		log.Printf("  Machine ID: (will auto-register)")
	}
}

// connectToServer establishes gRPC connection to the server
func connectToServer(serverAddr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	return conn
}

// registerAgent registers the agent with the server
// Returns the machine_id (either provided or newly created by server)
func registerAgent(agentClient pb.AgentServiceClient, machineID string) string {
	regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer regCancel()

	metadata := map[string]string{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"go_version": runtime.Version(),
	}

	hostname := utils.MustHostname()
	returnedMachineID, err := agentclient.Register(regCtx, agentClient, machineID, Version, utils.GetLocalIP(), hostname, metadata)
	if err != nil {
		log.Fatalf("RegisterAgent failed: %v", err)
	}

	if machineID == "" {
		log.Printf("Registered new machine: %s (hostname: %s)", returnedMachineID, hostname)
	} else {
		log.Printf("Registered with server: machine_id=%s", returnedMachineID)
	}

	return returnedMachineID
}

// startBackgroundTasks starts all background goroutines and waits for shutdown
func startBackgroundTasks(ctx context.Context, client pb.AgentServiceClient, machineID string, flags *Flags) {
	var wg sync.WaitGroup

	// Heartbeat loop - uses machineID
	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeat.Loop(ctx, client, machineID, flags.heartbeatInterval)
	}()

	// Config poll loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		config.PollLoop(ctx, client, machineID, flags.configPollInterval)
	}()

	// Stream logs loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		logs.StreamLoop(ctx, client, machineID)
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Wait for all goroutines to finish
	wg.Wait()
}
