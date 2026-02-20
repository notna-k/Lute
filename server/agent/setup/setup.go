package setup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/lute/agent/setup/types"
	"github.com/lute/agent/utils"
)

// Run executes the interactive setup process
func Run(apiURL, version, buildTime string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       Lute Agent Setup               ║")
	fmt.Printf("║       Version: %-21s ║\n", version)
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	// 1. Prompt for service name
	serviceName := promptServiceName(reader)

	// 2. Collect system information
	sysInfo := collectSystemInfo(serviceName, version, buildTime)
	displaySystemInfo(sysInfo)

	// 3. Register with the server
	setupResp := registerWithServer(apiURL, sysInfo)

	// 4. Auto-start the agent in detached (background) mode
	startAgent(setupResp, version)
}

// promptServiceName prompts the user for a service name
func promptServiceName(reader *bufio.Reader) string {
	fmt.Print("Enter service name: ")
	serviceName, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read input: %v", err)
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		log.Fatal("Service name cannot be empty")
	}
	return serviceName
}

// collectSystemInfo gathers system information
func collectSystemInfo(serviceName, version, buildTime string) *types.SetupRequest {
	fmt.Println()
	fmt.Println("Collecting system information...")

	hostname, _ := os.Hostname()
	localIP := utils.GetLocalIP()

	return &types.SetupRequest{
		Name:     serviceName,
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUs:     runtime.NumCPU(),
		IP:       localIP,
		Version:  version,
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"build_time": buildTime,
		},
	}
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

	var setupResp types.SetupResponse
	if err := json.Unmarshal(respBody, &setupResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	fmt.Println()
	fmt.Println("✓ Machine registered successfully!")
	fmt.Printf("  Machine ID: %s\n", setupResp.MachineID)
	fmt.Println()

	return &setupResp
}

// startAgent starts the agent in background mode
func startAgent(setupResp *types.SetupResponse, version string) {
	fmt.Println("Starting agent in background...")

	exePath, err := os.Executable()
	if err != nil {
		log.Printf("Warning: cannot find own binary path: %v", err)
		displayManualInstructions(setupResp)
		return
	}

	logFile := fmt.Sprintf("/tmp/lute-agent-%s.log", setupResp.MachineID)
	lf, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Warning: cannot open log file %s: %v", logFile, err)
		displayManualInstructions(setupResp)
		return
	}
	defer lf.Close()

	cmd := createAgentCommand(exePath, setupResp, lf)
	if err := cmd.Start(); err != nil {
		log.Printf("Warning: failed to start agent: %v", err)
		displayManualInstructions(setupResp)
		return
	}

	displayStartupInfo(cmd.Process.Pid, logFile)
}

// createAgentCommand creates the command to start the agent
func createAgentCommand(exePath string, setupResp *types.SetupResponse, logFile *os.File) *exec.Cmd {
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
	fmt.Printf("  lute-agent --server %s --machine-id %s\n",
		setupResp.GRPCAddress, setupResp.MachineID)
}

// displayStartupInfo displays information about the started agent
func displayStartupInfo(pid int, logFile string) {
	fmt.Printf("✓ Agent started (PID %d)\n", pid)
	fmt.Printf("  Logs: %s\n", logFile)
	fmt.Println()
	fmt.Println("Manage:")
	fmt.Printf("  Stop:   kill %d\n", pid)
	fmt.Printf("  Logs:   tail -f %s\n", logFile)
}
