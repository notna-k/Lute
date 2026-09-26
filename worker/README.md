# Lute worker

Go module `github.com/lute/worker`: the `lute-worker` agent that runs jobs for the Lute core.
It ships as a container image and runs jobs as sibling containers on the host's (rootless)
Docker. Host setup, running and updating: [`docs/worker.md`](../docs/worker.md).

## Layout

- **`cmd/worker/`** — entrypoint: startup checks, registration, signals, stopping its own container.
- **`internal/config`** — flags and `LUTE_*` variables.
- **`internal/datadir`** — the mounted data dir: marker, mount check, `jobs/<id>/{log,meta.json}`, retention.
- **`internal/state`** — `state.json`, the identity core returned from `Register`.
- **`internal/engine`** — the Docker `/info` probe (rootless, limits) and the agent's own container id.
- **`internal/agent`** — the authenticated gRPC stream, job dispatch, drain on SIGTERM and on delete.
- **`internal/runner`** — a `container` job: per-job network and volume, clone container, reaper.
- **`internal/joblog`** — paged reads of job logs.
- **`internal/metrics`** — host metrics sent with heartbeats.

## Build

```bash
make worker-build     # host binary, worker/bin/lute-worker
make worker-image     # linux/amd64 image, lute-worker:dev
```

Run the binary directly against rootful Docker for local work:

```bash
LUTE_SERVER=localhost:50051 LUTE_TOKEN=lute_rt_dev_bootstrap_token LUTE_ALLOW_ROOTFUL=1 \
  LUTE_DATA_DIR=./.worker-data ./bin/lute-worker run
```
