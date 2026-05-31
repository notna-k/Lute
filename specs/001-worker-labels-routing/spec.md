# Feature Specification: Worker Labels & Job Routing

**Feature Branch**: `001-worker-labels-routing`

**Created**: 2026-05-30

**Status**: Draft

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Tag Workers with Labels (Priority: P1)

As a platform operator, I want to attach key-value labels to my registered workers so that I can
categorise them by capability, region, or environment.

**Why this priority**: Labels are the prerequisite for all routing. Without this, job routing cannot
exist. It is also independently useful for display and filtering in the dashboard.

**Independent Test**: Register two workers, assign labels to each, then retrieve each worker's
details and verify the labels are saved and returned correctly.

**Acceptance Scenarios**:

1. **Given** a registered worker, **When** I attach labels `{gpu: "true", region: "us-east"}`,
   **Then** the worker record reflects those labels immediately.
2. **Given** a worker with existing labels, **When** I update the label set,
   **Then** the old labels are replaced by the new set.
3. **Given** a worker with labels, **When** I remove all labels,
   **Then** the worker record shows an empty label set.
4. **Given** a label key longer than 63 characters or containing invalid characters,
   **When** I attempt to save it, **Then** the system rejects the request with a clear error.

---

### User Story 2 - Declare Job Routing Selectors (Priority: P2)

As an API consumer, I want to attach a label selector to a job when I enqueue it so that the job
is only dispatched to workers whose labels satisfy that selector.

**Why this priority**: Selector declaration is the second half of the routing contract. A job with
a selector but no matching worker must not be silently lost — it must wait or fail fast.

**Independent Test**: Enqueue a job with selector `{gpu: "true"}` and two workers where only one
has that label. Verify the job is dispatched exclusively to the matching worker.

**Acceptance Scenarios**:

1. **Given** a job with selector `{region: "eu-west"}` and a worker labelled `{region: "eu-west"}`,
   **When** the job is dispatched, **Then** the matching worker receives it.
2. **Given** a job with selector `{gpu: "true"}` and no worker with that label,
   **When** dispatch is attempted, **Then** the job remains pending (not dead-lettered) and is
   retried when a matching worker becomes available or connects.
3. **Given** a job with no selector, **When** dispatched,
   **Then** any available worker may receive it (existing behaviour unchanged).
4. **Given** a job with selector `{env: "prod", region: "us-east"}`,
   **When** dispatched, **Then** only workers carrying BOTH labels are eligible.

---

### User Story 3 - Filter Workers by Label in Dashboard (Priority: P3)

As a dashboard user, I want to filter the worker list by one or more labels so that I can quickly
find workers with a specific capability without scrolling through all registered workers.

**Why this priority**: Operators with many workers need label-based search. This delivers user
value without blocking the core routing feature.

**Independent Test**: Register five workers with varying labels. Apply a label filter in the
dashboard. Verify only matching workers are shown.

**Acceptance Scenarios**:

1. **Given** workers with varying labels, **When** I filter by `region=us-east`,
   **Then** only workers carrying that label are displayed.
2. **Given** an active label filter, **When** all matching workers go offline,
   **Then** the list shows empty state (not an error).
3. **Given** a label filter, **When** I clear it, **Then** all workers are shown again.

---

### Edge Cases

- A worker goes offline while a job with a matching selector is already dispatched to it — the job
  must be re-queued and re-dispatched to another eligible worker.
- A label selector references a key that no currently registered worker has — the job waits (does
  not immediately dead-letter).
- A worker reconnects after being dead and now satisfies a pending selector — pending jobs are
  eligible for dispatch to it.
- Label values are case-sensitive (`GPU=true` ≠ `gpu=true`).
- Maximum of 32 labels per worker; exceeding this is rejected.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to assign, update, and remove key-value labels on any worker they
  own via the management interface.
- **FR-002**: Labels MUST follow a key-value format where keys are 1–63 characters
  (alphanumeric, `-`, `_`, `.`) and values are 0–255 characters.
- **FR-003**: A worker MUST carry at most 32 labels; attempts to exceed this MUST be rejected.
- **FR-004**: Job submissions MUST accept an optional selector: a map of key-value pairs that the
  target worker's labels must fully contain (all selector pairs present in worker labels).
- **FR-005**: The dispatcher MUST only route a job to a worker whose label set is a superset of the
  job's selector (exact-match semantics per key-value pair).
- **FR-006**: Jobs with an unsatisfied selector MUST remain in a pending state and be retried for
  dispatch whenever a worker connects or its labels change.
- **FR-007**: Jobs without a selector MUST continue to be dispatched to any available worker
  (backward compatible — no behaviour change for existing jobs).
- **FR-008**: The worker list endpoint MUST support filtering by one or more label key-value pairs.
- **FR-009**: Label changes on a worker MUST trigger a re-evaluation of any pending selector jobs
  for that worker's queues.
- **FR-010**: The system MUST surface a clear error when a job's selector cannot be satisfied by
  any currently registered (non-dead) worker, so operators are not left guessing.

### Key Entities

- **WorkerLabel**: A key-value pair attached to a worker. Key: string (validated format). Value:
  string. Belongs to one worker. Many per worker (max 32).
- **JobSelector**: An optional map of key-value pairs attached to a job at enqueue time. Immutable
  after enqueue. Zero or more pairs. Used exclusively for dispatch matching.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can label a worker and see the labels reflected in under 2 seconds with
  no page refresh.
- **SC-002**: A job with a matching selector is dispatched to the correct worker within the same
  time window as an unlabelled job (no measurable routing overhead visible to the operator).
- **SC-003**: A job with an unsatisfied selector is never silently dropped — it remains visible in
  the queue as pending and its selector is shown in the job detail view.
- **SC-004**: Filtering the worker list by a label returns accurate results within 1 second for
  fleets of up to 500 workers.
- **SC-005**: Zero regressions: jobs submitted without a selector continue to behave identically
  to pre-feature behaviour (verified by existing job dispatch tests passing).

## Assumptions

- Label matching uses exact equality per key-value pair (no regex, no wildcard) for v1.
- A job's selector is immutable once enqueued; changing routing requires re-enqueuing.
- Workers are owned by individual users; label management is scoped to the owning user only.
- The public API (API-key authenticated runs) also supports the selector field on job submission.
- Workers that are in `dead` status are excluded from dispatch candidate evaluation regardless
  of label match.
- No UI for bulk-labelling multiple workers at once in v1 — one worker at a time.
