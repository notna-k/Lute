# Docker examples

Ready-to-run Docker Compose setups. Each directory stands alone: copy it anywhere, fill in its
`.env.example` as `.env`, and run `docker compose up -d`.

| Example | What it runs | Use it to |
|---|---|---|
| [`quickstart/`](quickstart/) | Postgres, core and one worker on one machine | try Lute out |
| [`server/`](server/) | Postgres and core | run Lute for real |
| [`worker/`](worker/) | one worker on rootless Docker | add a build machine |

The images come from the GitHub Container Registry: `ghcr.io/notna-k/lute-core`, which also serves
the panel, and `ghcr.io/notna-k/lute-worker`. The quickstart follows `latest`; the server and worker
examples pin a release, so an update is a deliberate tag bump.

## quickstart

```bash
cd examples/quickstart
cp .env.example .env     # set JWT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD
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
cp .env.example .env     # secrets, the panel's URL, where job definitions live
docker compose up -d
```

- `LUTE_URL` is the address people open the panel at, e.g. `https://lute.example.com`. The API
  rejects browser requests from any other origin.
- `JOB_DEFS_PATH` is the directory of job definitions. Point it at a checkout of your jobs
  repository; after a `git pull`, press **Sync from Git** in the panel.
- Core serves the panel and the API on port 8080, and gRPC for workers on 50051. Neither speaks TLS, so
  put a TLS-terminating proxy in front of both. If you serve the panel over plain HTTP anyway, set
  `AUTH_COOKIE_SECURE=false` or sign-in does not stick.

Every setting core reads is listed in [Configuring core](https://notna-k.github.io/Lute/docs/configuration/).

## worker

On the build machine, set up rootless Docker for an unprivileged user first
([Running a worker](https://notna-k.github.io/Lute/docs/workers/running/), steps 1 and 2). Then,
as that user:

```bash
cd examples/worker
cp .env.example .env     # LUTE_SERVER, and LUTE_TOKEN from Workers → Add worker
docker compose up -d
```

The worker keeps its identity and job logs in `~/.local/share/lute-worker`, so the token is needed
on the first start only. To update: `docker compose pull && docker compose up -d`.
