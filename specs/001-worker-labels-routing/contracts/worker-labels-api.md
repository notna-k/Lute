# Contract: Worker Labels API

**Feature**: 001-worker-labels-routing
**Auth**: JWT (all endpoints — `Authorization: Bearer <token>` or `auth_token` cookie)

---

## GET /api/v1/workers

Existing endpoint. Extended with optional label filter query parameter.

### Query Parameters (new)

| Parameter | Format | Example | Notes |
|-----------|--------|---------|-------|
| `label` | `key:value` | `?label=gpu:true` | Repeatable. Multiple = AND. Omit = no filter. |

### Response (unchanged shape, `labels` field added)

```json
[
  {
    "id": "abc123",
    "name": "gpu-worker-01",
    "status": "alive",
    "labels": { "gpu": "true", "region": "us-east" },
    "agent_ip": "10.0.0.5",
    "last_seen": "2026-05-30T12:00:00Z",
    "metadata": { ... },
    "metrics": { ... },
    "created_at": "2026-05-01T00:00:00Z",
    "updated_at": "2026-05-30T12:00:00Z"
  }
]
```

**Note**: `labels` is `null` or absent for workers with no labels.

---

## GET /api/v1/workers/:id

Existing endpoint. Extended to include `labels` in response (same shape as list above).

---

## PATCH /api/v1/workers/:id/labels

**New endpoint.** Atomically replaces the full label set for a worker.

### Request Body

```json
{
  "labels": {
    "gpu": "true",
    "region": "us-east",
    "env": "prod"
  }
}
```

| Field | Type | Required | Constraints |
|-------|------|----------|-------------|
| `labels` | `object` | Yes | Max 32 key-value pairs. Pass `{}` to clear all labels. |

**Key constraints**: 1–63 chars, pattern `[a-zA-Z0-9_\-.]`
**Value constraints**: 0–255 chars

### Success Response — `200 OK`

```json
{
  "id": "abc123",
  "name": "gpu-worker-01",
  "labels": { "gpu": "true", "region": "us-east", "env": "prod" },
  "status": "alive",
  ...
}
```

### Error Responses

| Status | Condition |
|--------|-----------|
| `400 Bad Request` | Invalid key format, value too long, or > 32 pairs |
| `401 Unauthorized` | Missing/invalid JWT |
| `403 Forbidden` | Worker belongs to a different user |
| `404 Not Found` | Worker ID not found |

### Side Effects

After a successful label update:
1. The in-memory `WorkerConnection.Labels` is updated atomically (if the worker is connected).
2. `DispatchQueue` is called for each queue the worker subscribes to, re-evaluating pending
   selector jobs.
