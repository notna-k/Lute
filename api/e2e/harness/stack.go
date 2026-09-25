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

// Credentials of the admin seeded into every stack.
const (
	AdminEmail    = "admin@e2e.test"
	AdminPassword = "e2e-admin-password"
	jwtSecret     = "e2e-jwt-secret-long-enough-for-hs256-signing"
)

// Stack is one running Lute: a database of its own, core in this process, and
// whatever agents a test starts against it.
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

// StackOption adjusts the config core boots with.
type StackOption func(*config.Config)

// WithJobDefsDir points core at an existing directory of definition YAML.
func WithJobDefsDir(dir string) StackOption {
	return func(c *config.Config) { c.JobDefs.Dir = dir }
}

// WithFastHeartbeat shortens agent liveness checks. Off by default: a stack that
// pings aggressively marks a worker dead in the gap between registering it and
// starting its agent, which is not what most tests are about.
func WithFastHeartbeat(interval, pingTimeout time.Duration, maxRetries int) StackOption {
	return func(c *config.Config) {
		c.Heartbeat.CheckInterval = interval
		c.Heartbeat.PingTimeout = pingTimeout
		c.Heartbeat.MaxRetries = maxRetries
	}
}

// WithLeaseGrace sets how long past its timeout a dispatched build counts as alive.
func WithLeaseGrace(grace, reclaimAfter time.Duration) StackOption {
	return func(c *config.Config) {
		c.Queue.LeaseGrace = grace
		c.Queue.ReclaimAfter = reclaimAfter
	}
}

// NewStack boots core against a fresh database and tears everything down on cleanup.
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
			Host:         "127.0.0.1",
			Port:         "0",
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
			Mode:         "test",
			// The panel's origin, so a WebSocket test can prove both the allowed
			// and the rejected case.
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

// start boots core and waits for it to answer. Splitting this from NewStack is what
// makes Restart possible.
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

	// Pin the ports the OS just handed out, so a Restart comes back on the same
	// addresses and a running agent can reconnect to them.
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

// Restart stops core and brings it back on the same database and the same ports —
// what a deploy of the stateless core looks like to a connected agent.
func (s *Stack) Restart() {
	s.t.Helper()
	s.stop()
	s.start()
}

// BaseURL is the HTTP origin of this stack's core.
func (s *Stack) BaseURL() string { return "http://" + s.server.HTTPAddr() }

// GRPCAddr is the address agents connect to.
func (s *Stack) GRPCAddr() string { return s.server.GRPCAddr() }

// Client returns an unauthenticated API client for this stack.
func (s *Stack) Client() *Client { return NewClient(s.t, s.BaseURL()) }

// AdminClient returns a client already signed in as the seeded admin.
func (s *Stack) AdminClient() *Client {
	s.t.Helper()
	c := s.Client()
	if _, err := c.Login(AdminEmail, AdminPassword); err != nil {
		s.t.Fatalf("admin login: %v", err)
	}
	return c
}

// Agents returns every agent this stack has started, for tests that need to take the
// whole fleet down.
func (s *Stack) Agents() []*Agent { return append([]*Agent(nil), s.agents...) }

// WriteJobDef puts a definition YAML in the directory core syncs from. It does not
// sync by itself: call Client.SyncJobDefs, or write before the stack boots.
func (s *Stack) WriteJobDef(name, yaml string) {
	s.t.Helper()
	path := filepath.Join(s.JobDefsDir, name)
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		s.t.Fatalf("write job def %s: %v", name, err)
	}
}

// RemoveJobDef deletes a definition file, as a commit removing it would.
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

// captureLogs tees core's log output into the test's artifact directory, so a CI
// failure comes with the server's side of the story.
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

// artifactDir is where a test's diagnostics land: core logs, agent stderr, job logs.
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
