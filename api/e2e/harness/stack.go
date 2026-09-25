//go:build e2e

package harness

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lute/api/internal/config"
	"github.com/lute/api/internal/server"
	"github.com/lute/api/internal/setup"
	"github.com/lute/api/internal/testutil/pgtest"
)

const (
	AdminEmail    = "admin@e2e.test"
	AdminPassword = "e2e-admin-password"
	jwtSecret     = "e2e-jwt-secret-long-enough-for-hs256-signing"
)

// Stack is one Lute: its own database, core in this process, and the agents a test starts.
type Stack struct {
	t           *testing.T
	DSN         string
	JobDefsDir  string
	ArtifactDir string
	Config      *config.Config

	deps   *setup.Deps
	server *server.Server

	agents []*Agent
}

type StackOption func(*config.Config)

func WithJobDefsDir(dir string) StackOption {
	return func(c *config.Config) { c.JobDefs.Dir = dir }
}

// WithFastHeartbeat is opt-in: aggressive pings mark a worker dead between registering
// it and starting its agent.
func WithFastHeartbeat(interval, pingTimeout time.Duration, maxRetries int) StackOption {
	return func(c *config.Config) {
		c.Heartbeat.CheckInterval = interval
		c.Heartbeat.PingTimeout = pingTimeout
		c.Heartbeat.MaxRetries = maxRetries
	}
}

func WithLeaseGrace(grace, reclaimAfter time.Duration) StackOption {
	return func(c *config.Config) {
		c.Queue.LeaseGrace = grace
		c.Queue.ReclaimAfter = reclaimAfter
	}
}

func NewStack(t *testing.T, pg *pgtest.Server, opts ...StackOption) *Stack {
	t.Helper()

	s := &Stack{
		t:           t,
		DSN:         pg.CreateDatabase(t),
		JobDefsDir:  t.TempDir(),
		ArtifactDir: artifactDir(t),
	}
	s.Config = s.baseConfig()
	for _, opt := range opts {
		opt(s.Config)
	}
	if s.Config.JobDefs.Dir != "" {
		s.JobDefsDir = s.Config.JobDefs.Dir
	} else {
		s.Config.JobDefs.Dir = s.JobDefsDir
	}

	s.captureLogs()
	s.start()

	t.Cleanup(func() {
		for _, a := range s.agents {
			a.stopQuietly()
		}
		s.stop()
	})
	return s
}

func (s *Stack) baseConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:           "127.0.0.1",
			Port:           "0",
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    60 * time.Second,
			Mode:           "test",
			AllowedOrigins: []string{"http://localhost:8080"},
		},
		Database: config.DatabaseConfig{DSN: s.DSN},
		GRPC:     config.GRPCConfig{Host: "127.0.0.1", Port: "0"},
		// Effectively off unless a test asks for WithFastHeartbeat.
		Heartbeat: config.HeartbeatConfig{
			CheckInterval: time.Hour,
			PingTimeout:   2 * time.Second,
			MaxRetries:    3,
		},
		WebSocket: config.WebSocketConfig{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     true,
			PingPeriod:      54 * time.Second,
			PongWait:        60 * time.Second,
			WriteWait:       10 * time.Second,
		},
		Auth: config.AuthConfig{
			JWTSecret:     jwtSecret,
			AccessTTL:     15 * time.Minute,
			RefreshTTL:    time.Hour,
			Issuer:        "lute",
			AdminEmail:    AdminEmail,
			AdminPassword: AdminPassword,
		},
		WorkerBinary: config.WorkerBinaryConfig{Dir: WorkerBinDir()},
		Metrics:      config.MetricsConfig{SnapshotInterval: time.Hour},
		Queue: config.QueueConfig{
			PollInterval: 200 * time.Millisecond,
			LeaseGrace:   2 * time.Second,
			ReclaimAfter: 2 * time.Second,
		},
		Webhooks: config.WebhooksConfig{PollInterval: 200 * time.Millisecond},
	}
}

func (s *Stack) start() {
	s.t.Helper()

	deps, err := setup.New(context.Background(), s.Config)
	if err != nil {
		s.t.Fatalf("initialize core: %v", err)
	}
	s.deps = deps

	srv := server.New(deps)
	if err := srv.Start(); err != nil {
		deps.Close()
		s.t.Fatalf("start core: %v", err)
	}
	s.server = srv

	// Pin the assigned ports, so after a Restart running agents reconnect to the same addresses.
	_, httpPort := hostPort(srv.HTTPAddr())
	_, grpcPort := hostPort(srv.GRPCAddr())
	s.Config.Server.Port = httpPort
	s.Config.GRPC.Port = grpcPort

	s.waitHealthy()
}

func (s *Stack) stop() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.t.Logf("shutdown core: %v", err)
		}
		s.server = nil
	}
	if s.deps != nil {
		s.deps.Close()
		s.deps = nil
	}
}

// Restart brings core back on the same database and ports, as a deploy looks to an agent.
func (s *Stack) Restart() {
	s.t.Helper()
	s.stop()
	s.start()
}

func (s *Stack) BaseURL() string { return "http://" + s.server.HTTPAddr() }

func (s *Stack) GRPCAddr() string { return s.server.GRPCAddr() }

func (s *Stack) Client() *Client { return NewClient(s.t, s.BaseURL()) }

func (s *Stack) AdminClient() *Client {
	s.t.Helper()
	c := s.Client()
	if _, err := c.Login(AdminEmail, AdminPassword); err != nil {
		s.t.Fatalf("admin login: %v", err)
	}
	return c
}

func (s *Stack) Agents() []*Agent { return append([]*Agent(nil), s.agents...) }

// WriteJobDef does not sync: call Client.SyncJobDefs, or write before the stack boots.
func (s *Stack) WriteJobDef(name, yaml string) {
	s.t.Helper()
	path := filepath.Join(s.JobDefsDir, name)
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		s.t.Fatalf("write job def %s: %v", name, err)
	}
}

func (s *Stack) RemoveJobDef(name string) {
	s.t.Helper()
	if err := os.Remove(filepath.Join(s.JobDefsDir, name)); err != nil {
		s.t.Fatalf("remove job def %s: %v", name, err)
	}
}

func (s *Stack) waitHealthy() {
	s.t.Helper()
	url := s.BaseURL() + "/api/health"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url) //nolint:gosec // fixed loopback URL
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("core did not become healthy at %s", url)
}

// captureLogs sends core's log (slog's default handler writes through package log) to the artifact dir.
func (s *Stack) captureLogs() {
	s.t.Helper()
	path := filepath.Join(s.ArtifactDir, "core.log")
	f, err := os.Create(path) //nolint:gosec // path is derived from the test name
	if err != nil {
		s.t.Logf("cannot capture core logs: %v", err)
		return
	}
	prevOut, prevFlags, prevPrefix := log.Writer(), log.Flags(), log.Prefix()
	log.SetOutput(f)
	s.t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		log.SetPrefix(prevPrefix)
		_ = f.Close()
	})
}

func artifactDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("LUTE_E2E_ARTIFACTS")
	if root == "" {
		root = "_artifacts"
	}
	dir := filepath.Join(root, safeName(t.Name()))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}
	return dir
}

func safeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func hostPort(addr string) (host, port string) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:]
		}
	}
	return addr, ""
}
