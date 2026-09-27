// Package config reads the agent's settings: a flag overrides its LUTE_* variable, which
// overrides the default. The server address may also come from state.json.
package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
)

const DefaultDataDir = "/var/lib/lute-worker"

type Config struct {
	Server       string
	TLS          bool
	Token        string
	Name         string
	Queues       []string
	Labels       map[string]string
	Concurrency  int
	DataDir      string
	RequireMount bool
	AllowRootful bool
	DrainTimeout time.Duration
	LogRetention time.Duration
	GitImage     string
	// JobMemory is in bytes and JobNanoCPUs in billionths of a CPU; zero means no limit.
	JobMemory   int64
	JobNanoCPUs int64
}

// DefaultGitImage clones repositories; pinned so a new git release cannot change a build.
const DefaultGitImage = "alpine/git:2.52.0"

// Load parses the run command's flags, with defaults taken from getenv.
func Load(args []string, getenv func(string) string, usage io.Writer) (*Config, error) {
	env := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(usage)
	server := fs.String("server", env("LUTE_SERVER", ""), "core gRPC address, host:port [LUTE_SERVER]")
	useTLS := fs.String("tls", env("LUTE_TLS", "0"), "dial core with TLS, e.g. behind a TLS proxy [LUTE_TLS]")
	token := fs.String("token", env("LUTE_TOKEN", ""), "registration token, used on the first start only [LUTE_TOKEN]")
	name := fs.String("name", env("LUTE_NAME", ""), "worker name, unique across Lute; defaults to the engine's host name [LUTE_NAME]")
	queues := fs.String("queues", env("LUTE_QUEUES", "default"), "comma-separated queues [LUTE_QUEUES]")
	labels := fs.String("labels", env("LUTE_LABELS", ""), "k=v,k=v labels sent at registration [LUTE_LABELS]")
	concurrency := fs.String("concurrency", env("LUTE_CONCURRENCY", "4"), "jobs at once [LUTE_CONCURRENCY]")
	dataDir := fs.String("data-dir", env("LUTE_DATA_DIR", DefaultDataDir), "identity, job logs and job metadata [LUTE_DATA_DIR]")
	requireMount := fs.String("require-mount", env("LUTE_REQUIRE_MOUNT", "0"), "exit unless the data dir is a mount point [LUTE_REQUIRE_MOUNT]")
	allowRootful := fs.String("allow-rootful", env("LUTE_ALLOW_ROOTFUL", "0"), "allow a rootful engine, for dev and CI [LUTE_ALLOW_ROOTFUL]")
	drainTimeout := fs.String("drain-timeout", env("LUTE_DRAIN_TIMEOUT", "30m"), "how long SIGTERM waits for running jobs [LUTE_DRAIN_TIMEOUT]")
	retention := fs.String("log-retention", env("LUTE_LOG_RETENTION", "720h"), "age at which job logs are pruned [LUTE_LOG_RETENTION]")
	gitImage := fs.String("git-image", env("LUTE_GIT_IMAGE", DefaultGitImage), "image used to clone repositories [LUTE_GIT_IMAGE]")
	memory := fs.String("job-memory", env("LUTE_JOB_MEMORY", ""), "memory limit per job, e.g. 2g [LUTE_JOB_MEMORY]")
	cpus := fs.String("job-cpus", env("LUTE_JOB_CPUS", ""), "CPU limit per job, e.g. 1.5 [LUTE_JOB_CPUS]")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	c := &Config{
		Server:   *server,
		Token:    *token,
		Name:     *name,
		Queues:   splitList(*queues),
		DataDir:  *dataDir,
		GitImage: *gitImage,
	}
	var errs []error
	var err error
	if c.Labels, err = parseLabels(*labels); err != nil {
		errs = append(errs, err)
	}
	if c.Concurrency, err = strconv.Atoi(*concurrency); err != nil || c.Concurrency < 1 || c.Concurrency > 1024 {
		errs = append(errs, fmt.Errorf("concurrency %q: want a number from 1 to 1024", *concurrency))
	}
	if c.TLS, err = strconv.ParseBool(*useTLS); err != nil {
		errs = append(errs, fmt.Errorf("tls %q: %w", *useTLS, err))
	}
	if c.RequireMount, err = strconv.ParseBool(*requireMount); err != nil {
		errs = append(errs, fmt.Errorf("require-mount %q: %w", *requireMount, err))
	}
	if c.AllowRootful, err = strconv.ParseBool(*allowRootful); err != nil {
		errs = append(errs, fmt.Errorf("allow-rootful %q: %w", *allowRootful, err))
	}
	if c.DrainTimeout, err = time.ParseDuration(*drainTimeout); err != nil {
		errs = append(errs, fmt.Errorf("drain-timeout: %w", err))
	}
	if c.LogRetention, err = time.ParseDuration(*retention); err != nil {
		errs = append(errs, fmt.Errorf("log-retention: %w", err))
	}
	if *memory != "" {
		if c.JobMemory, err = units.RAMInBytes(*memory); err != nil || c.JobMemory <= 0 {
			errs = append(errs, fmt.Errorf("job-memory %q: want a size such as 512m or 2g", *memory))
		}
	}
	if *cpus != "" {
		f, err := strconv.ParseFloat(*cpus, 64)
		if err != nil || f <= 0 {
			errs = append(errs, fmt.Errorf("job-cpus %q: want a positive number such as 1.5", *cpus))
		}
		c.JobNanoCPUs = int64(f * 1e9)
	}
	if len(c.Queues) == 0 {
		errs = append(errs, errors.New("queues: at least one queue is required"))
	}
	if c.DataDir == "" {
		errs = append(errs, errors.New("data-dir is required"))
	}
	return c, errors.Join(errs...)
}

// LimitsConfigured reports whether jobs must run with memory or CPU limits.
func (c *Config) LimitsConfigured() bool {
	return c.JobMemory > 0 || c.JobNanoCPUs > 0
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseLabels(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, pair := range splitList(s) {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("labels: %q is not k=v", pair)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}
