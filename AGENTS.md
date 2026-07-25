# AGENTS.md — guidance for coding agents
This document helps automated coding agents (and humans) work safely and consistently in **Lute**. Read it before making non-trivial changes.
---

## Purpose & product goal

**Lute** is a SaaS-style platform for operating **workers** (remote agents / machines), running **jobs** through a **queue**, and exposing dashboards and APIs.

Agents should preserve:

- Correct auth boundaries (JWT-authenticated users vs API keys vs public endpoints).
- Stability of worker ↔ API contracts (HTTP + WebSocket + gRPC where applicable).
- Clear separation between **API server**, **worker binary**, **shared protobuf**, and **web UI**.

---

## Repository map

```text
Lute/
├── api/                    # Go module github.com/lute/api — Core: HTTP API, WS, gRPC (panel is a separate container now)
│   ├── cmd/api/            # API entrypoint
│   └── internal/           # All implementation (handlers, repos, queue, middleware, ui embed)
├── worker/                 # Go module github.com/lute/worker — standalone worker CLI (Docker/git runners, heartbeat, metrics)
│   ├── cmd/worker/
│   └── internal/
├── shared/proto/           # Go module github.com/lute/proto — protobuf/gRPC definitions & generated code (do not hand-edit generated files)
├── ui/                     # React + Vite + TypeScript SPA (JWT auth against the API; TanStack Query)
├── infrastructure/dev/     # Docker Compose: three containers (postgres + core + admin); `jobdefs/` job-definition YAML
├── Makefile                # Primary automation (compose, worker builds, lint, release-ish targets)
├── .golangci.yml           # golangci-lint v2; excludes generated proto tree
├── .zed/settings.json      # Optional repo-local Zed (ESLint via `ui/`, format-on-save)
└── AGENTS.md               # This file
```

**Routing overview (API)** (`api/internal/router/router.go`):

- `/api` — health and WS (`GET /api/ws`).
- `/api/v1/auth/...` — login, refresh, logout, `me` (issues the JWTs everything else consumes).
- `/api/v1/...` — authenticated dashboard/worker UX APIs (JWT middleware on sensitive groups).
- `/api/public/v1/...` — public API surfaces (API-key oriented handlers).
- Static SPA registration via `internal/ui` after API routes.

---

## Toolchain versions

| Area | Expectation |
|------|-------------|
| Go | **1.26** (`go 1.26.0` in `api/go.mod`, `worker/go.mod`, `shared/proto/go.mod`) |
| Node | **25.x** — required for `ui/` (`engines`, `ui/.nvmrc`, repo `.nvmrc`); Docker UI stages use **`node:25-alpine`**. |
| PostgreSQL | **Primary/deployed DB** (`DB_DRIVER=postgres`, `POSTGRES_DSN`); persists domain data **and** the job queue. Compose runs it as its own container. |
| SQLite | **Optional** for quick native runs (`DB_DRIVER=sqlite`, `SQLITE_PATH`). Not used by the Compose stack. |

Agents must **not** silently downgrade language versions or swap major framework versions without explicit instruction.

---

## Local setup (human or agent validating changes)

### Full stack via Docker Compose

Documented in `infrastructure/dev/README.md`:

1. Copy `infrastructure/dev/.env.example` to **`.env` at the repo root** (the Makefile passes `--env-file $(CURDIR)/.env` to compose) and set `JWT_SECRET`, `ADMIN_EMAIL`, and `ADMIN_PASSWORD` as described there.
2. From repo root: `make dev-up` (requires Docker BuildKit; `DOCKER_BUILDKIT=1` is set in `Makefile`).
3. App + API: `http://localhost:8080`, health: `http://localhost:8080/api/health`.

### Native dev (no Compose)

Requires only **SQLite** (embedded in-process via `modernc.org/sqlite`):

- `SQLITE_PATH` default `lute.db` (working directory when running the API binary locally)
- `SQLITE_BUSY_TIMEOUT` default `5s`

Typical split:

- **UI**: `cd ui && npm ci && npm run dev` (Node **25** — `.nvmrc` at repo root and in `ui/`; Vite dev server port **3000** per `vite.config.ts`).
- **API**: run from `api/` with env vars matching deployment (JWT secret, seeded admin, SQLite path, worker binary dir).
- **Worker**: `make worker-build` → binary under `worker/bin/`; configure API’s `WORKER_BINARY_DIR` when serving binaries locally.

`JWT_SECRET` (≥ 32 bytes) plus `ADMIN_EMAIL`/`ADMIN_PASSWORD` are required for realistic auth flows — the admin user is seeded on first startup (see infra README).

---

## Commands agents should know

Run from repository root unless noted.

| Task | Command |
|------|---------|
| UI lint | `cd ui && npm run lint` |
| UI production build | `cd ui && npm run build` |
| Go lint (API + worker only) | `make go-lint` |
| Worker (current OS/arch) | `make worker-build` |
| Worker (all platforms) | `make worker-build-all` |
| Embed UI into API tree + build API binary | `make api-build` |
| Compose up | `make dev-up` |

**Do not** run linters against generated files under `shared/proto/` except via protobuf tooling — `.golangci.yml` excludes that path.

---

## Module boundaries & imports

- **`github.com/lute/api`**: Server only. Imports `github.com/lute/proto` as a normal module dependency (replace points at `shared/proto`).
- **`github.com/lute/worker`**: Worker only. Uses `replace github.com/lute/proto => ../shared/proto` in `worker/go.mod`.
- **`github.com/lute/proto`**: Shared messages/services. Regenerate with normal protobuf/grpc toolchain — **never invent manual edits** to generated `.pb.go` unless the task is explicitly code generation.

When adding cross-cutting types, prefer **proto** or **small shared packages** rather than importing `api` from `worker` or vice versa.

---

## Code style & practices — Go (`api/`, `worker/`)

1. **Packages**: Keep `internal/` private to each module; avoid creating import cycles.
2. **Errors**: Wrap with `%w` where appropriate; return meaningful domain errors at handlers’ boundary.
3. **Context**: Pass `context.Context` through I/O boundaries (DB, gRPC, HTTP handlers).
4. **Configuration**: Read via `api/internal/config` patterns (`getEnv`, typed structs), not scattered `os.Getenv` in random packages unless extending config deliberately.
5. **HTTP**: Gin routers grouped by domain (`internal/*/router.go`, `routes.go`). New endpoints belong next to their domain handler, wired through `internal/router/router.go`.
6. **Data access**: SQL repositories live under `api/internal/db/repos`; models under `api/internal/db/models`.
7. **Concurrency**: Respect existing queue (`internal/queue`), scheduler, heartbeat, and snapshot jobs — avoid blocking the HTTP goroutine with long synchronous work without a documented reason.
8. **Security**: Never log secrets, JWT signing keys, raw refresh tokens, or password hashes; validate auth middleware placement before adding authenticated routes.
9. **Performance**: Worker depends on heavy deps (e.g. Docker client); avoid redundant imports or rebuilding unrelated binaries in Docker layers unnecessarily.

Run **`make go-lint`** before declaring Go work complete.

---

## Code style & practices — TypeScript (`ui/`)

1. **Strict TS**: Project uses TypeScript + React 18 + Vite; follow existing patterns (`src/features`, `src/pages`, `src/components`, `src/hooks`, `src/services`).
2. **ESLint**: `npm run lint` must pass with **`--max-warnings 0`**.

   Config highlights (`ui/.eslintrc.cjs`):

   - `@typescript-eslint/quotes`: **single** strings (with `avoidEscape`, template literals allowed).
   - `jsx-quotes`: **prefer-single**.
   - React Hooks rules enforced.
   - `react-refresh/only-export-components` allows hooks named **`useAuth`** and **`useTheme`** alongside providers in context files.

3. **Formatting**: No Prettier in repo — formatting is driven by **ESLint `--fix`** and editor conventions; match surrounding files (2-space indent, single quotes).
4. **Imports**: Use `@/` path alias (`vite.config.ts`) for `src`.
5. **API calls**: Prefer centralized patterns in `src/services/api.ts` (and related hooks) rather than scattering raw `fetch` with duplicated base URLs.
6. **Env**: Vite exposes only `VITE_*` vars; remember embedded production build uses empty `VITE_API_URL` for same-origin API (`Makefile` `ui-build`).

---

## Protobuf & generated code

- Source `.proto` files and generation scripts live under **`shared/proto/`** (inspect layout before editing).
- **Agents must not** hand-edit generated Go protobuf/grpc outputs unless explicitly tasked with regeneration + verification.
- If `.proto` contracts change: regenerate stubs and ensure **`api`** and **`worker`** still compile and tests pass.

---

## Docker & CI-minded constraints

- **`api/Dockerfile`**: Multi-stage — UI build → Go API + cross-compiled worker binaries baked into image paths expected by runtime (`WORKER_BINARY_DIR`).
- **`Makefile`** pins **`golangci-lint`** via `go run ...@v2.11.4` for reproducibility.

Agents changing Dockerfile caching or Compose healthchecks should preserve:

- BuildKit cache mounts for npm and Go (`GOMODCACHE`, `GOCACHE`).
- Sensible Compose **`depends_on` / healthchecks** when optional backing services are added beside the API.

---

## Testing & verification checklist

Before claiming work is done:

1. **Go**: `make go-lint` clean for `api/` and `worker/`.
2. **UI**: `cd ui && npm run lint`; if types/build touched, `npm run build`.
3. **Integration**: When behaviour crosses worker/API/UI, describe how you validated it (manual steps or automated tests if added).

If adding tests:

- Place Go tests next to packages (`*_test.go`).
- Place UI tests only if project already uses a runner pattern — **do not** introduce Jest/Vitest without repo consensus.

---

## Editor / agent ergonomics

- **Zed**: `.zed/settings.json` pins ESLint working directory to `./ui` and enables format-on-save with ESLint fixes — agents editing TS/JS should respect ESLint rules by construction.
- **VS Code**: Minimal checked-in settings under `.vscode/` — do not assume everyone uses the same editor.

---

## What agents should avoid

1. **Drive-by refactors** unrelated to the task (especially cross-module renames).
2. **Committing secrets** (`.env`, service account JSON, production URIs). Use placeholders and reference `infrastructure/dev/README.md`.
3. **Editing generated protobuf outputs** without regeneration workflow.
4. **Breaking HTTP/WebSocket/gRPC contracts** without updating clients (`ui`, `worker`).
5. **Adding heavyweight dependencies** without justification (especially to `worker/` — compile time and binary size matter).

---

## Commit & PR narrative

Write commits and PR descriptions as:

- Full sentences explaining **what** and **why**.
- Scoped diffs; mention verification commands actually run (`make go-lint`, `npm run lint`, etc.).

---

## Further reading

- `infrastructure/dev/README.md` — Compose env vars and ports.
- `ui/README.md` — Frontend setup details.
- `worker/README.md` — Worker module layout.

---

*Maintain this file when architecture or tooling materially changes so agents stay aligned with human contributors.*
