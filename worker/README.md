# Lute worker

Go module `github.com/lute/worker`.

## Layout

- **`cmd/worker/`** — `main` package (CLI entrypoint: `lute-worker`).
- **`internal/`** — implementation packages not importable by other modules (`handler`, `heartbeat`, `metrics`, `runner`, `setup`, `utils`).

Build from the module root:

```bash
go build -o lute-worker ./cmd/worker
```

Or `make worker-build` from the repository root.
