# Developer Quickstart: Worker Labels & Job Routing

**Feature**: 001-worker-labels-routing
**Branch**: `001-worker-labels-routing`

## Local Setup

```bash
# Start the full stack
make dev-up

# Run Go linter (before any PR)
make go-lint

# Run UI linter
cd ui && npm run lint
```

## Verify the Feature End-to-End

### 1. Register two workers (use the UI or API)

Workers register via the worker binary `--setup` flow. For local testing,
two worker instances can be started against the same API.

### 2. Label one worker via the API

```bash
# Replace <worker-id> and <token> with real values
curl -X PATCH http://localhost:8080/api/v1/workers/<worker-id>/labels \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"labels": {"gpu": "true", "region": "us-east"}}'
```

Expected: `200 OK` with the updated worker object including `"labels": {"gpu":"true","region":"us-east"}`.

### 3. Enqueue a selector job

```bash
curl -X POST http://localhost:8080/api/v1/jobs \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "queue": "default",
    "type": "container",
    "payload": {"command": "echo hello"},
    "selector": {"gpu": "true"}
  }'
```

Expected: Job dispatched only to the labelled worker. The unlabelled worker receives no
assignment for this job.

### 4. Filter worker list

```bash
curl "http://localhost:8080/api/v1/workers?label=gpu:true" \
  -H "Authorization: Bearer <token>"
```

Expected: Only the labelled worker is returned.

### 5. Verify selector-miss behaviour

Enqueue a job with `"selector": {"gpu": "v100"}` (no worker has this label).

```bash
curl -X GET http://localhost:8080/api/v1/jobs/<job-id> \
  -H "Authorization: Bearer <token>"
```

Expected: Job status is `"pending"` — not dead-lettered.

## Key Files to Understand

| File | What it does |
|------|-------------|
| `api/internal/grpc/connection_manager.go` | `FindAvailableWorker` — label matching lives here |
| `api/internal/grpc/server.go` | `DispatchJob` — calls FindAvailableWorker with selector |
| `api/internal/db/models/worker.go` | `Labels` field definition |
| `api/internal/queuejob/job.go` | `Selector` field definition |
| `api/internal/worker/handler.go` | `PatchLabels` / `GetLabels` REST handlers |
| `ui/src/features/workers/LabelEditor.tsx` | React label editor component |

## Proto Regeneration

After editing `shared/proto/worker.proto`:

```bash
# From shared/proto/
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       worker.proto
```

Verify `api/` and `worker/` still compile after regen.
