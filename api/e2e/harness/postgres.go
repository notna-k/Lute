//go:build e2e

package harness

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for the admin connection
)

// postgresImage matches the version the dev stack and production run, so the suite
// exercises the same SQL dialect quirks.
const postgresImage = "postgres:17-alpine"

// postgresLabel marks the container the suite starts, so a later run can reap it.
const postgresLabel = "lute-e2e=postgres"

const (
	pgUser     = "lute"
	pgPassword = "lute"
	pgAdminDB  = "postgres"
)

// Postgres is the database server the suite runs against: either one the suite
// started in a container, or a pre-provisioned instance named by
// LUTE_E2E_POSTGRES_DSN (how CI supplies its service container).
type Postgres struct {
	host        string
	port        string
	user        string
	password    string
	containerID string
}

var dbSeq atomic.Uint64

// StartPostgres reuses LUTE_E2E_POSTGRES_DSN when set, otherwise runs a throwaway
// container on an ephemeral port. Call Stop when the suite finishes.
func StartPostgres() (*Postgres, error) {
	if dsn := strings.TrimSpace(os.Getenv("LUTE_E2E_POSTGRES_DSN")); dsn != "" {
		pg, err := parsePostgresDSN(dsn)
		if err != nil {
			return nil, fmt.Errorf("LUTE_E2E_POSTGRES_DSN: %w", err)
		}
		if err := pg.waitReady(60 * time.Second); err != nil {
			return nil, err
		}
		log.Printf("harness: using Postgres at %s:%s", pg.host, pg.port)
		return pg, nil
	}

	if !DockerAvailable() {
		return nil, fmt.Errorf("no Docker daemon and no LUTE_E2E_POSTGRES_DSN: cannot provide a database")
	}

	// A run that was interrupted before its cleanup leaves its server behind.
	ReapLabelled(postgresLabel)

	id, err := docker("run", "-d", "--rm",
		"--label", postgresLabel,
		"-e", "POSTGRES_USER="+pgUser,
		"-e", "POSTGRES_PASSWORD="+pgPassword,
		"-e", "POSTGRES_DB="+pgAdminDB,
		// fsync off: this database is thrown away, and migrations per test dominate the runtime.
		"-p", "127.0.0.1:0:5432", postgresImage,
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

	pg := &Postgres{host: host, port: port, user: pgUser, password: pgPassword, containerID: id}
	if err := pg.waitReady(90 * time.Second); err != nil {
		pg.Stop()
		return nil, err
	}
	log.Printf("harness: started %s on %s:%s (container %s)", postgresImage, host, port, id[:12])
	return pg, nil
}

// Stop removes the container the suite started. A reused instance is left alone.
func (p *Postgres) Stop() {
	if p == nil || p.containerID == "" {
		return
	}
	if _, err := docker("rm", "-f", p.containerID); err != nil {
		log.Printf("harness: removing postgres container: %v", err)
	}
	p.containerID = ""
}

// CreateDatabase makes a database of its own for one stack and drops it on cleanup,
// so tests cannot see each other's rows even when they share a server.
func (p *Postgres) CreateDatabase(t *testing.T) string {
	t.Helper()

	name := fmt.Sprintf("lute_e2e_%d_%d", time.Now().UnixNano()%1e9, dbSeq.Add(1))
	admin := p.open(t, pgAdminDB)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	t.Cleanup(func() {
		// Terminate stragglers first: a leaked connection makes DROP fail and the
		// next run inherits the database.
		_, _ = admin.Exec(
			"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
		if _, err := admin.Exec("DROP DATABASE IF EXISTS " + name); err != nil {
			t.Logf("drop database %s: %v", name, err)
		}
		_ = admin.Close()
	})
	return p.dsn(name)
}

func (p *Postgres) dsn(database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		p.user, p.password, p.host, p.port, database)
}

func (p *Postgres) open(t *testing.T, database string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", p.dsn(database))
	if err != nil {
		t.Fatalf("open %s: %v", database, err)
	}
	db.SetMaxOpenConns(2)
	return db
}

func (p *Postgres) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("pgx", p.dsn(pgAdminDB))
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
	return fmt.Errorf("postgres at %s:%s not ready within %s: %w", p.host, p.port, timeout, lastErr)
}

// splitMapping turns `docker port`'s output into host and port. The command can
// print one line per address family; the first usable line wins.
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

// parsePostgresDSN reads the server coordinates out of a URL-style DSN. The
// database it names is ignored: every stack creates its own.
func parsePostgresDSN(dsn string) (*Postgres, error) {
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
		user = pgUser
	}
	return &Postgres{host: host, port: port, user: user, password: password}, nil
}
