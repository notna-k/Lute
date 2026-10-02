# Docker examples

Ready-to-run Docker Compose setups. Each directory stands alone: copy it anywhere, fill in its
`example.env` as `.env`, and run `docker compose up -d`.

| Example | What it runs | Use it to |
|---|---|---|
| [`quickstart/`](quickstart/) | Postgres, core, the panel and one worker on one machine | try Lute out |
| [`server/`](server/) | Postgres, core and the panel | run Lute for real |
| [`worker/`](worker/) | one worker on rootless Docker | add a build machine |

Core and the worker come from `ghcr.io/notna-k/lute-core` and `ghcr.io/notna-k/lute-worker`. The
panel is built from this repository on the first `docker compose up`, which takes a few minutes.

## quickstart

```bash
cd examples/quickstart
cp example.env .env      # set JWT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD
docker compose up -d
```

Open http://localhost:8080 and sign in as the admin. The worker enrols itself; under **Jobs**, run
`hello`. Its definition is [`quickstart/jobdefs/hello.yaml`](quickstart/jobdefs/hello.yaml): add
your own YAML next to it and press **Sync from Git**.

The worker runs jobs on the host's Docker, so it is started with `LUTE_ALLOW_ROOTFUL=1`. That is
fine on your laptop; on a shared machine use the server and worker examples instead.

`docker compose down` stops it, `docker compose down -v` also deletes the data.

## server

```bash
cd examples/server
cp example.env .env      # secrets, the panel's URL, where job definitions live
docker compose up -d
```

- `LUTE_URL` is the address people open the panel at, e.g. `https://lute.example.com`. The API
  rejects browser requests from any other origin.
- `JOB_DEFS_PATH` is the directory of job definitions. Point it at a checkout of your jobs
  repository; after a `git pull`, press **Sync from Git** in the panel.
- The panel listens on port 8080 and core's gRPC, for workers, on 50051. Neither speaks TLS, so
  put a TLS-terminating proxy in front of both. If you serve the panel over plain HTTP anyway, set
  `AUTH_COOKIE_SECURE=false` or sign-in does not stick.

Every setting core reads is listed in [Configuring core](https://notna-k.github.io/Lute/docs/configuration/).

## worker

On the build machine, set up rootless Docker for an unprivileged user first
([Running a worker](https://notna-k.github.io/Lute/docs/workers/running/), steps 1 and 2). Then,
as that user:

```bash
cd examples/worker
cp example.env .env      # LUTE_SERVER, and LUTE_TOKEN from Workers → Add worker
docker compose up -d
```

The worker keeps its identity and job logs in `~/.local/share/lute-worker`, so the token is needed
on the first start only. To update: `docker compose pull && docker compose up -d`.
