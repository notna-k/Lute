# Contributing to Lute

Lute is a young project with no users yet, so almost everything is still open for discussion. Bug
reports, ideas and pull requests are all welcome.

## Before you start

- For anything bigger than a small fix, open an issue first and describe what you want to change.
  It is cheaper to agree on the shape of a change than to review a finished one that went the wrong
  way.
- There is no backwards-compatibility burden yet. If the right fix is a breaking change, propose it.
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

## House rules

- **Git is the source of truth for job definitions.** YAML in `infrastructure/dev/jobdefs/` is
  synced into Postgres; panel edits are drift until they are exported and committed.
- **New DB model** → register it in `migrate.RegisteredModels()`
  (`api/internal/db/migrate/migrate.go`), or its table is never created.
- **gRPC contract** → edit `shared/proto/worker.proto` and run `shared/proto/generate.sh`. Never
  hand-edit the generated `*.pb.go`.
- Configuration comes from the repo-root `.env`; add new keys to `.env.example` with a sane default.

## Before opening a pull request

Run whatever is relevant to the files you touched:

```bash
go build ./...          # in api/ and/or worker/
go vet ./...
make go-lint            # golangci-lint for both Go modules
go test ./...           # in the affected module
cd ui && ./node_modules/.bin/tsc --noEmit
```

New features and packages ship with tests. Go tests live next to the code as `*_test.go`; many
packages have none yet, so you may be writing the first ones.

If the change is visible in the panel, add a screenshot or a short recording to the PR.

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

Attribution in commit trailers (`Co-Authored-By:`) is welcome but not required.

## License

By contributing you agree that your contribution is licensed under the [MIT License](LICENSE) that
covers this project.
