# Contributing to Lute

Lute is a young project with no users yet, so almost everything is still open for discussion. Bug
reports, ideas and pull requests are all welcome.

## Before you start

- For anything bigger than a small fix, open an issue first and describe what you want to change.
- Keep it simple. A smaller change that a stranger can read beats a clever one.

## Development setup

```bash
cp .env.example .env     # set JWT_SECRET (>= 32 bytes), ADMIN_EMAIL, ADMIN_PASSWORD
make dev-up              # postgres + core + admin at :8080, API at :8081
make dev-logs            # tail everything
make dev-down            # stop (dev-clean also wipes Postgres)
```

The UI can also run against the dev stack directly:

```bash
cd ui && npm install && npm run dev     # :3000
```

You need Docker, Go 1.26+ and Node 25.x. `infrastructure/dev/README.md` documents the stack and
every environment variable.

## Tests

Unit and package tests live next to the code and run with `make go-test`.

The end-to-end suite in `api/e2e/` drives the whole system instead: core boots inside the test
process on ephemeral ports, the real `lute-worker` binary runs as a child process, job bodies run in
real containers, and a throwaway `postgres:17-alpine` backs it. Tests only use HTTP, WebSocket and
gRPC, so they describe how Lute behaves rather than how it is written.

```bash
make e2e        # compiles the agent, then runs the suite (needs Docker)
make e2e-vet    # vet the e2e-tagged files
```

It is behind the `e2e` build tag, so `make go-test` stays fast and needs no Docker. Useful details:

- `LUTE_E2E_POSTGRES_DSN=postgres://...` runs against an existing Postgres instead of starting a
  container — that is how CI uses its service container.
- Each test writes its diagnostics to `api/e2e/_artifacts/<test>/`: core's log, every agent's
  stderr, and the job logs. CI uploads them when the suite fails.
- A new scenario is a top-level `Test*` with one `harness.Stack`. Wait on observable state with
  `harness.WaitFor` / `Eventually` / `Never`; a `sleep` is either slow or flaky.

## Before opening a pull request

Run whatever is relevant to the files you touched:

```bash
go build ./...          # in api/ and/or worker/
go vet ./...
make go-lint            # golangci-lint for both Go modules
go test ./...           # in the affected module
make e2e                # if you touched core, the worker, or the queue
cd ui && ./node_modules/.bin/tsc --noEmit
```

## Commits and pull requests

- Branch off `master`, open the PR against `master`.
- Conventional commits: `feat(jobs): ...`, `fix(worker): ...`, `chore: ...`, `docs: ...`.
- Keep the PR description about what changed and why, and mention anything you deliberately left
  out.

## AI-assisted contributions

Using an AI assistant (Claude Code, Copilot, Cursor, whatever you like) is explicitly fine — parts
of this repo were written that way. Two conditions:

- **You own the result.** Read every line you submit, make sure it builds, lints and is tested, and
  be able to explain why it is written that way in review.
- **No unreviewed bulk output.** Large generated diffs, invented APIs, or comments and docs that
  describe code that does not exist will be sent back.

## License

By contributing you agree that your contribution is licensed under the [MIT License](LICENSE) that
covers this project.
