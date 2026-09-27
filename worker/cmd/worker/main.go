package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/lute/proto"

	"github.com/lute/worker/internal/agent"
	"github.com/lute/worker/internal/config"
	"github.com/lute/worker/internal/datadir"
	"github.com/lute/worker/internal/engine"
	"github.com/lute/worker/internal/runner"
	"github.com/lute/worker/internal/state"
)

// Set at build time via -ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// exitConfig (EX_CONFIG) means a person has to fix something; Docker's restart backoff
// keeps retrying meanwhile, and `docker logs` shows why.
const exitConfig = 78

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cmd, args := "run", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "run":
		os.Exit(run(args))
	case "version", "--version", "-v":
		fmt.Printf("lute-worker %s (built %s)\n", Version, BuildTime)
	case "help", "--help", "-h":
		printUsage(os.Stdout)
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown command: %q\n\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintf(w, `lute-worker %s — Lute build agent

Usage:
  lute-worker run [flags]   connect to core and run jobs (the default)
  lute-worker version
  lute-worker help

Every flag also reads a LUTE_* variable; see "lute-worker run -h".
It is meant to run as a container on rootless Docker:

  docker run -d --name lute-worker --restart unless-stopped --stop-timeout 1800 \
    -v "$XDG_RUNTIME_DIR/docker.sock:/var/run/docker.sock" \
    -v "$HOME/.local/share/lute-worker:/var/lib/lute-worker" \
    -e LUTE_SERVER=lute.example.com:50051 -e LUTE_TOKEN=lute_rt_... \
    ghcr.io/notna-k/lute-worker
`, Version)
}

func fail(code int, format string, args ...any) int {
	_, _ = fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	return code
}

func run(args []string) int {
	cfg, err := config.Load(args, os.Getenv, os.Stderr)
	if err != nil {
		return fail(exitConfig, "%v", err)
	}
	slog.Info("Lute worker starting", "version", Version, "build", BuildTime)

	data, err := datadir.Open(cfg.DataDir, cfg.RequireMount)
	if err != nil {
		return fail(exitConfig, "%v", err)
	}

	// Jobs get their own context: the first SIGTERM drains, only a second one cancels them.
	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	var running atomic.Pointer[agent.Agent]
	go func() {
		<-sigs
		a := running.Load()
		if a == nil {
			cancel() // nothing runs yet: stop starting up
			return
		}
		slog.Info("Signal received: draining; a second signal cancels running jobs", "timeout", cfg.DrainTimeout)
		a.Drain()
		select {
		case <-sigs:
			slog.Warn("Second signal: cancelling running jobs")
		case <-time.After(cfg.DrainTimeout):
			slog.Warn("Drain timeout: cancelling running jobs")
		}
		cancelJobs()
	}()

	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fail(1, "docker client: %v", err)
	}
	defer func() { _ = docker.Close() }()
	eng, err := engine.Probe(ctx, docker)
	if err != nil {
		return fail(1, "cannot reach the Docker engine at %s (is its socket mounted?): %v", docker.DaemonHost(), err)
	}
	if !eng.Rootless && !cfg.AllowRootful {
		return fail(exitConfig, "the Docker engine is rootful: access to its socket is root on the host. Run the worker on rootless Docker, or set LUTE_ALLOW_ROOTFUL=1 for dev and CI")
	}
	if missing := eng.Missing(); len(missing) > 0 {
		if cfg.LimitsConfigured() {
			return fail(exitConfig, "job limits are configured but the engine cannot enforce %s limits; delegate cgroup v2 controllers to the user (see docs/worker.md)", strings.Join(missing, ", "))
		}
		slog.Warn("The engine cannot enforce some resource limits; jobs run without them", "missing", missing)
	}

	st, err := state.Load(data.StatePath())
	if err != nil {
		return fail(exitConfig, "%v", err)
	}
	if cfg.Server == "" && st != nil {
		cfg.Server = st.Server
	}
	if cfg.Server == "" {
		return fail(exitConfig, "LUTE_SERVER is required (core's gRPC address, host:port)")
	}
	if st == nil {
		if st, err = register(ctx, cfg, eng, data); err != nil {
			if ctx.Err() != nil {
				return 0
			}
			return fail(exitConfig, "register: %v", err)
		}
	}
	slog.Info("Worker identity", "worker_id", st.WorkerID, "server", cfg.Server, "engine", eng.Version, "rootless", eng.Rootless)

	self := selfContainer(ctx, docker)
	runner.Reap(ctx, docker, st.WorkerID)
	go pruneLoop(ctx, data, cfg.LogRetention)

	limits := runner.Limits{Memory: cfg.JobMemory, NanoCPUs: cfg.JobNanoCPUs}
	if eng.PidsLimit {
		limits.Pids = 1024
	}
	a := agent.New(agent.Config{
		ServerAddr:  cfg.Server,
		TLS:         cfg.TLS,
		WorkerID:    st.WorkerID,
		Secret:      st.Secret,
		Queues:      cfg.Queues,
		Concurrency: int32(cfg.Concurrency),
		Version:     Version,
		Engine:      eng.Proto(),
		Jobs: &agent.Jobs{
			DataDir: data.Root,
			Runner: &runner.Runner{
				Docker:   docker,
				Data:     data,
				WorkerID: st.WorkerID,
				GitImage: cfg.GitImage,
				Limits:   limits,
			},
		},
	}, jobCtx)
	running.Store(a)

	outcome, err := a.Run(ctx)
	if err != nil {
		return fail(exitConfig, "core refused the worker: %v", status.Convert(err).Message())
	}
	switch outcome {
	case agent.Deleted:
		// A worker deleted while offline may still run jobs nobody will collect; stop them and
		// let them clean up, since a stopped container is never restarted to reap them.
		cancelJobs()
		a.WaitJobs()
		if self != "" {
			stopSelf(docker, self)
		}
		slog.Info("Worker deleted; exiting")
	case agent.Drained:
		slog.Info("Drained; exiting")
	}
	return 0
}

// register enrols the worker with LUTE_TOKEN and saves the identity core returns.
func register(ctx context.Context, cfg *config.Config, eng engine.Info, data *datadir.Dir) (*state.State, error) {
	if cfg.Token == "" {
		return nil, errors.New("this worker is not registered yet: set LUTE_TOKEN to a registration token from the panel")
	}
	name := cfg.Name
	if name == "" {
		name = eng.Name
	}
	resp, err := agent.Register(ctx, cfg.Server, cfg.TLS, &pb.RegisterRequest{
		Token:    cfg.Token,
		Name:     name,
		Version:  Version,
		Protocol: pb.Protocol,
		Engine:   eng.Proto(),
		Queues:   cfg.Queues,
		Labels:   cfg.Labels,
	})
	if status.Code(err) == codes.AlreadyExists {
		return nil, fmt.Errorf("%s; set LUTE_NAME", status.Convert(err).Message())
	}
	if err != nil {
		return nil, err
	}
	st := &state.State{WorkerID: resp.WorkerId, Secret: resp.Secret, Server: cfg.Server}
	if err := state.Save(data.StatePath(), st); err != nil {
		return nil, fmt.Errorf("save identity: %w", err)
	}
	slog.Info("Registered", "worker_id", st.WorkerID, "name", name)
	return st, nil
}

// selfContainer is the agent's own container id, or "" when it runs as a plain binary.
func selfContainer(ctx context.Context, docker client.APIClient) string {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return ""
	}
	id := engine.ContainerIDFromMountinfo(f)
	_ = f.Close()
	if id == "" {
		return ""
	}
	if _, err := docker.ContainerInspect(ctx, id); err != nil {
		slog.Warn("Cannot find the agent's own container on the engine; a deleted worker will exit instead of stopping", "id", id[:12], "err", err)
		return ""
	}
	return id
}

// stopSelf stops the agent's container through the API: under "unless-stopped" only that
// keeps it down, even across a daemon restart. The stop's SIGTERM ends this process.
func stopSelf(docker client.APIClient, id string) {
	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGTERM)
	go func() {
		<-term
		os.Exit(0)
	}()
	slog.Info("Stopping own container", "id", id[:12])
	if err := docker.ContainerStop(context.Background(), id, container.StopOptions{}); err != nil {
		slog.Error("Stop own container", "err", err)
	}
}

func pruneLoop(ctx context.Context, data *datadir.Dir, retention time.Duration) {
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for {
		if n, err := data.Prune(retention, time.Now()); err != nil {
			slog.Warn("Prune job logs", "err", err)
		} else if n > 0 {
			slog.Info("Pruned old job logs", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
