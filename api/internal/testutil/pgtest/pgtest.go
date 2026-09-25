// Package pgtest provides a real Postgres server for tests: the one named by
// LUTE_E2E_POSTGRES_DSN, or a throwaway container on the host Docker daemon.
package pgtest

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for the admin connection
)

// DSNEnv names a pre-provisioned server to use instead of starting a container.
const DSNEnv = "LUTE_E2E_POSTGRES_DSN"

// image matches what the dev stack and production run, so tests see the same dialect.
const image = "postgres:17-alpine"

// label marks the containers this package starts; pidLabel records the owning process,
// so a later run reaps only leftovers and never a parallel package's server.
const (
	label    = "lute-test=postgres"
	pidLabel = "lute-test.pid"
)

const (
	defaultUser     = "lute"
	defaultPassword = "lute"
	adminDB         = "postgres"
)

// Server is a Postgres instance tests carve databases out of.
type Server struct {
	host        string
	port        string
	user        string
	password    string
	containerID string
}

var dbSeq atomic.Uint64

// Start reuses DSNEnv when set, otherwise runs a container on an ephemeral port.
// Call Stop when done.
func Start() (*Server, error) {
	if dsn := strings.TrimSpace(os.Getenv(DSNEnv)); dsn != "" {
		s, err := parseDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", DSNEnv, err)
		}
		if err := s.waitReady(60 * time.Second); err != nil {
			return nil, err
		}
		return s, nil
	}

	if exec.Command("docker", "info").Run() != nil {
		return nil, fmt.Errorf("no Docker daemon and no %s: cannot provide a database", DSNEnv)
	}
	reapLabelled()

	id, err := docker("run", "-d", "--rm",
		"--label", label,
		"--label", pidLabel+"="+strconv.Itoa(os.Getpid()),
		"-e", "POSTGRES_USER="+defaultUser,
		"-e", "POSTGRES_PASSWORD="+defaultPassword,
		"-e", "POSTGRES_DB="+adminDB,
		"-p", "127.0.0.1:0:5432", image,
		// Durability off: these databases are thrown away and migrations dominate runtime.
		"-c", "fsync=off", "-c", "full_page_writes=off", "-c", "synchronous_commit=off",
	)
	if err != nil {
		return nil, err
	}
	mapped, err := docker("port", id, "5432/tcp")
	if err != nil {
		_, _ = docker("rm", "-f", id)
		return nil, err
	}
	host, port, err := splitMapping(mapped)
	if err != nil {
		_, _ = docker("rm", "-f", id)
		return nil, err
	}

	s := &Server{host: host, port: port, user: defaultUser, password: defaultPassword, containerID: id}
	if err := s.waitReady(90 * time.Second); err != nil {
		s.Stop()
		return nil, err
	}
	log.Printf("pgtest: started %s on %s:%s (container %s)", image, host, port, id[:12])
	return s, nil
}

// Stop removes the container Start ran. A reused server is left alone.
func (s *Server) Stop() {
	if s == nil || s.containerID == "" {
		return
	}
	if _, err := docker("rm", "-f", s.containerID); err != nil {
		log.Printf("pgtest: removing postgres container: %v", err)
	}
	s.containerID = ""
}

// CreateDatabase makes a fresh database, drops it on cleanup and returns its DSN.
func (s *Server) CreateDatabase(t testing.TB) string {
	t.Helper()

	name := fmt.Sprintf("lute_test_%d_%d", time.Now().UnixNano()%1e9, dbSeq.Add(1))
	admin, err := sql.Open("pgx", s.dsn(adminDB))
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	admin.SetMaxOpenConns(2)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	t.Cleanup(func() {
		// A leaked connection makes DROP fail and the next run inherits the database.
		_, _ = admin.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name); err != nil {
			t.Logf("drop database %s: %v", name, err)
		}
		_ = admin.Close()
	})
	return s.dsn(name)
}

var shared *Server

// Main starts a server for a package's tests and stops it afterwards. Without Docker
// or DSNEnv the tests skip locally but fail under CI, so CI never loses them silently.
func Main(m *testing.M) int {
	s, err := Start()
	if err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintln(os.Stderr, "pgtest:", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, "pgtest: Postgres tests will skip:", err)
		return m.Run()
	}
	shared = s
	defer s.Stop()
	return m.Run()
}

// NewDatabase creates a database on the server Main started and returns its DSN.
func NewDatabase(t testing.TB) string {
	t.Helper()
	if shared == nil {
		t.Skip("no Postgres available (run with Docker or set " + DSNEnv + ")")
	}
	return shared.CreateDatabase(t)
}

func (s *Server) dsn(database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		s.user, s.password, s.host, s.port, database)
}

func (s *Server) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("pgx", s.dsn(adminDB))
		if err == nil {
			lastErr = db.Ping()
			_ = db.Close()
			if lastErr == nil {
				return nil
			}
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("postgres at %s:%s not ready within %s: %w", s.host, s.port, timeout, lastErr)
}

// splitMapping reads host and port from `docker port`, which may print one line per address family.
func splitMapping(mapped string) (host, port string, err error) {
	for line := range strings.SplitSeq(strings.TrimSpace(mapped), "\n") {
		h, p, serr := net.SplitHostPort(strings.TrimSpace(line))
		if serr != nil {
			continue
		}
		if h == "0.0.0.0" || h == "::" || h == "" {
			h = "127.0.0.1"
		}
		if _, cerr := strconv.Atoi(p); cerr != nil {
			continue
		}
		return h, p, nil
	}
	return "", "", fmt.Errorf("cannot read a host:port out of %q", mapped)
}

// parseDSN reads the server coordinates from a URL-style DSN; the database it names is ignored.
func parseDSN(dsn string) (*Server, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("want a postgres:// URL, got %q", u.Scheme)
	}
	host, port := u.Hostname(), u.Port()
	if host == "" {
		return nil, fmt.Errorf("no host in %q", dsn)
	}
	if port == "" {
		port = "5432"
	}
	password, _ := u.User.Password()
	user := u.User.Username()
	if user == "" {
		user = defaultUser
	}
	return &Server{host: host, port: port, user: user, password: password}, nil
}

// reapLabelled removes servers whose owning test process is gone.
func reapLabelled() {
	out, err := docker("ps", "-a", "--no-trunc", "--filter", "label="+label,
		"--format", `{{.ID}} {{.Label "`+pidLabel+`"}}`)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(out, "\n") {
		id, pidStr, _ := strings.Cut(strings.TrimSpace(line), " ")
		if id == "" {
			continue
		}
		if pid, err := strconv.Atoi(pidStr); err == nil && processAlive(pid) {
			continue
		}
		_, _ = docker("rm", "-f", id)
	}
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func docker(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).Output()
	if err != nil {
		detail := ""
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			detail = ": " + strings.TrimSpace(string(ee.Stderr))
		}
		return "", fmt.Errorf("docker %s: %w%s", strings.Join(args, " "), err, detail)
	}
	return strings.TrimSpace(string(out)), nil
}
