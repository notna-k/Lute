// Package setup registers this host with the Lute server and starts the agent in the background.
package setup

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Options configure a setup run.
type Options struct {
	APIURL    string
	ClaimCode string
	Version   string
	BuildTime string
}

type registerRequest struct {
	Name      string            `json:"name"`
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	Arch      string            `json:"arch"`
	CPUs      int               `json:"cpus"`
	IP        string            `json:"ip"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ClaimCode string            `json:"claim_code,omitempty"`
}

type registerResponse struct {
	WorkerID    string `json:"worker_id"`
	GRPCAddress string `json:"grpc_address"`
}

// DaemonLogPath is where the background agent's stdout and stderr go; `lute-worker logs` reads it.
func DaemonLogPath() string {
	return filepath.Join(os.TempDir(), "lute-worker.log")
}

// Run prompts for a service name on stdin, registers the host, and starts the agent in the background.
func Run(opts Options) error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║       Lute Worker Setup              ║")
	fmt.Printf("║       Version: %-21s ║\n", opts.Version)
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	name, err := promptServiceName(os.Stdin)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Collecting system information...")
	req := systemInfo(name, opts)
	printSystemInfo(req)

	fmt.Printf("Registering with server at %s ...\n", opts.APIURL)
	resp, err := register(opts.APIURL, req)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("✓ Worker registered successfully!")
	fmt.Printf("  Worker ID: %s\n", resp.WorkerID)
	fmt.Println()

	fmt.Println("Starting worker in background...")
	pid, err := startAgent(resp)
	if err != nil {
		fmt.Printf("Could not auto-start (%v). Run manually:\n", err)
		fmt.Printf("  lute-worker run --server %s --worker-id %s\n", resp.GRPCAddress, resp.WorkerID)
		return nil
	}
	fmt.Printf("✓ Worker started (PID %d)\n", pid)
	fmt.Printf("  Logs: %s\n", DaemonLogPath())
	fmt.Println()
	fmt.Println("Manage:")
	fmt.Printf("  Stop:   kill %d\n", pid)
	fmt.Println("  Logs:   lute-worker logs -f")
	return nil
}

func promptServiceName(in io.Reader) (string, error) {
	fmt.Print("Enter service name: ")
	name, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || name == "") {
		return "", fmt.Errorf("read service name: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("service name cannot be empty")
	}
	return name, nil
}

func systemInfo(name string, opts Options) *registerRequest {
	hostname, _ := os.Hostname()
	return &registerRequest{
		Name:      name,
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
		IP:        localIP(),
		Version:   opts.Version,
		ClaimCode: opts.ClaimCode,
		Metadata: map[string]string{
			"go_version": runtime.Version(),
			"build_time": opts.BuildTime,
		},
	}
}

func printSystemInfo(r *registerRequest) {
	fmt.Printf("  Name:     %s\n", r.Name)
	fmt.Printf("  Hostname: %s\n", r.Hostname)
	fmt.Printf("  OS:       %s\n", r.OS)
	fmt.Printf("  Arch:     %s\n", r.Arch)
	fmt.Printf("  CPUs:     %d\n", r.CPUs)
	fmt.Printf("  IP:       %s\n", r.IP)
	fmt.Println()
}

// localIP returns the first non-loopback IPv4 address, or "unknown".
func localIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "unknown"
}

func register(apiURL string, req *registerRequest) (*registerResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	url := strings.TrimRight(apiURL, "/") + "/api/public/v1/workers/bootstrap/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("connect to server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	case http.StatusConflict:
		return nil, fmt.Errorf("registration rejected, another worker is already registered for this machine: %s\n"+
			"Delete the existing worker in the Lute UI, or stop its agent process, then try again",
			errorMessage(respBody))
	default:
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, errorMessage(respBody))
	}

	var out registerResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &out, nil
}

// errorMessage pulls the message out of core's {"error": {"message"}} body, falling back to the raw body.
func errorMessage(body []byte) string {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		return envelope.Error.Message
	}
	return strings.TrimSpace(string(body))
}

// startAgent re-executes this binary as a detached `run` that logs to DaemonLogPath.
func startAgent(resp *registerResponse) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("find own binary: %w", err)
	}

	lf, err := os.OpenFile(DaemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log file: %w", err)
	}
	defer func() { _ = lf.Close() }()

	cmd := exec.Command(exe, "run", "--server", resp.GRPCAddress, "--worker-id", resp.WorkerID)
	cmd.Stdout = lf
	cmd.Stderr = lf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}
