# Research: Worker Labels & Job Routing

**Feature**: 001-worker-labels-routing
**Date**: 2026-05-30

## Decision Log

### 1. Label Storage: Dedicated Field vs. Reuse `Metadata`

**Decision**: Add a new `Labels map[string]string` field to the `Worker` model, stored as a
separate JSON column. Do not overload the existing `Metadata` column.

**Rationale**: `Metadata` already stores runtime registration data (hostname, OS, arch, CPUs) sent
by the worker binary during `--setup`. Mixing operator-managed labels into that field would make
the distinction between system-set and user-set data invisible at the API level and would require
filtering in every query. A dedicated field keeps the semantics clean and lets GORM AutoMigrate
handle the column addition automatically.

**Alternatives considered**:
- *Separate `worker_labels` join table*: cleaner relational model but adds a join to every worker
  fetch and a new repo. Rejected — labels are always fetched with the worker; JSON column is
  simpler and sufficient for ≤32 labels.
- *Reuse `Metadata`*: rejected — conflates system metadata with user labels.

---

### 2. Selector Storage: Job Payload JSON vs. New Column

**Decision**: Add `Selector map[string]string` directly to the `Job` struct in `queuejob/job.go`.
Because `queue_slots.payload` stores the full JSON-serialised `Job`, this requires no DDL change.
Existing rows deserialise `selector` as `nil`, preserving backward compatibility.

**Rationale**: The queue_slots table stores job data as a JSON blob by design. Adding a field to
the struct is zero-migration work and keeps the job envelope self-contained. Selector is immutable
post-enqueue — no update path needed.

**Alternatives considered**:
- *Separate `selector` column on `queue_slots`*: would enable index-assisted dispatch filtering
  in a future DB-side scheduling scenario. Rejected for v1 — dispatch is in-memory in the gRPC
  server, so a DB index adds no benefit now.

---

### 3. Dispatch Matching: Where to Enforce the Selector

**Decision**: Extend `ConnectionManager.FindAvailableWorker(queueName string)` to accept a second
parameter `selector map[string]string`. Each `WorkerConnection` carries a copy of the worker's
labels (loaded from DB at gRPC registration time). The matching loop adds a label-superset check
before accepting a worker as a candidate.

**Rationale**: The `DispatchJob` function in `grpc/server.go` already calls `FindAvailableWorker`.
The label data is in-memory on `WorkerConnection`, making matching O(n·m) where n = connected
workers and m = selector pairs — acceptable for the stated scale (≤500 workers, ≤32 labels).
No DB round-trip required per dispatch.

**Alternatives considered**:
- *DB-side matching (SQL WHERE clause)*: would require a custom SQLite JSON query; PostgreSQL
  supports JSON operators but SQLite does not. Rejected — in-memory is simpler and faster.
- *Worker announces labels in heartbeat*: would keep labels fresh in real-time but adds proto
  complexity. Rejected for v1 — labels are managed via the API and a connection reload suffices.

---

### 4. Label Freshness: How `WorkerConnection.Labels` Stays Current

**Decision**: Labels are loaded from the DB once at gRPC `Connect` registration (when the worker
sends a `WorkerRegistration` message). When a user updates labels via the REST API
(`PATCH /api/v1/workers/:id/labels`), the handler calls `ConnMgr.UpdateWorkerLabels(workerID,
newLabels)` which mutates the in-memory `WorkerConnection.Labels` and then calls `DispatchQueue`
for all queues the worker is subscribed to, triggering re-evaluation of pending selector jobs.

**Rationale**: Labels are changed infrequently (operator action, not automated). A single
mutation point (REST handler) is sufficient. No background sync needed.

**Alternatives considered**:
- *Worker sends labels in every heartbeat pong*: rejected — heartbeat is worker-driven, but label
  assignment is operator-driven from the API. The worker binary has no knowledge of labels.
- *Poll DB on every dispatch*: rejected — unnecessary DB hit on every job dispatch.

---

### 5. Proto Change: `WorkerRegistration.labels`

**Decision**: Add `map<string,string> labels = 3` to the `WorkerRegistration` proto message.
The worker binary does NOT send labels (it has no knowledge of them). The API server's gRPC
handler uses the `worker_id` from the first `Connect` message to fetch labels from the DB and
populate `WorkerConnection.Labels` directly.

**Rationale**: Labels are managed by the API user, not by the worker process. The worker binary
is a dumb compute node — it should not be responsible for knowing its own labels. The proto field
is added as documentation/future hook but the server-side DB load is authoritative for v1.

**Alternatives considered**:
- *Don't change the proto at all*: viable for v1 since labels are loaded server-side. Chosen to
  add the field anyway for forward compatibility — a future worker binary update could send labels
  for self-registration workflows.

---

### 6. "No Matching Worker" Handling

**Decision**: If `FindAvailableWorker` returns nil due to selector mismatch (no eligible
connected worker), `DispatchJob` returns `false` and the job stays in `QueueLaneReady`. No
immediate error is surfaced. The job will be attempted again on the next `DispatchQueue` call
(triggered by any worker connecting or a label update).

**Rationale**: Consistent with how the queue already handles "no available worker" (queue
saturation) — the job waits. This preserves the at-least-once delivery guarantee.

**Alternatives considered**:
- *Immediately dead-letter selector-mismatched jobs*: rejected per spec FR-006 — jobs must wait,
  not dead-letter silently.
- *Return a `202 Accepted` with a warning at enqueue time if no matching worker exists*: useful
  UX but adds a synchronous label-check at enqueue. Deferred to v2.
