package setup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lute/worker/internal/setup/types"
	"github.com/lute/worker/internal/utils"
)

// Run executes the interactive setup process.
// claimCode is optional; when set, the new machine is linked to that user.
func Run(apiURL, version, buildTime, claimCode string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       Lute Worker Setup              ║")
	fmt.Printf("║       Version: %-21s ║\n", version)
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. Prompt for service name
	serviceName := promptServiceName(reader)

	// 2. Collect system information
	sysInfo := collectSystemInfo(serviceName, version, buildTime, claimCode)
	displaySystemInfo(sysInfo)

	// 3. Register with the server
	setupResp := registerWithServer(apiURL, sysInfo)

	// 4. Auto-start the worker in detached (background) mode
	startWorker(setupResp, version)
}

// promptServiceName prompts the user for a service name
func promptServiceName(reader *bufio.Reader) string {
	fmt.Print("Enter service name: ")
	serviceName, err := reader.ReadString('\n')
	if err != nil {
		slog.Error("Failed to read input", "err", err)
		os.Exit(1)
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		slog.Error("Service name cannot be empty")
		os.Exit(1)
	}
	return serviceName
}

// collectSystemInfo gathers system information
func collectSystemInfo(serviceName, version, buildTime, claimCode string) *types.SetupRequest {
	fmt.Println()
	fmt.Println("Collecting system information...")

	hostname, _ := os.Hostname()
	localIP := utils.GetLocalIP()

	req := &types.SetupRequest{
		Name:      serviceName,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
		IP:        localIP,
		Version:   version,
		ClaimCode: claimCode,
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"build_time": buildTime,
		},
	}
	return req
}

// displaySystemInfo displays collected system information
func displaySystemInfo(sysInfo *types.SetupRequest) {
	fmt.Printf("  Name:     %s\n", sysInfo.Name)
	fmt.Printf("  Hostname: %s\n", sysInfo.Hostname)
	fmt.Printf("  OS:       %s\n", sysInfo.OS)
	fmt.Printf("  Arch:     %s\n", sysInfo.Arch)
	fmt.Printf("  CPUs:     %d\n", sysInfo.CPUs)
	fmt.Printf("  IP:       %s\n", sysInfo.IP)
	fmt.Println()
}

// registerWithServer sends registration request to the server
func registerWithServer(apiURL string, sysInfo *types.SetupRequest) *types.SetupResponse {
	fmt.Printf("Registering with server at %s ...\n", apiURL)

	body, err := json.Marshal(sysInfo)
	if err != nil {
		slog.Error("Failed to serialize request", "err", err)
		os.Exit(1)
	}

	url := strings.TrimRight(apiURL, "/") + "/api/v1/worker/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("Failed to connect to server", "err", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response", "err", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		slog.Error("Server returned error", "status", resp.StatusCode, "body", string(respBody))
		os.Exit(1)
	}

	var setupResp types.SetupResponse
	if err := json.Unmarshal(respBody, &setupResp); err != nil {
		slog.Error("Failed to parse response", "err", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ Machine registered successfully!")
	fmt.Printf("  Machine ID: %s\n", setupResp.MachineID)
	fmt.Println()

	return &setupResp
}

// startWorker starts the worker in background mode
func startWorker(setupResp *types.SetupResponse, version string) {
	fmt.Println("Starting worker in background...")

	exePath, err := os.Executable()
	if err != nil {
		slog.Warn("cannot find own binary path", "err", err)
		displayManualInstructions(setupResp)
		return
	}

	logFile := fmt.Sprintf("/tmp/lute-worker-%s.log", setupResp.MachineID)
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Warn("cannot open log file", "path", logFile, "err", err)
		displayManualInstructions(setupResp)
		return
	}
	defer lf.Close()

	cmd := createWorkerCommand(exePath, setupResp, lf)
	if err := cmd.Start(); err != nil {
		slog.Warn("failed to start worker", "err", err)
		displayManualInstructions(setupResp)
		return
	}

	displayStartupInfo(cmd.Process.Pid, logFile)
}

// createWorkerCommand creates the command to start the worker
func createWorkerCommand(exePath string, setupResp *types.SetupResponse, logFile *os.File) *exec.Cmd {
	cmd := exec.Command(exePath,
		"--server", setupResp.GRPCAddress,
		"--machine-id", setupResp.MachineID,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Set platform-specific process attributes for detaching from terminal
	setDetachedProcessAttr(cmd)

	return cmd
}

// displayManualInstructions shows manual start instructions
func displayManualInstructions(setupResp *types.SetupResponse) {
	fmt.Println("Could not auto-start. Run manually:")
	fmt.Printf("  lute-worker --server %s --machine-id %s\n",
		setupResp.GRPCAddress, setupResp.MachineID)
}

// displayStartupInfo displays information about the started agent
func displayStartupInfo(pid int, logFile string) {
	fmt.Printf("✓ Worker started (PID %d)\n", pid)
	fmt.Printf("  Logs: %s\n", logFile)
	fmt.Println()
	fmt.Println("Manage:")
	fmt.Printf("  Stop:   kill %d\n", pid)
	fmt.Printf("  Logs:   tail -f %s\n", logFile)
}
