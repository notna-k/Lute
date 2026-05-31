# Data Model: Worker Labels & Job Routing

**Feature**: 001-worker-labels-routing
**Date**: 2026-05-30

## Changed Entities

### Worker (modified)

**Table**: `workers`
**Change**: Add `labels` JSON column (GORM AutoMigrate handles this automatically).

```go
// api/internal/db/models/worker.go — add this field
Labels map[string]string `json:"labels,omitempty" gorm:"serializer:json"`
```

| Field | Type | Constraints | Notes |
|-------|------|------------|-------|
| `labels` | `map[string]string` | Optional, max 32 pairs | Stored as JSON. Nil/empty = no labels (unlabelled worker). |

**Label key validation**:
- 1–63 characters
- Characters: `[a-zA-Z0-9_\-.]`
- Validated by `go-playground/validator` binding tag on the API handler DTO

**Label value validation**:
- 0–255 characters (empty string is valid)

**State transitions**: Labels are freely mutable via `PATCH /api/v1/workers/:id/labels`. The
full label map is replaced atomically (not merged) on each PATCH.

---

### Job (modified)

**Storage**: `queue_slots.payload` (JSON blob — no DDL change)
**Change**: Add `selector` field to the `Job` struct.

```go
// api/internal/queuejob/job.go — add this field
Selector map[string]string `json:"selector,omitempty"`
```

| Field | Type | Constraints | Notes |
|-------|------|------------|-------|
| `selector` | `map[string]string` | Optional, max 32 pairs, immutable post-enqueue | Nil/empty = route to any available worker (backward compatible). |

**Matching semantics**: For a worker to be eligible for a job, the worker's `Labels` map MUST
contain every key-value pair in `job.Selector`. Extra labels on the worker are allowed.

```
eligible = ∀ (k,v) ∈ selector : worker.labels[k] == v
```

---

### WorkerConnection (in-memory only)

**Location**: `api/internal/grpc/connection_manager.go`
**Change**: Add `Labels map[string]string` to `WorkerConnection`.

```go
type WorkerConnection struct {
    // ... existing fields ...
    Labels map[string]string  // loaded from DB at registration; updated via ConnMgr.UpdateWorkerLabels
}
```

This is NOT persisted — it mirrors the DB `workers.labels` value in memory for fast dispatch
matching. Updated atomically by `ConnMgr.UpdateWorkerLabels` when the REST API changes labels.

---

## New Repository Method

```go
// api/internal/db/repos/worker_repository.go

// GetByUserIDAndLabels returns workers owned by userID whose labels contain all selector pairs.
// An empty selector returns all workers for the user (same as GetByUserID).
func (r *WorkerRepository) GetByUserIDAndLabels(ctx context.Context, userID id.ID, filter map[string]string) ([]*models.Worker, error)
```

Used by the `GET /api/v1/workers?label=key:value` filter endpoint.

---

## No New Tables

This feature deliberately avoids adding a `worker_labels` join table. The JSON column approach
is consistent with the existing pattern used for `Worker.Metadata` and `Worker.Metrics`.
A join table would be appropriate if label-based queries needed index support — deferred to a
future migration if query performance becomes a concern at scale.
