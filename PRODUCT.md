# Lute — Lightweight, code-first job orchestration

> Status: **direction doc**. Captures the decisions from the repurpose discussion.
> This supersedes the old "SaaS worker platform" framing in `AGENTS.md` (which stays
> accurate for the current code, but the target below is where we're going).

---

## 1. What we're building

A **lightweight Jenkins**: a server that runs **jobs** on **remote workers**, with a
rich admin panel and a first-class programmatic API — but without the legacy weight.

Positioning, in one line each:

- **vs Jenkins** — ships what normally needs a dozen plugins (typed inputs with a real
  UI, first-class worker integration, config-as-code) and drops the legacy (no Groovy
  pipelines, no controller filesystem as a database, no plugin sprawl).
- **vs Temporal** — dramatically simpler to adopt, and the admin panel is a real
  **control surface**, not just a monitoring dashboard. We explicitly **do not** attempt
  durable/replayable execution right now.

### Design pillars

1. **Stateless Core.** The Core container holds no durable state on its own filesystem.
   State lives in Postgres; logs and artifacts live in object storage.
2. **Code-first, GitOps.** Job definitions live in a Git repo. The repo is the source of
   truth; the panel reflects it.
3. **Rich typed inputs.** A parameter-schema system renders proper input UI (date
   pickers, custom/multi selects, etc.) *and* validates trigger payloads server-side.
   This is our clearest win over Jenkins.
4. **Programmatic by default.** Everything triggerable and observable via API + API keys,
   with idempotency and webhooks.
5. **Runs with `docker compose up`.** Core + Postgres + MinIO + Panel, plus workers you
   run wherever the work is.

### Non-goals (for now)

- No Groovy / no scripted pipelines.
- No durable/replayable execution (no event-sourced workflow engine).
- No DAG / no parallel stages. **One job = one command in one container.**
- No plugin marketplace.

---

## 2. Topology

```
                        ┌──────────────────┐
                        │   Admin Panel     │  (decoupled SPA, its own deployable)
                        │   React + Vite    │
                        └─────────┬────────┘
                                  │ HTTPS (public API + JWT)
                                  ▼
   ┌──────────┐  gRPC stream  ┌──────────────────┐   SQL    ┌────────────┐
   │ Worker   │◄─────────────►│      Core         │◄────────►│  Postgres  │
   │ (docker) │   assign/log   │  (stateless)      │          └────────────┘
   └──────────┘               │  HTTP + WS + gRPC │
   ┌──────────┐               │                   │   S3 API  ┌────────────┐
   │ Worker   │◄─────────────►│                   │◄─────────►│ MinIO / S3 │
   └──────────┘               └─────────┬────────┘           └────────────┘
                                        │ clone / sync
                                        ▼
                                ┌────────────────┐
                                │  Job-def Git    │  (source of truth for job config)
                                │  repo(s)        │
                                └────────────────┘
```

Deployables:

- **Core** — stateless Go service. HTTP API, WebSocket, gRPC worker endpoint. No embedded
  UI anymore.
- **Admin Panel** — standalone SPA build, served independently (its own container / static
  host). Talks to Core only through the public API. Configured with `CORE_API_URL`.
- **Workers** — unchanged model: connect to Core over gRPC, run container jobs.
- **Postgres** — durable domain state + job queue.
- **MinIO (or any S3-compatible)** — archived logs + build artifacts.

---

## 3. Changes from today's code

| Area | Today | Target |
|------|-------|--------|
| DB | SQLite embedded (`modernc.org/sqlite`) | **Postgres** (external dependency) |
| Panel | Embedded in API via `internal/ui` static embed | **Decoupled** SPA, separate deployable |
| Config | Ad-hoc runs, no reusable definition | **Job definitions** synced from **Git** |
| Log/artifact storage | Worker filesystem, streamed on demand | Live: streamed (unchanged). Archived: **object storage** |
| Core statefulness | SQLite file on disk | **Stateless** |

What we keep as-is (the hard parts already built):

- gRPC worker protocol (`WorkerService.Connect`): registration, heartbeats, job assign,
  drain/shutdown, log tailing (`JobLogRequest`/`JobLogResponse`).
- The queue engine: lanes, priority, delayed release, DLQ, per-minute stats.
- `ContainerJobSpec` (image + optional git repo + `request_params` → env + bash command).
- Auth: JWT + refresh-token families; API keys; public API surface; idempotency; webhooks.

---

## 4. Object model

### Job (new — the definition)

A reusable, versioned definition. **Sourced from Git**, projected into Postgres for
serving/indexing. Fields:

- `name`, `slug`, `description`
- **targeting**: queue name + worker label selector
- **spec**: a `ContainerJobSpec` template (image, optional `source_repository`, `command`)
- **parameters**: a **parameter schema** (see §5)
- **triggers**: manual / webhook / cron (cron reuses the existing scheduler)
- **source ref**: which repo + path + commit the definition came from (for GitOps drift)

### Run / Build

The existing `Run`, reframed as a **build**:

- gains optional `job_id` (`null` ⇒ ad-hoc run — this is the **hybrid** case)
- stores the **resolved parameter values** for the trigger
- per-job build history is simply `runs WHERE job_id = ?`

Keep: `queue`, `type`, `idempotency_key`, `webhook_*`, `user_id`/`api_key_id`.

### JobExecution

Worker-side result record. Keep `success`, `error`, `elapsed_ms`, `finished_at`.
**Add** object-storage keys: `archived_log_key`, `execution_log_key`, `artifact_keys[]`.

### Worker / Queue

Unchanged in shape. Workers register queues + concurrency + labels; queue keeps lanes /
priority / DLQ / stats.

---

## 5. Parameter schema — the crown jewel

A typed spec attached to a Job that **both** renders the input UI **and** validates the
trigger payload server-side. One definition, two consumers (panel + API). This is the
thing Jenkins never had.

Supported types (initial set):

| Type | UI | Notes |
|------|----|-------|
| `string` | text / textarea | `pattern`, `minLength`, `maxLength` |
| `number` | number input | `min`, `max`, `step` |
| `bool` | switch | |
| `select` | custom dropdown | `options[]`, static or from a data source |
| `multiselect` | chips / multi dropdown | `min`/`max` selected |
| `date` | date picker | ISO output |
| `datetime` | date + time picker | timezone-aware |
| `secret` | masked input | resolved from a secret store, never echoed |
| `file` | file upload | uploaded to object storage, passed as a URL/key |

Each param → an env var in the container (matching today's `request_params`). Sketch:

```yaml
parameters:
  - name: environment
    type: select
    label: Target environment
    required: true
    options: [dev, staging, prod]
    default: staging
  - name: release_date
    type: date
    label: Release date
  - name: dry_run
    type: bool
    default: true
```

Validation happens in Core on trigger — the API rejects a payload that doesn't satisfy the
schema, so programmatic callers get the same guarantees the UI does.

---

## 6. Config-as-code (GitOps)

**Git is the source of truth.**

- Job definitions are files (YAML) in a repo, one job per file (or a directory convention).
- Core **syncs** from the repo: on webhook push and/or on a poll interval, it reads the
  definitions and upserts the projected `Job` rows in Postgres.
- Definitions carry their source ref (repo + path + commit). The panel shows jobs synced
  from Git as **config-read-only** — you trigger and observe from the panel, but you edit
  config by changing the repo.
- Ad-hoc runs (no `job_id`) remain fully panel/API-driven — GitOps governs *definitions*,
  not one-off executions.

Canonical serialization rule: a Job's YAML is a pure projection of its fields. Nothing in a
definition may be expressible only outside the file — no hidden panel-only config.

---

## 7. Storage model

- **Live logs:** streamed from the worker over the existing gRPC `JobLogRequest`/`Response`
  path. Unchanged — good UX, already built.
- **On completion:** the worker uploads the final job log, execution log, and any declared
  artifacts to object storage. Core persists only the **keys** in Postgres.
- Behind a small `BlobStore` interface (S3-compatible). MinIO ships in compose; point it at
  real S3/R2/GCS in production.

Result: logs survive worker death, artifacts exist, and Core stays stateless.

---

## 8. Triggers

- **Manual** — panel or API, with a validated parameter payload.
- **Webhook** — inbound; also drives GitOps sync on push.
- **Cron** — reuses the existing scheduler.

Outbound webhooks (per-run `webhook_url` / events) already exist — keep them for
build-completion notifications.

---

## 9. First milestone (thin vertical slice)

1. **SQLite → Postgres** + compose service; make Core stateless.
2. **Decouple the panel** from the API embed; stand it up as its own deployable against the
   public API.
3. **Job definition** model + **parameter schema** format + server-side validation.
4. **GitOps sync**: read job defs from a repo, upsert `Job` rows.
5. **Panel**: view a Job, render its typed inputs, trigger a build → existing queue →
   existing gRPC worker runs the container.
6. **Object-storage archival** of the final log on completion.

Outcome: define a parameterized job in Git, trigger it from the panel with real typed
inputs, watch it run with live logs, and keep the log afterward — the smallest thing that
already feels like the product.

### Open questions to resolve during the slice

- **Secrets**: `secret`-typed params need a store (encrypted at rest) + resolution at
  trigger time. Decide whether it lands in this slice or the next.
- **Multi-repo GitOps**: one job-def repo to start, or many from day one?
- **Panel auth**: same JWT flow, now cross-origin — confirm CORS + token handling for the
  decoupled deployment.
