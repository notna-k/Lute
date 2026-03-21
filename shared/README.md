# Shared

This directory holds **multiple Go modules** (one per subdirectory), not a single `shared` module.

| Module | Path | Import |
|--------|------|--------|
| Worker gRPC proto | [`proto/`](proto/) | `github.com/lute/proto` |

Add new shared modules as sibling folders under `shared/` with their own `go.mod`.
