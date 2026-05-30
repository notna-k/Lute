<!--
SYNC IMPACT REPORT
==================
Version change: (unversioned template) → 1.0.1
Added sections:
  - Core Principles (5 principles)
  - Technology Stack Constraints
  - Development Workflow
  - Governance
Modified principles: N/A (initial constitution)
Removed sections: N/A (initial constitution)
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ Constitution Check section aligns with these principles
  - .specify/templates/spec-template.md ✅ User stories + acceptance scenarios align
  - .specify/templates/tasks-template.md ✅ Phase structure covers security, observability, testing tasks
Deferred TODOs: none
-->

# Lute Constitution

## Core Principles

### I. Service Boundary Integrity (NON-NEGOTIABLE)

Lute MUST maintain hard boundaries between its three service modules:
`api` (Go/Gin REST + WebSocket), `worker` (Go/Docker orchestration), and `ui` (React/TypeScript).

- Cross-service communication MUST go through defined proto contracts in `shared/proto`.
- No module may import another module's internal packages directly.
- The `ui` MUST communicate with `api` exclusively via HTTP REST or WebSocket; it MUST NOT call
  `worker` directly.
- Breaking a proto contract requires a version bump and backward-compatible migration plan.

**Rationale**: Prevents tight coupling that makes independent deployment and testing impossible.
This is the foundational architectural decision for the platform.

### II. API-First Design

Every new product capability MUST have its API surface defined before implementation begins.

- REST endpoints MUST be designed with explicit request/response shapes before handler code is written.
- gRPC services MUST have `.proto` definitions approved before Go stub generation.
- The `ui` is a consumer of the API, not a driver — UI changes MUST NOT dictate backend shapes.
- Public endpoints MUST be documented; internal gRPC endpoints MUST have comments in `.proto` files.

**Rationale**: API-first ensures the platform remains UI-agnostic and enables parallel development
across frontend and backend without blocking dependencies.

### III. Security by Default (NON-NEGOTIABLE)

All protected resources MUST enforce JWT authentication (HS256 + refresh-token families).

- JWT verification MUST occur in Gin middleware before any handler executes.
- Refresh tokens MUST be stored server-side and invalidated on rotation (token family tracking).
- No secrets, credentials, or tokens MUST appear in source code or committed `.env` files.
- User input at every HTTP boundary MUST be validated using `go-playground/validator` bindings.
- SQL queries MUST use GORM parameterized queries; raw SQL with user data is forbidden.
- CORS MUST be explicitly configured; wildcard origins are forbidden in production.

**Rationale**: Security failures in a SaaS platform are catastrophic for user trust. These rules
encode lessons from common auth vulnerabilities (token leaks, SQL injection, CSRF).

### IV. Observability First

Every service MUST emit structured, queryable signals. Silence is not acceptable behavior.

- All services MUST instrument with OpenTelemetry (`go.opentelemetry.io/otel`).
- HTTP handlers and gRPC methods MUST emit traces with request/response metadata.
- Errors MUST be logged with structured fields (at minimum: `error`, `service`, `operation`).
- The `worker` MUST emit job lifecycle events (started, completed, failed) with container IDs.
- Health check endpoints MUST exist at `/health` (liveness) in the `api` service.

**Rationale**: A SaaS platform without observability cannot diagnose production incidents.
OpenTelemetry is already wired into both Go modules — it MUST be used, not bypassed.

### V. Simplicity and YAGNI

Complexity MUST be justified by a concrete, present requirement.

- No abstractions, interfaces, or patterns introduced for hypothetical future use.
- The `api` and `worker` MUST prefer flat package structures over deep hierarchies.
- The `ui` MUST prefer co-located component files over premature shared-component extraction.
- Dependencies MUST be chosen for maintenance longevity, not feature breadth.
- `make dev-up` MUST bring the full stack to a working state; local dev MUST NOT require manual
  steps beyond this command.

**Rationale**: Premature abstraction in a growing codebase is a primary source of maintenance debt.
Start simple; extract only when the third use case makes the pattern clear.

## Technology Stack Constraints

The following technology choices are locked for this platform. Deviating requires a constitution
amendment with explicit rationale.

**Backend — API** (`api/`):
- Language: Go 1.26+
- HTTP framework: Gin (`github.com/gin-gonic/gin`)
- ORM: GORM with PostgreSQL in production, SQLite for development/testing
- Auth: `github.com/golang-jwt/jwt/v5` (HS256 only; RS256 requires amendment)
- gRPC: `google.golang.org/grpc` + shared proto definitions in `shared/proto`

**Backend — Worker** (`worker/`):
- Language: Go 1.26+
- Container orchestration: Docker SDK (`github.com/docker/docker`)
- Service protocol: gRPC client to `api`

**Frontend** (`ui/`):
- Language: TypeScript (strict mode enforced)
- Framework: React 18 + Vite
- Styling: TailwindCSS (utility-first; custom CSS requires justification)
- Data fetching: `@tanstack/react-query` (no raw `fetch` calls inside components)
- Routing: `react-router-dom` v6
- Node.js runtime: ≥ 25.0.0

**Infrastructure**:
- Local dev: Docker Compose (`infrastructure/dev`)
- Build: `Makefile` targets are the canonical entry points for all operations

## Development Workflow

These workflow rules apply to all contributors and all features.

- All features MUST be developed on a feature branch following the naming convention
  `###-short-description` (e.g., `001-user-auth`).
- The `make go-lint` target MUST pass before any Go PR is submitted.
- The `make dev-up` target MUST succeed on a clean checkout before marking a feature complete.
- Database schema changes MUST include a GORM migration; no manual schema edits in production.
- The UI MUST be built via `make ui-build` and embedded in the API binary before release.
- PR descriptions MUST reference the relevant spec or plan document from `.specify/`.

## Governance

This constitution supersedes all informal practices, prior undocumented norms, and verbal agreements.

- **Amendment procedure**: Any principle change requires a PR updating this file with:
  1. Rationale for the change.
  2. A migration plan for code that relied on the old principle.
  3. Version bump per semantic versioning rules (MAJOR/MINOR/PATCH).
- **Compliance review**: Every PR MUST verify compliance with all NON-NEGOTIABLE principles
  (I. Service Boundary Integrity, III. Security by Default) before merge.
- **Versioning policy**:
  - MAJOR — principle removal or redefinition.
  - MINOR — new principle or section added.
  - PATCH — wording, typos, non-semantic clarifications.
- **Runtime guidance**: See `AGENTS.md` and the current plan document for feature-level agent guidance.

**Version**: 1.0.1 | **Ratified**: 2026-05-30 | **Last Amended**: 2026-05-30
