---
title: Configuring core
description: Every environment variable core reads, with its default.
---

Core is configured with environment variables. The
[Compose examples](https://github.com/notna-k/Lute/tree/master/examples) set them in `compose.yaml`
and read secrets from `.env`. Durations use Go syntax: `90s`, `15m`, `720h`.

Workers have their own settings; see [Running a worker](../workers/running/#settings).

## Required

| Variable | Meaning |
|---|---|
| `JWT_SECRET` | Signs panel sessions. At least 32 bytes: `openssl rand -base64 48`. |
| `ADMIN_EMAIL` | The first admin, created on startup if no user with this email exists. |
| `ADMIN_PASSWORD` | That admin's password. |

## Database

| Variable | Default | Meaning |
|---|---|---|
| `POSTGRES_DSN` | `postgres://lute:lute@localhost:5432/lute?sslmode=disable` | Postgres holds everything durable, the job queue included. |

## Servers

| Variable | Default | Meaning |
|---|---|---|
| `SERVER_HOST` | `0.0.0.0` | HTTP listen address. |
| `SERVER_PORT` | `8080` | HTTP port: REST, WebSocket, and the panel when core serves it. |
| `SERVER_READ_TIMEOUT` | `15s` | |
| `SERVER_WRITE_TIMEOUT` | `15s` | |
| `SERVER_IDLE_TIMEOUT` | `60s` | |
| `GRPC_HOST` | `0.0.0.0` | gRPC listen address for workers. |
| `GRPC_PORT` | `50051` | gRPC port. It speaks plaintext; put a TLS proxy in front across untrusted networks. |
| `CORS_ALLOWED_ORIGINS` | the panel's local origins | Comma-separated origins allowed to call the API from a browser. |
| `GIN_MODE` | `debug` | `release` in production. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`. |

## Sign-in

| Variable | Default | Meaning |
|---|---|---|
| `ACCESS_TOKEN_TTL` | `15m` | Lifetime of a panel access token. |
| `REFRESH_TOKEN_TTL` | `720h` | Lifetime of a refresh token (30 days). |
| `JWT_ISSUER` | `lute` | Issuer claim in tokens. |
| `AUTH_COOKIE_SECURE` | `false` | Mark the refresh cookie `Secure`; turn it on behind HTTPS. |

## Jobs and queue

| Variable | Default | Meaning |
|---|---|---|
| `JOB_DEFS_DIR` | empty | Directory of job definition YAML to [sync from](../jobs/git-sync/). |
| `QUEUE_POLL_INTERVAL` | `1s` | How often the queue sweeper runs. |
| `QUEUE_LEASE_GRACE` | `60s` | Added to a job's timeout before its lease is reaped, so a late result is not lost. |
| `QUEUE_RECLAIM_AFTER` | `60s` | How long one sweeper holds a claimed lease before another core may retake it. |
| `WEBHOOK_POLL_INTERVAL` | `5s` | How often pending webhook deliveries are sent. |

## Workers

| Variable | Default | Meaning |
|---|---|---|
| `WORKER_BOOTSTRAP_TOKEN` | empty | Seeded as a registration token on startup; the dev stack's worker uses it. |
| `WORKER_GRPC_ADDR` | request host + `GRPC_PORT` | gRPC address printed in the **Add worker** command. |
| `WORKER_IMAGE` | the tag matching core's version | Image printed in the **Add worker** command. |
| `HEARTBEAT_CHECK_INTERVAL` | `30s` | How often worker liveness is checked. |
| `HEARTBEAT_PING_TIMEOUT` | `5s` | |
| `HEARTBEAT_MAX_RETRIES` | `3` | Missed checks before a worker is marked dead. |
| `METRICS_SNAPSHOT_INTERVAL` | `5m` | How often worker metrics are recorded for history. |

## Compose stack only

| Variable | Default | Meaning |
|---|---|---|
| `ADMIN_PORT` | `8080` | Host port of the panel container. |
| `API_HTTP_PORT` | `8081` | Host port of core's HTTP. |
| `API_GRPC_PORT` | `50051` | Host port of core's gRPC. |
