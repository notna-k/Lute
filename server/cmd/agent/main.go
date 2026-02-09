package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/lute/server/proto/agent"
)

var (
	// Version is set at build time via -ldflags
	Version   = "dev"
	BuildTime = "unknown"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SetupRequest is sent to the server to register a new machine
type SetupRequest struct {
	Name     string            `json:"name"`
	Hostname string            `json:"hostname"`
	OS       string            `json:"os"`
	Arch     string            `json:"arch"`
	CPUs     int               `json:"cpus"`
	IP       string            `json:"ip"`
	Version  string            `json:"version"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SetupResponse is returned by the server after registration
type SetupResponse struct {
	MachineID   string `json:"machine_id"`
	AgentID     string `json:"agent_id"`
	GRPCAddress string `json:"grpc_address"`
	Message     string `json:"message"`
}

// pendingCmd represents a command received from the server config
type pendingCmd struct {
	ID      string   `json:"id"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Flags
	serverAddr := flag.String("server", "localhost:50051", "gRPC server address")
	apiURL := flag.String("api", "http://localhost:8080", "HTTP API base URL")
	machineID := flag.String("machine-id", "", "Machine ID to register with")
	agentID := flag.String("agent-id", "", "Agent ID (auto-generated if empty)")
	heartbeatInterval := flag.Duration("heartbeat", 30*time.Second, "Heartbeat interval")
	configPollInterval := flag.Duration("config-poll", 60*time.Second, "Config poll interval")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	setupFlag := flag.Bool("setup", false, "Run interactive setup: register this VM with the server")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("lute-agent %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// ---- SETUP MODE ----
	if *setupFlag {
		runSetup(*apiURL)
		return
	}

	// ---- NORMAL (DAEMON) MODE ----
	if *machineID == "" {
		log.Fatal("--machine-id is required (run with --setup to register first)")
	}

	if *agentID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		*agentID = fmt.Sprintf("agent-%s-%d", hostname, time.Now().Unix())
	}

	log.Printf("Lute Agent %s starting (build: %s)", Version, BuildTime)
	log.Printf("  Server:     %s", *serverAddr)
	log.Printf("  Machine ID: %s", *machineID)
	log.Printf("  Agent ID:   %s", *agentID)

	// ----- gRPC connection -----
	conn, err := grpc.NewClient(*serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)

	// 1. RegisterAgent
	regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
	resp, err := client.RegisterAgent(regCtx, &pb.RegisterAgentRequest{
		AgentId:   *agentID,
		MachineId: *machineID,
		Version:   Version,
		IpAddress: getLocalIP(),
		Metadata: map[string]string{
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"go_version": runtime.Version(),
			"hostname":   mustHostname(),
		},
	})
	regCancel()
	if err != nil {
		log.Fatalf("RegisterAgent failed: %v", err)
	}
	log.Printf("Registered with server: %s (server %s)", resp.Message, resp.ServerVersion)

	// 2. UpdateMachineStatus → "running"
	updateStatus(client, *agentID, *machineID, "running", "Agent started")

	// Root context — cancelled on SIGINT/SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// 3. Heartbeat loop (with system metrics)
	wg.Add(1)
	go func() {
		defer wg.Done()
		heartbeatLoop(ctx, client, *agentID, *heartbeatInterval)
	}()

	// 4. Config poll loop (also picks up pending commands)
	wg.Add(1)
	go func() {
		defer wg.Done()
		configPollLoop(ctx, client, *agentID, *machineID, *configPollInterval)
	}()

	// 5. StreamLogs (real-time event channel from server)
	wg.Add(1)
	go func() {
		defer wg.Done()
		streamLogsLoop(ctx, client, *agentID, *machineID)
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down agent...")

	// UpdateMachineStatus → "stopped"
	updateStatus(client, *agentID, *machineID, "stopped", "Agent shutting down")

	// Final heartbeat
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, _ = client.Heartbeat(shutCtx, &pb.HeartbeatRequest{
		AgentId: *agentID,
		Status:  "stopped",
	})
	shutCancel()

	cancel() // signal goroutines
	wg.Wait()
	log.Println("Agent stopped")
}

// ---------------------------------------------------------------------------
// updateStatus helper
// ---------------------------------------------------------------------------
func updateStatus(client pb.AgentServiceClient, agentID, machineID, status, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.UpdateMachineStatus(ctx, &pb.UpdateMachineStatusRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Status:    status,
		Message:   message,
	})
	if err != nil {
		log.Printf("UpdateMachineStatus(%s) failed: %v", status, err)
		return
	}
	log.Printf("UpdateMachineStatus(%s): %s", status, resp.Message)
}

// ---------------------------------------------------------------------------
// Heartbeat loop — sends status + system metrics
// ---------------------------------------------------------------------------
func heartbeatLoop(ctx context.Context, client pb.AgentServiceClient, agentID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Heartbeat loop started (every %s)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Heartbeat loop stopped")
			return
		case <-ticker.C:
			hbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, err := client.Heartbeat(hbCtx, &pb.HeartbeatRequest{
				AgentId: agentID,
				Status:  "running",
				Metrics: collectMetrics(),
			})
			cancel()

			if err != nil {
				log.Printf("Heartbeat failed: %v", err)
			}
		}
	}
}

// collectMetrics gathers lightweight system metrics
func collectMetrics() map[string]string {
	m := map[string]string{
		"num_goroutine": strconv.Itoa(runtime.NumGoroutine()),
		"num_cpu":       strconv.Itoa(runtime.NumCPU()),
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	m["mem_alloc_mb"] = strconv.FormatUint(mem.Alloc/1024/1024, 10)
	m["mem_sys_mb"] = strconv.FormatUint(mem.Sys/1024/1024, 10)

	return m
}

// ---------------------------------------------------------------------------
// Config poll loop — fetches config + executes pending commands
// ---------------------------------------------------------------------------
func configPollLoop(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Config poll started (every %s)", interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Config poll stopped")
			return
		case <-ticker.C:
			pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			resp, err := client.GetMachineConfig(pollCtx, &pb.GetMachineConfigRequest{
				AgentId:   agentID,
				MachineId: machineID,
			})
			cancel()

			if err != nil {
				log.Printf("GetMachineConfig failed: %v", err)
				continue
			}

			if !resp.Success {
				log.Printf("GetMachineConfig: server returned failure: %s", resp.Message)
				continue
			}

			// Apply config updates
			applyConfig(resp.Config)

			// Execute any pending commands
			if raw, ok := resp.Config["pending_commands"]; ok && raw != "" {
				var cmds []pendingCmd
				if err := json.Unmarshal([]byte(raw), &cmds); err != nil {
					log.Printf("Failed to parse pending_commands: %v", err)
					continue
				}
				for _, cmd := range cmds {
					go executeCommand(ctx, client, agentID, machineID, cmd)
				}
			}
		}
	}
}

// applyConfig applies server-pushed configuration
func applyConfig(cfg map[string]string) {
	if lvl, ok := cfg["log_level"]; ok {
		log.Printf("Config: log_level=%s", lvl)
	}
	// heartbeat_interval changes would require restarting the ticker;
	// for now we just log it.
	if hb, ok := cfg["heartbeat_interval"]; ok {
		log.Printf("Config: heartbeat_interval=%ss", hb)
	}
}

// executeCommand runs a command locally and reports the result back to the server
func executeCommand(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string, cmd pendingCmd) {
	log.Printf("Executing command %s: %s %v", cmd.ID, cmd.Command, cmd.Args)

	// Notify server that we're picking up this command
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, _ = client.ExecuteCommand(execCtx, &pb.ExecuteCommandRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Command:   cmd.Command,
		Args:      cmd.Args,
		Env:       map[string]string{"command_id": cmd.ID, "stage": "start"},
	})

	// Execute locally
	c := exec.CommandContext(execCtx, cmd.Command, cmd.Args...)
	output, err := c.CombinedOutput()

	exitCode := 0
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Report result back
	_, reportErr := client.ExecuteCommand(execCtx, &pb.ExecuteCommandRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Command:   cmd.Command,
		Args:      cmd.Args,
		Env: map[string]string{
			"command_id": cmd.ID,
			"stage":      "done",
			"output":     string(output),
			"exit_code":  strconv.Itoa(exitCode),
			"error":      errMsg,
		},
	})
	if reportErr != nil {
		log.Printf("Failed to report command result for %s: %v", cmd.ID, reportErr)
	} else {
		log.Printf("Command %s finished: exit=%d", cmd.ID, exitCode)
	}
}

// ---------------------------------------------------------------------------
// StreamLogs — subscribe to server event stream
// ---------------------------------------------------------------------------
func streamLogsLoop(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string) {
	for {
		select {
		case <-ctx.Done():
			log.Println("StreamLogs loop stopped")
			return
		default:
		}

		if err := streamLogs(ctx, client, agentID, machineID); err != nil {
			log.Printf("StreamLogs disconnected: %v — reconnecting in 10s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}
}

func streamLogs(ctx context.Context, client pb.AgentServiceClient, agentID, machineID string) error {
	stream, err := client.StreamLogs(ctx, &pb.StreamLogsRequest{
		AgentId:   agentID,
		MachineId: machineID,
		Level:     "info",
	})
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}

	log.Println("StreamLogs connected")

	for {
		msg, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("stream recv error: %w", err)
		}

		// Handle server-pushed messages
		switch msg.Source {
		case "command_queue":
			log.Printf("StreamLogs [command]: %s", msg.Message)
			// Commands are also delivered via config poll; this is a real-time hint
		default:
			log.Printf("StreamLogs [%s/%s]: %s", msg.Source, msg.Level, msg.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// Setup mode — interactive registration via HTTP
// ---------------------------------------------------------------------------
func runSetup(apiURL string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       Lute Agent Setup               ║")
	fmt.Printf("║       Version: %-21s ║\n", Version)
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. Prompt for service name
	fmt.Print("Enter service name: ")
	serviceName, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		log.Fatal("Service name cannot be empty")
	}

	// 2. Collect system information
	fmt.Println()
	fmt.Println("Collecting system information...")

	hostname, _ := os.Hostname()
	localIP := getLocalIP()

	sysInfo := &SetupRequest{
		Name:     serviceName,
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

	fmt.Printf("  Name:     %s\n", sysInfo.Name)
	fmt.Printf("  Hostname: %s\n", sysInfo.Hostname)
	fmt.Printf("  OS:       %s\n", sysInfo.OS)
	fmt.Printf("  Arch:     %s\n", sysInfo.Arch)
	fmt.Printf("  CPUs:     %d\n", sysInfo.CPUs)
	fmt.Printf("  IP:       %s\n", sysInfo.IP)
	fmt.Println()

	// 3. Register with the server
	fmt.Printf("Registering with server at %s ...\n", apiURL)

	body, err := json.Marshal(sysInfo)
	if err != nil {
		log.Fatalf("Failed to serialize request: %v", err)
	}

	url := strings.TrimRight(apiURL, "/") + "/api/v1/agent/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Fatalf("Server returned error %d: %s", resp.StatusCode, string(respBody))
	}

	var setupResp SetupResponse
	if err := json.Unmarshal(respBody, &setupResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ Machine registered successfully!")
	fmt.Printf("  Machine ID: %s\n", setupResp.MachineID)
	fmt.Printf("  Agent ID:   %s\n", setupResp.AgentID)
	fmt.Println()

	// 4. Auto-start the agent in detached (background) mode
	fmt.Println("Starting agent in background...")

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: cannot find own binary path: %v", err)
		fmt.Println("Could not auto-start. Run manually:")
		fmt.Printf("  lute-agent --server %s --machine-id %s --agent-id %s\n",
			setupResp.GRPCAddress, setupResp.MachineID, setupResp.AgentID)
		return
	}

	logFile := fmt.Sprintf("/tmp/lute-agent-%s.log", setupResp.MachineID)
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Warning: cannot open log file %s: %v", logFile, err)
		fmt.Println("Could not auto-start. Run manually:")
		fmt.Printf("  lute-agent --server %s --machine-id %s --agent-id %s\n",
			setupResp.GRPCAddress, setupResp.MachineID, setupResp.AgentID)
		return
	}

	cmd := exec.Command(exePath,
		"--server", setupResp.GRPCAddress,
		"--machine-id", setupResp.MachineID,
		"--agent-id", setupResp.AgentID,
	)
	cmd.Stdout = lf
	cmd.Stderr = lf

	// Set platform-specific process attributes for detaching from terminal
	setDetachedProcessAttr(cmd)

	if err := cmd.Start(); err != nil {
		lf.Close()
		log.Printf("Warning: failed to start agent: %v", err)
		fmt.Println("Could not auto-start. Run manually:")
		fmt.Printf("  lute-agent --server %s --machine-id %s --agent-id %s\n",
			setupResp.GRPCAddress, setupResp.MachineID, setupResp.AgentID)
		return
	}

	lf.Close()

	fmt.Printf("✓ Agent started (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("  Logs: %s\n", logFile)
	fmt.Println()
	fmt.Println("Manage:")
	fmt.Printf("  Stop:   kill %d\n", cmd.Process.Pid)
	fmt.Printf("  Logs:   tail -f %s\n", logFile)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// getLocalIP returns the first non-loopback IPv4 address
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}
