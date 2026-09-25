# Lute worker

Go module `github.com/lute/worker`: the `lute-worker` agent that runs jobs for the Lute core.

## Layout

- **`cmd/worker/`** — CLI entrypoint (`run`, `setup`, `logs`, `version`).
- **`internal/agent`** — gRPC connect/reconnect loop, job dispatch, heartbeats, job-log reads.
- **`internal/runner`** — runs a `container` job in Docker.
- **`internal/joblog`** — per-job log files and paged reads.
- **`internal/metrics`** — host metrics sent with heartbeats.
- **`internal/setup`** — `lute-worker setup`: registers the host and starts the agent in the background.

## Usage

```bash
lute-worker setup --api http://localhost:8080 --claim-code <CODE>   # register and start in the background
lute-worker run --server localhost:50051 --worker-id <ID>           # run a registered agent in the foreground
lute-worker logs -f                                                 # follow the background agent's log
```

## Build

`make worker-build` from the repository root, or `go build -o lute-worker ./cmd/worker` here.
