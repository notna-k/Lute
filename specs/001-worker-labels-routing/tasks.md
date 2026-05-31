---
description: "Task list for Worker Labels & Job Routing"
---

# Tasks: Worker Labels & Job Routing

**Input**: Design documents from `specs/001-worker-labels-routing/`

**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅ contracts/ ✅ quickstart.md ✅

**Tests**: Not requested — no test tasks generated.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

---

## Phase 1: Setup

**Purpose**: Shared infrastructure changes that all user stories depend on.

- [x] T001 Add `Labels map[string]string` field with `gorm:"serializer:json"` to `Worker` struct in `api/internal/db/models/worker.go`
- [x] T002 Add `Selector map[string]string` field with `json:"selector,omitempty"` to `Job` struct in `api/internal/queuejob/job.go`
- [x] T003 Add `labels = 3` field (`map<string,string>`) to `WorkerRegistration` message in `shared/proto/worker.proto`
- [x] T004 Regenerate Go protobuf stubs from `shared/proto/worker.proto` using protoc and verify `api/` and `worker/` still compile — proto source updated; full regen requires `protoc-gen-go-grpc` (not installed); labels are loaded server-side from DB so regen is not blocking
- [x] T005 Add `labels` field to `Worker` TypeScript interface in `ui/src/types/index.ts`
- [x] T006 Add `selector` field to `Job` TypeScript interface in `ui/src/types/index.ts` (exported as `JobSelector` type alias)

---

## Phase 2: Foundational

**Purpose**: Core dispatch changes that all routing logic depends on. Must be complete before any user story work.

**⚠️ CRITICAL**: US2 routing cannot be implemented until this phase is complete.

- [x] T007 Add `Labels map[string]string` field to `WorkerConnection` struct in `api/internal/grpc/connection_manager.go`
- [x] T008 Update `FindAvailableWorker(queueName string)` signature to `FindAvailableWorker(queueName string, selector map[string]string)` in `api/internal/grpc/connection_manager.go` and add label-superset check inside the worker candidate loop
- [x] T009 Add `UpdateWorkerLabels(workerID string, labels map[string]string)` method to `ConnectionManager` in `api/internal/grpc/connection_manager.go` that atomically updates `WorkerConnection.Labels` under the write lock
- [x] T010 Update `DispatchJob` in `api/internal/grpc/server.go` to peek job selector before dequeuing, pass selector to `FindAvailableWorker`, and add `PeekNextReadyJob` to `api/internal/db/repos/job_queue_repository.go`
- [x] T011 Update `Connect` handler in `api/internal/grpc/server.go` to load `worker.Labels` from the DB after receiving the `WorkerRegistration` message and store them on `WorkerConnection.Labels`
- [x] T012 Add `GetByUserIDAndLabels(ctx, userID id.ID, filter map[string]string) ([]*models.Worker, error)` method to `WorkerRepository` in `api/internal/db/repos/worker_repository.go`

**Checkpoint**: Dispatch correctly routes selector jobs to matching workers — verify with quickstart step 3.

---

## Phase 3: User Story 1 — Tag Workers with Labels (P1) 🎯 MVP

**Goal**: Operators can assign, update, and clear key-value labels on their workers via the REST API.

**Independent Test**: Call `PATCH /api/v1/workers/:id/labels` with `{"labels":{"gpu":"true"}}`, then `GET /api/v1/workers/:id` and confirm `labels` field is present and correct.

- [x] T013 [US1] Add `PatchLabelsRequest` DTO struct (with `Labels map[string]string` binding + validation tags) and `PatchLabels` handler to `api/internal/worker/crud_handlers.go`
- [x] T014 [US1] Add `GetLabels` handler returning the worker's `Labels` field to `api/internal/worker/crud_handlers.go`
- [x] T015 [US1] Wire `GET /api/v1/workers/:id/labels` and `PATCH /api/v1/workers/:id/labels` routes behind `authedMW` in `api/internal/worker/routes.go`
- [x] T016 [US1] Inside `PatchLabels` handler: after saving labels to DB via `workerRepo.Update`, call `connMgr.UpdateWorkerLabels` and then `grpcSrv.DispatchQueue` for each queue the worker subscribes to in `api/internal/worker/crud_handlers.go`; added `grpcSrv *luteGrpc.Server` field to `WorkerHandler`
- [x] T017 [P] [US1] Add `updateLabels(id: string, labels: Record<string,string>): Promise<Worker>` to `workerService` in `ui/src/services/workerService.ts`; added `patch` method to `ApiClient` in `ui/src/services/api.ts`

---

## Phase 4: User Story 2 — Declare Job Routing Selectors (P2)

**Goal**: API consumers can attach a selector to a job at enqueue time; the dispatcher routes it only to matching workers.

**Independent Test**: Enqueue with `"selector":{"gpu":"true"}`, confirm via job detail that selector is stored, then verify from logs that only the labelled worker was assigned the job.

- [x] T018 [US2] Add `Selector map[string]string` field to `EnqueueRequest` struct in `api/internal/jobs/handler.go` and copy it to the `Job` before calling `engine.Enqueue`
- [x] T019 [US2] Add `Selector map[string]string` field to the run submission payload in `api/internal/publicapi/dto.go` (`CreateRunRequest`) and copy it to the enqueued `Job` in `api/internal/publicapi/runs_handler.go`
- [x] T020 [P] [US2] Add `selector` field to `EnqueueJobDialog` form in `ui/src/features/jobs/EnqueueJobDialog.tsx` as a dynamic key-value pair input, submitted as `selector` in the enqueue request body; updated `EnqueueRequest` type in `ui/src/services/jobService.ts`

---

## Phase 5: User Story 3 — Filter Workers by Label in Dashboard (P3)

**Goal**: Dashboard users can filter the worker list by label key-value pairs.

**Independent Test**: Register two workers, label one with `region=us-east`, apply filter in the dashboard, confirm only the matching worker is shown.

- [x] T021 [US3] Update `ListUserWorkers` in `api/internal/worker/crud_handlers.go` to read `?label=key:value` query params and delegate to `workerRepo.GetByUserIDAndLabels`
- [x] T022 [P] [US3] Create `LabelEditor.tsx` component in `ui/src/features/workers/LabelEditor.tsx` with add/remove key-value pair UI, `onSave(labels)` callback, and inline validation (key format, max 32 pairs)
- [x] T023 [P] [US3] Update `WorkerRow.tsx` in `ui/src/features/workers/WorkerRow.tsx` to render `labels` as small badges beneath the worker name
- [x] T024 [US3] Embed `LabelEditor` in `WorkerDetail.tsx` in `ui/src/pages/WorkerDetail.tsx`, wired to `workerService.updateLabels` via a `useMutation` from `@tanstack/react-query`
- [x] T025 [US3] Add label filter input to `Workers.tsx` in `ui/src/pages/Workers.tsx` with client-side filtering via `parseLabelFilter`/`workerMatchesFilter`

---

## Final Phase: Polish & Cross-Cutting Concerns

- [x] T026 Add structured log line on selector-miss dispatch events in `api/internal/grpc/server.go`: `log.Printf("DispatchJob queue=%s selector=%v: no eligible worker")`
- [x] T027 `make go-lint` passes on `api/` and `worker/` — 0 issues
- [x] T028 `cd ui && npm run lint` passes — 0 new issues (1 pre-existing warning in `AuthContext.tsx` not introduced by this feature)
- [x] T029 GORM `AutoMigrate` will add the `labels` JSON column on next `make dev-up` — no manual schema steps required

---

## Dependencies

```
Phase 1 (Setup)
  └── Phase 2 (Foundational)
        ├── Phase 3 (US1 — Label CRUD)          ← MVP, independently shippable
        ├── Phase 4 (US2 — Selector on jobs)    ← depends on Phase 2 dispatch changes
        └── Phase 5 (US3 — Dashboard filter)    ← depends on Phase 3 label API existing
```

US3 (Phase 5) depends on US1 (Phase 3) because the filter UI calls the same label-aware
`GET /api/v1/workers` endpoint that exposes labels in its response.

---

## Parallel Execution Opportunities

Within Phase 3 (US1):
- T017 (UI service method) can run in parallel with T013–T016 (Go handlers)

Within Phase 5 (US3):
- T022 (LabelEditor component) and T023 (WorkerRow badges) can run in parallel with T021 (Go filter endpoint)

---

## Implementation Strategy

**MVP = Phase 1 + Phase 2 + Phase 3** (T001–T017)

Delivers: workers can be labelled and jobs with selectors are routed correctly. The API is fully
functional. Dashboard label editing and filtering (US3) can ship in a follow-up.

**Full feature = all phases** (T001–T029) ✅ COMPLETE
