# Idea: Unit-based Data Processing Platform

## One-liner

A data processing platform where the primary abstraction is not the pipeline, but the **Unit** — a typed, schema-versioned data entity with built-in history and reactive processes attached to it.

---

## The problem with current ETL/streaming tools

| Tool | Primary abstraction | Weakness |
|---|---|---|
| Kafka + Flink | Stream of events | Entity identity and history are your problem |
| Airbyte / Fivetran | Connector pipeline | Transforms are bolted on, no entity concept |
| dbt | SQL transform | Only transforms, no extraction or entity history |
| Benthos | Pipeline config | No schema ownership, stateless |
| EventStoreDB | Event stream per aggregate | Complex to set up, no polyglot execution |

**The gap**: none of these give you a first-class data entity that owns its schema, its history, and the processes that react to it — all in one place.

---

## Core concepts

### Unit

A Unit is a typed data entity with a base and an arbitrary set of children. Each child can live in a different storage backend.

```yaml
unit: Order
schema:
  version: 3
  fields:
    id: uuid
    customer_id: uuid
    total: float
    status: enum[pending, paid, shipped, cancelled]
children:
  invoice:
    backend: s3
    key: "invoices/{id}.pdf"
  shipping_status:
    backend: http
    url: "https://api.dhl.com/track/{tracking_number}"
    cache: 5m
  fraud_score:
    backend: unit
    ref: FraudScore
    foreign_key: order_id
  audit_log:
    backend: event_log
```

Accessing children is uniform regardless of where they live:

```
GET /units/order/abc123/children/invoice        → streams PDF from S3
GET /units/order/abc123/children/shipping_status → fetches + caches DHL response
GET /units/order/abc123/children/fraud_score     → resolves linked FraudScore Unit
GET /units/order/abc123/children/audit_log       → returns event history
```

Properties:
- **Schema versioned** — schema changes are tracked; old and new versions coexist during migration
- **History** — every mutation is stored as an immutable event; current state is a projection
- **Identity** — each Unit instance has a stable ID; you can fetch current state or replay history
- **Children** — enrichments and attachments declared in schema, resolved transparently by the platform
- **Subscribable** — processes declare which Units they react to

### Process

A Process is a function that reacts to Unit changes. It can be written in any language and optionally delegates execution to an external orchestrator.

```yaml
process: EnrichOrder
trigger:
  unit: Order
  on: [created, updated]
  filter: "status == 'paid'"
mode: async
executor: dagu          # optional — delegate to Dagu DAG
dag: process-order      # Dagu DAG name to trigger
# or: run inline
runtime: python
code: ./processes/enrich_order.py
```

Processes can:
- Run inline (platform spawns the runtime, passes Unit data via stdin/gRPC)
- Delegate to an external orchestrator (Dagu, Temporal, Prefect) via webhook or API call
- Write results back to the Unit as new children or field updates

---

## What's unique

### Entity-centric, not pipeline-centric

In Kafka you think: "I have a topic of order events, I write a consumer to process them."
Here you think: "I have an Order Unit with an invoice child in S3, a fraud score child in another Unit, and a shipping status child from an external API."

The Unit owns the schema, the history, the children, and the processes. You don't wire pipelines — you define entities and their shape.

### Unified child access over heterogeneous storage

Children of a Unit can live anywhere — Postgres, S3, an external HTTP API, another Unit. The platform resolves them transparently. No application code needs to know "go to S3 for the invoice, call DHL for shipping status."

### Orchestrator-agnostic execution

The platform does not force you to use its built-in process executor. Processes can delegate to Dagu, Temporal, Prefect, or Airflow via webhooks. The platform owns the **data entity layer**; orchestrators own the **execution layer**. They're pluggable — the platform works standalone too.

```
Order Unit created
  → platform emits webhook
    → Dagu starts "process-order" DAG
        ├── step: validate (calls GET /units/order/{id})
        ├── step: charge payment
        └── step: generate invoice (calls POST /units/order/{id}/children/invoice)
```

### Schema evolution built-in

When a schema changes, old Unit instances are migrated lazily via a declared migration function. No manual migration scripts, no downtime.

---

## Architecture (MVP)

```
┌──────────┐    REST / gRPC    ┌───────────────────┐
│  Client  │ ────────────────► │   API Gateway     │
└──────────┘                   └────────┬──────────┘
                                        │
                ┌───────────────────────┼───────────────────────┐
                │                       │                        │
         ┌──────▼──────┐      ┌─────────▼───────┐    ┌──────────▼──────┐
         │  Unit Store  │      │ Schema Registry  │    │ Process Scheduler│
         │  (Postgres + │      │                 │    │                  │
         │   event log) │      └─────────────────┘    └────────┬─────────┘
         └──────┬───────┘                                       │
                │                                    ┌──────────▼──────────┐
         ┌──────▼───────┐                            │  inline executor    │
         │ Child Resolver│                            │  (container/wasm)   │
         │  S3 / HTTP /  │                            └──────────┬──────────┘
         │  Unit / log   │                                       │ or
         └──────────────┘                            ┌──────────▼──────────┐
                                                     │ external orchestrator│
                                                     │ (Dagu / Temporal /  │
                                                     │  Prefect — webhook) │
                                                     └─────────────────────┘
```

### Services

- **API Gateway** — REST API for reading/writing Units, navigating children, defining schemas and processes
- **Unit Store** — persists Unit instances + immutable event log (Postgres)
- **Schema Registry** — stores schema + children declarations, validates mutations
- **Child Resolver** — resolves children transparently: fetches from S3, calls HTTP APIs (with caching), joins linked Units
- **Process Scheduler** — watches the event log, triggers matching processes
- **Inline Executor** — runs process code in isolation (container/wasm), handles retries and timeouts
- **Orchestrator Bridge** — emits webhooks to external orchestrators (Dagu, Temporal, Prefect); optional, platform works without it

---

## MVP scope

### In scope
- [ ] Define Unit schemas + children declarations via YAML or API
- [ ] Create / update Unit instances via REST
- [ ] Event log per Unit instance (full history queryable)
- [ ] Child resolution: S3 files and linked Units (foreign key)
- [ ] Attach async processes (trigger on create/update, filter by field)
- [ ] Inline execution: Python and Go runtimes
- [ ] Orchestrator bridge: trigger Dagu DAG via webhook on Unit change
- [ ] Process retry with exponential backoff + dead-letter queue
- [ ] Basic UI: browse Units, inspect children, view history, view process runs

### Out of scope (post-MVP)
- [ ] HTTP API children (external APIs as children)
- [ ] Sync processes (complex transactional guarantees)
- [ ] Schema migration functions
- [ ] WASM runtime
- [ ] Cross-Unit joins and projections
- [ ] Kafka/external event source ingestion
- [ ] Temporal / Prefect bridge (Dagu only for MVP)

---

## Distributed systems concepts involved

- **Event sourcing** — Unit history is the source of truth; current state is derived
- **At-least-once delivery** — process scheduler delivers events to executors with retries
- **Idempotency** — processes must be idempotent; platform tracks which events have been processed
- **Schema evolution** — versioned schemas with backward compatibility checks
- **Isolation** — each process runs in its own container; failures don't affect other processes

---

## Integration with orchestrators

The platform integrates with existing orchestrators without forking or depending on them:

- **Your platform** owns: Unit schema, history, children, storage abstraction, webhook emission
- **Dagu / Temporal / Prefect** owns: task DAGs, step dependencies, retry policies, execution logs

Integration is one-directional from the platform's perspective: emit a webhook when a Unit changes, let the orchestrator decide what to do. Orchestrator steps call back into your REST API to read/write Units and children.

This means:
- The platform is useful standalone (inline executor only)
- Adding orchestrator support is additive, not a rewrite
- Users can swap orchestrators without changing Unit definitions

---

## CV pitch

> "A data entity platform where Units own their schema, history, and heterogeneous children (DB, S3, linked entities) — with pluggable process execution via inline runners or external orchestrators like Dagu."

Demonstrates: event sourcing, storage abstraction, schema registry, polyglot runtimes, at-least-once delivery, orchestrator integration patterns.
