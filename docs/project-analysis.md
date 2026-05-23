# Lute — Project setup analysis

## Overall verdict
Solid layout that mostly follows Go community conventions (multi-module workspace, `cmd/`/`internal/` split, generated proto isolated). The biggest issues are a couple of unidiomatic choices (multiple Go modules where one would do, mixing UI build inside the API Dockerfile) and a few inconsistencies in Makefile/Compose that will bite later.

---

## What matches Go standards well

- **`cmd/<bin>/main.go` layout** — `api/cmd/api/`, `worker/cmd/worker/`. Idiomatic.
- **`internal/` for non-exported packages** — both modules use it correctly; nothing leaks out as a public package by accident.
- **Generated protobuf isolated** in `shared/proto/` with `.golangci.yml` excluding it from lint. Good.
- **`go.work`** stitching the three modules together for local development. Standard since Go 1.18.
- **Reproducible builds**: `-ldflags="-s -w"`, `CGO_ENABLED=0`, version stamping via `-X main.Version` / `main.BuildTime`. Good.
- **Pinned linter version** (`golangci-lint@v2.11.4`) via `go run`. Excellent CI hygiene.
- **Build cache mounts** (`--mount=type=cache` for `/go/pkg/mod`, `/root/.cache/go-build`, npm). Modern, BuildKit-aware.
- **Repo-level layout** (`docs/`, `infrastructure/`, `shared/`, `ui/`) matches the [golang-standards/project-layout](https://github.com/golang-standards/project-layout) conventions.

---

## Issues / improvements

### 1. Three Go modules where one would suffice
You have `api/`, `worker/`, `shared/proto/` as separate modules with `replace github.com/lute/proto => ../shared/proto`. This is the *Kubernetes-style* multi-module pattern, but Lute isn't published as separate consumable libraries. Costs of this choice:

- Three `go.mod` / `go.sum` to keep version-synced (already drifting: `api` has `golang.org/x/net v0.50.0`, `worker` has `v0.49.0`, `proto` `v0.48.0`).
- `go.work.sum` (32 KB) duplicates much of what's in the per-module sums.
- Dependabot / `go mod tidy` work multiplied 3×.
- `replace` directives mean nothing outside the workspace can `go get` these modules.

**Recommendation**: collapse to **one module** at repo root: `github.com/notna-k/lute` with `api/`, `worker/`, `proto/` as packages. Keep `cmd/api` and `cmd/worker` at root. Only split modules if you actually publish `proto` to external consumers.

### 2. `api/Dockerfile` does too much
It builds the UI, the API, *and* cross-compiles 5 worker binaries — all inside the API image build. Problems:

- A UI-only change rebuilds worker binaries.
- A worker code change rebuilds the UI.
- The image name says "api" but its job is "monorepo release builder".

**Recommendation**: split into `infrastructure/docker/api.Dockerfile`, `worker.Dockerfile`, and a separate `release.Dockerfile` (or a CI job) that produces worker cross-compile artifacts. Move all Dockerfiles to `infrastructure/docker/` rather than `api/Dockerfile`. (Note your `.dockerignore` already comments that "Docker only reads THIS file" — that's a smell that the build context spans the whole repo.)

### 3. `infrastructure/` is sparse
Only `infrastructure/dev/` exists. The directory implies multiple environments. Either:
- Add `infrastructure/prod/`, `infrastructure/ci/` as they materialize, or
- Rename `infrastructure/dev/` → `deploy/` or `docker/` since "infrastructure" suggests IaC (Terraform/Pulumi).

### 4. Makefile inconsistencies
- `dev-up-clean` is the *documented* "main command" in comments but `help` advertises `dev-up` (which doesn't rebuild). The clean version isn't in `.PHONY`. Either rename or fix the help.
- `dev-build` doesn't pass `--no-cache` despite the help text saying "without cache".
- `worker-build-all` repeats 5 nearly identical recipes — a `foreach` loop or pattern rule would cut it to 5 lines.
- No `make test` / `make fmt` targets. Add `go test ./...` per module and `gofmt`/`goimports`.
- No `clean` target for `worker/bin/`, `bin/`, `api/internal/ui/web/`.

### 5. `.golangci.yml` is minimal
Only `default: standard` with a path exclusion. Consider enabling: `errcheck`, `gosec`, `gocritic`, `revive`, `unused`, plus `gofumpt`/`goimports` formatters. Add `enable-all: false` explicitly.

### 6. `shared/` directory has only one child (`proto/`)
If nothing else is shared, flatten to `proto/` at root. If more shared code is planned, fine to keep.

### 7. Worker `internal/utils/`
`utils` packages are a Go anti-pattern — name by purpose (`pathutil`, `ioutil`, etc.) or fold into the caller package.

### 8. `docker-compose.yml` uses obsolete `version: '3.8'`
Compose v2 ignores it and warns. Drop the line.

### 9. No `.tool-versions` / no Go version pin outside `go.mod`
CI presumably reads `go.mod` — fine. But the `golang:1.26-alpine` in the Dockerfile is duplicated; consider an `ARG GO_VERSION=1.26` to make bumps single-source.

### 10. `go 1.26.0` — verify this is intentional
Go 1.26 isn't released as of 2026-05; if you're on a tip/rc, that's fine, but `go.mod` directives typically pin to minor (`1.26`) not patch. Three modules disagreeing on patch is a setup smell.

### 11. No top-level `README.md` content (96 bytes)
Add a brief overview + pointer to AGENTS.md. AGENTS.md is excellent — promote essentials.

---

## Suggested priority

1. **Drop multi-module structure** → single module (biggest leverage).
2. **Split Dockerfiles** by component, move to `infrastructure/docker/`.
3. **Makefile**: fix `dev-build` cache flag, add `test`/`fmt`/`clean`, deduplicate cross-compile loop.
4. **Expand `.golangci.yml`** with the linters above.
5. Cosmetic: drop compose `version:`, rename `utils`, flesh out README.
