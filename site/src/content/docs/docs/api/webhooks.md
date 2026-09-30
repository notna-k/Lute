---
title: Webhooks
description: Get told when a run starts, completes or fails, and check that the call came from Lute.
---

Pass a `webhook` when you [create a run](../runs/#createRun) and core calls your URL as the run
moves along:

```json
"webhook": {
  "url": "https://ci.example.com/hooks/lute",
  "events": ["run.completed", "run.failed"]
}
```

## Events

| Event | When |
|---|---|
| `run.started` | A worker took the run; again on each retry. Off unless you list it. |
| `run.completed` | The run finished successfully. |
| `run.failed` | The run failed and has no retries left, timed out, or was lost with its worker. |

Leave `events` out to get `run.completed` and `run.failed`.

## The request

Core sends a `POST` with a JSON body:

```json
{
  "event": "run.completed",
  "run_id": "3f6c1d2e-8a4b-4c1f-9e7d-5b2a0c9d8e71",
  "queue": "deploy",
  "type": "container",
  "timestamp": 1790759741,
  "data": { "success": true, "elapsed_ms": 98214, "worker_id": "66f9e0a1c4b2d3e4f5a6b7c8" }
}
```

`data` depends on the event:

| Event | `data` |
|---|---|
| `run.started` | `worker_id`, `attempts` |
| `run.completed` | `success: true`, `elapsed_ms`, `worker_id` |
| `run.failed` | `success: false`, `error`, `attempts`, `worker_id` |

Every request carries these headers:

| Header | Value |
|---|---|
| `X-Lute-Event` | The event name. |
| `X-Lute-Delivery` | ID of this delivery; the same across retries. |
| `X-Lute-Timestamp` | Unix seconds when it was signed. |
| `X-Lute-Signature` | `t=<timestamp>,v1=<hex HMAC>` |
| `User-Agent` | `Lute-Webhook/1.0` |

Answer with any `2xx` within 10 seconds. Anything else, or no answer, is retried after 1, 2, 4, 8
and 16 minutes: 6 attempts in all.

## Checking the signature

Every run has a signing secret: the `secret` you passed, or one core generated and returned once as
`webhook_secret`. The signature is an HMAC-SHA256 over the timestamp, a dot, and the raw body:

```js
import { createHmac, timingSafeEqual } from 'node:crypto';

function verify(secret, header, rawBody) {
	const { t, v1 } = Object.fromEntries(header.split(',').map((kv) => kv.split('=')));
	const expected = createHmac('sha256', secret).update(`${t}.${rawBody}`).digest('hex');
	const fresh = Math.abs(Date.now() / 1000 - Number(t)) < 300;
	return fresh && timingSafeEqual(Buffer.from(v1, 'hex'), Buffer.from(expected, 'hex'));
}
```

Verify against the raw bytes, before any JSON parsing, and reject old timestamps so a captured
request cannot be replayed.
