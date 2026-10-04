---
title: API overview
description: Base URL, API keys, errors, idempotency and paging for the public API.
sidebar:
  label: Overview
---

The public API lets scripts and other CI systems start jobs, follow their runs, and manage workers.
It is JSON over HTTPS under `/api/public/v1` on your core. The [`lute` command line](../cli/) is
built on it; reach for it before writing a client of your own.

The reference pages are generated from [`openapi.yaml`](../../openapi.yaml), which a test in
core keeps in step with the routes it serves. Point any OpenAPI client generator at it.

| Resource | What it is |
|---|---|
| [Server and identity](../api/identity/) | The server's version, and who a key acts as. |
| [Jobs](../api/jobs/) | Job definitions, their parameters, and starting a run of one. |
| [Runs](../api/runs/) | Every run, of a job or of a raw queued job: status and logs. |
| [Workers](../api/workers/) | The machines runs go to. |
| [Worker commands](../api/commands/) | One-off commands executed by a worker's agent. |
| [Registration tokens](../api/tokens/) | Tokens a new worker enrols with. |

## Authentication

Create a key in the panel under **Settings → API keys**. The key is shown once. Send it as a bearer
token:

```bash
curl https://lute.example.com/api/public/v1/runs \
  -H "Authorization: Bearer $LUTE_API_KEY"
```

Keys come in two kinds:

| Kind | Acts as | Sees | Use it for |
|---|---|---|---|
| Account key | The user who made it | That user's runs, runs service keys started, and that user's workers | Your laptop and your own scripts |
| Service key | Itself | Every run. Not workers, which belong to users: those endpoints answer `403` | CI and other automation that should outlive any one person |

Only its owner sees an account key, and it stops working when its owner's account is removed. Every
signed-in user sees and can revoke service keys. Runs a service key starts show its name in the panel.
[`GET /whoami`](../api/identity/who-am-i/) says which kind a key is.

Revoke a key under **Settings → API keys**; requests with it fail with `401` right away.

## Errors

Every error has the same shape:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "webhook.url is required when webhook is provided",
    "fields": { "webhook.url": "required" }
  }
}
```

| `code` | Status | When |
|---|---|---|
| `bad_request` | 400 | The request cannot be read. |
| `validation_failed` | 400 | Inputs are invalid; `fields` names each one. |
| `unauthorized` | 401 | Missing, unknown or revoked API key. |
| `forbidden` | 403 | The key may not do this, e.g. a service key on a worker endpoint. |
| `not_found` | 404 | It does not exist, or it is not yours. |
| `conflict` | 409 | It clashes with the current state, e.g. cancelling a run that started. |
| `unavailable` | 503 | Something it depends on is offline, e.g. the worker holding a log. |
| `internal` | 500 | A bug. The message is generic; details are in core's log. |

## Idempotency

Send an `Idempotency-Key` header when you [start a job](../api/jobs/start-job-run/), or an
`idempotency_key` field when you create a raw run. A second request with the same key returns the
first run with `200` instead of creating another, so a retried request never starts a job twice. Keys
are per owner: per user, or per service key.

## Paging

List endpoints take `offset` and `limit` and answer with `total`, `offset` and `limit` next to the
items. Logs page with a cursor instead; see [Read a run's log](../api/runs/get-run-logs/).

## Conventions

- IDs are 24-character hex strings. Wherever a run ID goes, its first 8 characters work too: the
  short form, `#66fb1c2d`, that the panel and the CLI show.
- Times are RFC 3339 in UTC.
- Fields without a value are left out of responses.
