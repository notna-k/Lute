---
title: Quick start
description: Run the whole stack locally with Docker Compose.
---

You need Docker with Docker Compose. Go 1.26+ and Node 25 are only needed to build outside Docker.

## Start the stack

```bash
git clone https://github.com/notna-k/Lute.git && cd Lute
cp .env.example .env
```

Set three values in `.env`:

```bash
# JWT signing key, at least 32 bytes:  openssl rand -base64 48
JWT_SECRET=replace-me-with-a-long-random-string-at-least-32-bytes
# The first admin, created on first start if no user with this email exists.
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me-on-first-login
```

Then start it:

```bash
make dev-up
```

That builds and starts four containers: Postgres, core, the panel and one worker.

| What | Where |
|---|---|
| Panel | http://localhost:8080 — sign in with `ADMIN_EMAIL` / `ADMIN_PASSWORD` |
| Core API | http://localhost:8081, health at `/api/health` |
| gRPC for workers | `localhost:50051` |

## Run your first job

The stack syncs the example job definitions in `infrastructure/dev/jobdefs/`. Open **Jobs** in the
panel, pick `web-release`, and press **Run**. The form comes from the job's
[parameters](../jobs/parameters/); the build log appears as the worker runs it.

To add your own, drop a YAML file next to the examples and press **Sync from Git**. See
[Job definitions](../jobs/definitions/) for the format.

## Day to day

```bash
make dev-logs    # follow every container's logs
make dev-down    # stop
make dev-clean   # stop and wipe Postgres and the worker's identity
```

The bundled worker enrols itself with `WORKER_BOOTSTRAP_TOKEN`. To add a real build machine, see
[Running a worker](../workers/running/).
