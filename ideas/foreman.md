# Idea: Background Job Queue with Real-time Dashboard

## One-liner

A self-hosted background job queue where workers connect outbound via gRPC (no firewall rules needed), and the server controls job assignment — with a built-in real-time dashboard showing queue depths, live job feed, worker utilization, and latency metrics.

---

## Gap vs existing tools

| Tool | Worker visibility | Job graph | Real-time UI | Server-controlled assignment |
|---|---|---|---|---|
| Asynq | No | No | No (separate) | No (workers poll Redis) |
| Machinery | No | Partial | No | No |
| Sidekiq | Partial | No | Pro only | No |
| BullMQ | Partial | No | Separate tool | No |
| **Foreman** | Yes | Yes | Built-in | Yes (gRPC push) |

The main architectural shift: Asynq/Sidekiq workers decide what to pull from Redis themselves. Foreman's server decides what to push to which worker — this lets the server enforce priorities, drain specific workers for deploys, and maintain a real-time picture of what's executing where.

---

## Architecture

```
┌─────────────────────────────────────────────────┐
│                  Foreman Server                  │
│                                                  │
│  Gin REST API ─────────── enqueue, inspect, mgmt │
│  WebSocket Hub ─────────── real-time dashboard   │
│  gRPC Server ──────────── worker connections     │
│  Queue Engine ─────────── Redis primitives       │
│  Scheduler ────────────── delayed + cron jobs    │
│  Stats Aggregator ─────── per-minute counters    │
└─────────┬────────────────────────────┬───────────┘
          │ Redis                      │ gRPC bidirectional stream
          ▼                            ▼
    ┌──────────┐              ┌────────────────┐
    │  Redis   │              │  Worker Binary │
    │          │              │  (connects     │
    │  queues  │              │   outbound)    │
    │  jobs    │              │                │
    │  stats   │              │  handler funcs │
    │  DLQ     │              │  registered by │
    └──────────┘              │  your app code │
                              └────────────────┘
```

Workers connect outbound (same pattern as Lute agents). The server pushes a job assignment down the gRPC stream; the worker executes its handler and sends the result back up. Workers behind firewalls, inside Kubernetes, on private networks — all work fine.

---

## Job lifecycle

```
enqueue() ──► ZSET queue:{name}
                    │
           server assigns to available worker
                    │
              status: running
                    │
         worker executes handler
                    │
         ┌──────────┴──────────┐
      success               failure
         │                      │
   status: done          attempts < max_retries?
                               │
                        yes ───┤ exponential backoff → delayed ZSET
                               │
                        no ────► DLQ + status: dead
```

---

## Redis data model

```
queue:{name}            ZSET  — score = priority (higher = first), member = jobID
delayed                 ZSET  — score = run_at unix timestamp, member = jobID
cron                    ZSET  — score = next_run unix timestamp, member = cronJobID
job:{id}                HASH  — all job fields (payload, status, attempts, etc.)
dlq:{name}              LIST  — permanently failed job IDs
stats:{queue}:{minute}  HASH  — processed, failed, enqueued, latency_sum, latency_count
workers                 HASH  — workerID → last_seen (for liveness tracking)
```

---

## Proto

```proto
service WorkerService {
  rpc Connect(stream WorkerMessage) returns (stream ServerMessage);
}

message WorkerMessage {
  oneof payload {
    WorkerRegistration  register  = 1;
    JobResult           result    = 2;
    WorkerHeartbeat     heartbeat = 3;
  }
}

message ServerMessage {
  oneof payload {
    JobAssignment       assign    = 1;
    DrainSignal         drain     = 2;
  }
}

message JobAssignment {
  string job_id      = 1;
  string queue       = 2;
  string type        = 3;
  bytes  payload     = 4;
  int32  timeout_sec = 5;
}

message JobResult {
  string job_id     = 1;
  bool   success    = 2;
  string error      = 3;
  int64  elapsed_ms = 4;
}
```

---

## REST API

```
POST   /api/jobs                    enqueue a job
GET    /api/jobs/:id                job detail
POST   /api/jobs/:id/retry          re-enqueue failed job
DELETE /api/jobs/:id                cancel pending job

GET    /api/queues                  list queues with depth + stats
GET    /api/queues/:name/jobs       paginated job list
POST   /api/queues/:name/pause
POST   /api/queues/:name/resume
POST   /api/queues/:name/drain      wait for running jobs, reject new
POST   /api/queues/:name/purge      delete all pending jobs

GET    /api/workers                 connected workers + utilization
GET    /api/stats/queues            time series (1m buckets, last 60 min)
GET    /api/stats/queues/:name

GET    /api/dlq/:queue              dead letter queue contents
POST   /api/dlq/:queue/retry-all

WS     /ws                          real-time event stream (dashboard)
```

---

## Worker SDK usage

```go
w := worker.New(worker.Config{
    ServerAddr:  "foreman:9000",
    Queues:      []string{"emails", "default"},
    Concurrency: 10,
})

w.Handle("send_email", func(ctx context.Context, job *worker.Job) error {
    var payload EmailPayload
    json.Unmarshal(job.Payload, &payload)
    return smtp.Send(payload)
})

w.Handle("resize_image", func(ctx context.Context, job *worker.Job) error {
    // ...
})

w.Run() // connects outbound via gRPC, blocks
```

Enqueue from anywhere via HTTP:

```bash
curl -X POST http://foreman/api/jobs \
  -H "X-API-Key: ..." \
  -d '{"queue":"emails","type":"send_email","payload":{"to":"x@y.com"}}'
```

---

## Project structure

```
foreman/
├── server/
│   ├── api/
│   │   ├── config/
│   │   ├── handlers/           # job, queue, worker, stats handlers
│   │   ├── middleware/         # cors, logger, api-key auth
│   │   ├── websocket/          # hub.go + client.go
│   │   ├── grpc/               # server.go (bidirectional stream)
│   │   ├── models/
│   │   └── main.go
│   ├── queue/
│   │   ├── engine.go           # enqueue, dequeue, complete, fail
│   │   ├── scheduler.go        # delayed + cron polling loop
│   │   └── stats.go            # per-minute counters, percentiles
│   └── proto/
│       └── worker/
├── worker/
│   ├── client/                 # gRPC stream client
│   ├── handler/                # handler registry (type → func)
│   └── main.go
└── ui/
    └── src/
        ├── pages/
        │   ├── Dashboard.tsx   # queue depths, stats, live feed
        │   ├── Jobs.tsx        # job list + detail + retry
        │   ├── Workers.tsx     # connected workers + utilization
        │   └── Queues.tsx      # queue management (pause/drain/purge)
        └── components/
            ├── LiveFeed.tsx    # WebSocket job event stream
            ├── LatencyChart.tsx
            └── QueueDepthBar.tsx
```

---

## What carries over from Lute

| Pattern | Lute | Foreman |
|---|---|---|
| gRPC bidirectional stream | agent heartbeat | worker job assignment |
| WebSocket hub | machine status events | job lifecycle events |
| Gin HTTP server | REST API | REST API |
| gorilla/websocket | hub.go + client.go | same |
| Middleware (CORS, logger) | ✓ | ✓ |
| React + Vite + MUI + Recharts | ✓ | ✓ |
| Docker + Makefile | ✓ | ✓ |

## What changes

| Concern | Lute | Foreman |
|---|---|---|
| Storage | MongoDB | Redis (sorted sets for queues) |
| Auth | Firebase | API key |
| Agent purpose | ping/pong heartbeat | job fetch → execute → report |
| Proto | `AgentService.Connect` | `WorkerService.Connect` |
| WebSocket events | machine status | job lifecycle |

---

## Distributed systems concepts involved

- **Server-controlled dispatch** — server pushes work to workers rather than workers polling; enables priority enforcement, drain-for-deploy, and real-time utilization tracking
- **At-least-once delivery** — job stays `running` in Redis; if worker disconnects mid-job the server re-enqueues it
- **Exponential backoff retry** — failed jobs re-enter the delayed ZSET with increasing delays
- **Dead letter queue** — permanently failed jobs moved to DLQ, inspectable and replayable from UI
- **Job graph (fan-out / fan-in)** — jobs declare `depends_on`; server holds dependents until parents complete, then re-enqueues
- **Churn tolerance** — workers reconnect with exponential backoff; in-flight jobs re-queued if worker goes offline

---

## CV pitch

> "A self-hosted background job queue where the server pushes assignments to workers over gRPC bidirectional streams — enabling server-controlled priority dispatch, graceful drain-for-deploy, and real-time dashboard visibility into queue depths, worker utilization, and job graphs."
