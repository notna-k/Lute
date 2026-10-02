---
title: Quick start
description: Try Lute on one machine with Docker Compose.
---

You need Docker with Docker Compose. The repository's
[`examples/`](https://github.com/notna-k/Lute/tree/master/examples) directory has ready-to-run
setups; this page uses the quickstart one, which runs everything on one machine.

## Start the stack

```bash
git clone https://github.com/notna-k/Lute.git
cd Lute/examples/quickstart
cp example.env .env
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
docker compose up -d
```

That starts four containers: Postgres, core, the panel and one worker. The panel is built from the
repository on the first start, which takes a few minutes.

| What | Where |
|---|---|
| Panel | http://localhost:8080 — sign in with `ADMIN_EMAIL` / `ADMIN_PASSWORD` |
| gRPC for workers | `localhost:50051` |

## Run your first job

The stack syncs the job definitions in `examples/quickstart/jobdefs/`. Open **Jobs** in the panel,
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
