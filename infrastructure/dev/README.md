# Development Docker Compose Setup

This directory runs the local Lute stack as **three separate containers**.

## Services

- **postgres** — PostgreSQL 17. Durable domain data **and** the job queue/state.
  Data lives in the `postgres_data` volume.
- **core** — the stateless Go backend (REST, WebSocket, gRPC). No embedded UI.
  Connects to Postgres via `POSTGRES_DSN` and syncs Git-managed **job definitions**
  from `JOB_DEFS_DIR` (mounted read-only from `./jobdefs`) on startup.
- **admin** — the decoupled React panel served by nginx. Talks to Core through
  the same origin: nginx proxies `/api` (and the `/api/ws` WebSocket) to `core`.

```
browser ──▶ admin (nginx :8080) ──/api──▶ core (:8080) ──▶ postgres (:5432)
worker  ─────────────────── gRPC ────────▶ core (:50051)
```

## Quick Start

1. **Create `.env`** in this directory:
   ```bash
   cd infrastructure/dev
   cp .env.example .env
   ```

2. **Configure required values** in `.env`:
   ```bash
   # JWT signing key. MUST be >= 32 bytes:  openssl rand -base64 48
   JWT_SECRET=replace-me-with-a-long-random-string-at-least-32-bytes
   # Bootstrap admin, seeded on first startup if no user with this email exists.
   ADMIN_EMAIL=admin@example.com
   ADMIN_PASSWORD=change-me-on-first-login
   ```
   Postgres credentials (`POSTGRES_USER/PASSWORD/DB`) default to `lute`/`lute`/`lute`;
   override them in `.env` for anything shared. Optional auth tuning is documented
   in `.env.example`.

3. **Start** (from project root):
   ```bash
   make dev-up          # or: cd infrastructure/dev && docker compose up -d --build
   ```

4. **Logs / stop**:
   ```bash
   make dev-logs
   make dev-down        # stop
   make dev-clean       # stop + remove volumes (wipes Postgres)
   ```

## Access Points

- **Admin panel**: http://localhost:8080
- **Core API (direct)**: http://localhost:8081 — e.g. health at
  http://localhost:8081/api/health
- **gRPC (workers)**: localhost:50051

## Job definitions (GitOps)

Job definitions are YAML files under `./jobdefs` (the Git source of truth). Core
reads them on startup and upserts them into Postgres; the admin panel shows them
as config-read-only. Edit a file and restart `core` (or re-run the stack) to
re-sync. See `jobdefs/web-release.yaml` for the format.

## Environment Variables

Configured in `.env` (copy from `.env.example`). Highlights:

- **Database**: Core runs with `DB_DRIVER=postgres` and a `POSTGRES_DSN` pointing
  at the `postgres` service (set by compose). SQLite remains available for native
  runs via `DB_DRIVER=sqlite`.
- **Ports**: `ADMIN_PORT` (8080), `API_HTTP_PORT` (8081), `API_GRPC_PORT` (50051).
- **Auth**: `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, plus optional token TTLs.

### Vite build-time variables

`VITE_*` vars are baked at **build time** into the **admin** image (`ui/Dockerfile`).
`VITE_API_URL` is left empty so the browser uses the same origin and nginx proxies
to Core.

## Rebuilding

```bash
docker compose build core     # backend only
docker compose build admin    # panel only
docker compose up -d --build  # everything
```

## Volumes

- `postgres_data`: PostgreSQL data directory.
