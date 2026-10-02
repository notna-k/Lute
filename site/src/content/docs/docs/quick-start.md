---
title: Quick start
description: Try Lute on one machine with Docker Compose.
---

You need Docker with Docker Compose. The quickstart runs everything on one machine from the latest
images on the GitHub Container Registry. It is one of the ready-to-run setups in the repository's
[`examples/`](https://github.com/notna-k/Lute/tree/master/examples) directory.

## Start the stack

Download the Compose file and the example job into a new directory:

```bash
mkdir -p lute/jobdefs && cd lute
base=https://raw.githubusercontent.com/notna-k/Lute/master/examples/quickstart
curl -fsSL -O "$base/compose.yaml" -o jobdefs/hello.yaml "$base/jobdefs/hello.yaml"
```

Create `.env` next to it with three values:

```bash
# JWT signing key, at least 32 bytes:  openssl rand -base64 48
JWT_SECRET=replace-me-with-a-long-random-string-at-least-32-bytes
# The first admin, created on first start if no user with this email exists.
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=change-me-on-first-login
```

Then start it:

```bash
docker compose up -d
```

That starts three containers: Postgres, core, which serves the panel, and one worker.

| What | Where |
|---|---|
| Panel | http://localhost:8080 — sign in with `ADMIN_EMAIL` / `ADMIN_PASSWORD` |
| gRPC for workers | `localhost:50051` |

## Run your first job

The stack syncs the job definitions in `jobdefs/`. Open **Jobs** in the panel,
pick `hello`, and press **Run**. The form comes from the job's [parameters](../jobs/parameters/);
the build log appears as the worker runs it.

To add your own, drop a YAML file next to `hello.yaml` and press **Sync from Git**. See
[Job definitions](../jobs/definitions/) for the format.

## Day to day

```bash
docker compose logs -f    # follow every container's logs
docker compose down       # stop
docker compose down -v    # stop and delete the data
```

The bundled worker runs jobs on this machine's Docker. For a real install, run the
[`server`](https://github.com/notna-k/Lute/tree/master/examples/server) example on one machine and
[add workers](../workers/running/) on your build machines.
