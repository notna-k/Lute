---
title: API overview
description: Base URL, API keys, errors, idempotency and paging for the public API.
sidebar:
  label: Overview
---

The public API lets scripts and other CI systems start runs, follow them, and manage workers. It is
JSON over HTTPS under `/api/public/v1` on your core.

The reference pages are generated from [`openapi.yaml`](../../openapi.yaml), which a test in
core keeps in step with the routes it serves. Point any OpenAPI client generator at it.

| Resource | What it is |
|---|---|
| [Runs](../api/runs/) | Jobs you put on a queue, their status and logs. |
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

A key acts as the user who created it: it sees that user's runs and workers. Revoke it in the same
place; requests with it fail with `401` right away.

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
| `unauthorized` | 401 | Missing or unknown API key. |
| `not_found` | 404 | It does not exist, or it is not yours. |
| `conflict` | 409 | It clashes with the current state, e.g. cancelling a run that started. |
| `unavailable` | 503 | Something it depends on is offline, e.g. the worker holding a log. |
| `internal` | 500 | A bug. The message is generic; details are in core's log. |

## Idempotency

Send an `idempotency_key` when you create a run. A second request with the same key returns the
first run with `200` instead of creating another, so a retried request never starts a job twice.

## Paging

List endpoints take `offset` and `limit` and answer with `total`, `offset` and `limit` next to the
items. Logs page with a cursor instead; see [Read a run's log](../api/runs/#getRunLogs).

## Conventions

- IDs are strings. Runs use UUIDs; everything else uses 24-character hex IDs.
- Times are RFC 3339 in UTC.
- Fields without a value are left out of responses.
