# AGENTS.md

## Ground Rules

This is a project, aimed to create a good jenkins alternative, with configuration as code, pipelines that are good, no legacy weight.

For a development and testing, compile binaries only for linux amd64. The machine I am working on is not very powerful.

By default, I want the code, setups, etc to be as simple and elegant as possible.
This is a sole PET project I do on weekends for my CV.
If, doing something, you find something to improve - I am open to it, go ahead and propose.

The project doesn't have real users yet, so you don't have to worry about backwards compatibility.
You can manipulate dev docker containers for this app however u want, there is no valuable data in them.

Chrome plugin is available to check and test the UI.

## Conventions

- **Git is the source of truth for job definitions.** YAML in `infrastructure/dev/jobdefs/` (one or more `---` documents per file) is synced into Postgres on startup and via "Sync from Git". A sync writes a definition only when its file changed, so panel edits stand until then; definitions missing from Git are kept unless *Prune* is on in Settings. Anything that differs from Git is flagged in the panel, and its YAML can be exported to commit it.
- **New DB model** → register it in `migrate.RegisteredModels()` (`api/internal/db/migrate/migrate.go`), or its table is never created.
- **gRPC contract** → edit `shared/proto/worker.proto` and run `shared/proto/generate.sh`; never hand-edit `*.pb.go`.
- Config comes from the repo-root `.env` (template: `.env.example`).

## Commands

```bash
make dev-up / dev-down / dev-logs     # compose stack (dev-clean wipes Postgres)
make worker-build                     # host (linux/amd64) worker binary
make go-lint                          # golangci-lint for api + worker
cd ui && npm run dev                  # UI dev server on :3000
```

Don't run `make worker-build-all` (cross-compiles 5 platforms) — use `worker-build` or `worker-build-linux`. `make api-build` runs `npm ci` + a full UI build; avoid it unless needed.

## Before calling a change done

Run what's relevant to the files touched:

1. `go build ./...` in `api/` and/or `worker/`
2. `go vet ./...` / `make go-lint`
3. `go test ./...` in the affected module
4. UI: `./node_modules/.bin/tsc --noEmit` in `ui/`
5. If told to, explore the UI through the Chrome connector to verify behaviour end to end. Supply screenshots or video recording to the PR.

## Tests

Every new feature or package ships with tests. Go tests live next to the code (`*_test.go`); today only `worker/internal/joblog` has them, so expect to create the first tests in a package.

## Git

Work on a feature branch off `master`, use conventional commits (`feat(jobs): ...`, `fix(worker): ...`), and open a PR to `master`.

## End-to-end tests

`api/e2e/` drives the whole system: core boots in the test process on ephemeral ports,
the **real** `lute-worker` binary runs as a child process, job bodies are real
containers, and a throwaway `postgres:17-alpine` backs it. Tests assert only through
HTTP, WebSocket and gRPC, so they survive refactors.

```bash
make e2e        # compiles the agent, then runs the suite (needs Docker)
make e2e-vet    # vet the tagged files
```

- Guarded by `//go:build e2e`, so `make go-test` stays fast and does not touch Docker.
- `LUTE_E2E_POSTGRES_DSN` points the suite at an existing Postgres instead of starting one.
- Per-test diagnostics (core log, agent stderr, job logs) land in `api/e2e/_artifacts/<test>/`;
  CI uploads them on failure.
- Add a scenario as a top-level `Test*` with one `harness.Stack`, and reach for
  `harness.WaitFor`/`Eventually`/`Never` rather than a sleep.
