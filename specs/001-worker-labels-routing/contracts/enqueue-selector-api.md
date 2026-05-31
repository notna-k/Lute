# Contract: Job Selector on Enqueue & Run Submit

**Feature**: 001-worker-labels-routing

---

## POST /api/v1/jobs  (Dashboard — JWT auth)

Existing endpoint. Extended with optional `selector` field.

### Request Body

```json
{
  "queue": "default",
  "type": "container",
  "payload": { ... },
  "priority": 0,
  "delay_ms": 0,
  "max_retries": 3,
  "timeout_sec": 300,
  "selector": {
    "gpu": "true",
    "region": "us-east"
  }
}
```

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `selector` | `object` | No | Max 32 key-value pairs. Omit or `null` = route to any worker. |

**Key constraints**: 1–63 chars, pattern `[a-zA-Z0-9_\-.]`
**Value constraints**: 0–255 chars

### Success Response — `201 Created` (unchanged)

```json
{
  "job_id": "uuid-...",
  "status": "pending",
  "message": "Job enqueued"
}
```

### Error Responses (new)

| Status | Condition |
|--------|-----------|
| `400 Bad Request` | Selector key/value fails validation or > 32 pairs |

**Note**: A selector that no current worker satisfies is NOT a 400 error — the job is accepted
and waits for a matching worker.

---

## POST /api/public/v1/runs  (Public API — API Key auth)

Existing endpoint. Extended with optional `selector` field in the run payload.

### Request Body

```json
{
  "queue": "default",
  "type": "container",
  "payload": { ... },
  "webhook_url": "https://example.com/hook",
  "webhook_events": ["run.completed", "run.failed"],
  "idempotency_key": "optional-key",
  "selector": {
    "env": "prod"
  }
}
```

Same `selector` constraints as `/api/v1/jobs`.

### Success Response — `201 Created` (unchanged)

```json
{
  "run_id": "...",
  "job_id": "...",
  "status": "pending"
}
```

---

## GET /api/v1/jobs/:id  (Job detail — existing)

No change required. The `selector` field is already part of the job JSON payload stored in
`queue_slots` and will be returned naturally when the job envelope is deserialised and returned.

---

## Proto change: WorkerRegistration

```protobuf
// shared/proto/worker.proto
message WorkerRegistration {
  repeated string queues = 1;
  int32 concurrency = 2;
  map<string, string> labels = 3;  // NEW: populated by API server; worker binary sends empty
}
```

This field is populated server-side (loaded from the DB when processing the `Connect` stream),
not sent by the worker binary. It is reserved for future worker self-registration workflows.
