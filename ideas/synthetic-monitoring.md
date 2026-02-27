# Idea: Self-hosted Synthetic Monitoring Platform

## One-liner

Deploy agents inside your infrastructure to monitor internal and external services — checks run from within your network, no traffic leaves, no tunneling required.

## The problem

SaaS monitoring tools (Checkly, Uptime Robot, Better Uptime) cannot reach services running inside a private network — behind a VPN, inside a Kubernetes cluster, or on an internal subnet. Their workarounds (tunnels, static IPs) are fragile and complex.

Lute agents deploy alongside your services. They run checks from inside your infrastructure and stream results back to the backend over a persistent gRPC connection.

## Gap vs existing tools

| Tool | Can reach private network | Self-hosted | Quorum alerting |
|---|---|---|---|
| Checkly | No (tunnel workaround) | No | No |
| Uptime Robot | No | No | No |
| Uptime Kuma | No (single process) | Yes | No |
| **Lute** | Yes | Yes | Yes |

## How it works

1. Deploy the backend (Docker / Kubernetes)
2. Install agents inside your infrastructure — each agent self-registers and maintains a bidirectional gRPC stream to the backend
3. Define checks in the UI: URL, method, headers, expected status code, expected response body
4. Agents execute checks on a schedule and stream results back
5. Backend aggregates results, stores history, and raises an incident when a configured threshold of agents report failure

## Quorum alerting

Each check can be assigned to multiple agents. An incident is opened only when at least `threshold` agents report failure within the same check window. This prevents a single unreliable agent or a localised network issue from generating noise.

```
Agent A: PASS  ─┐
Agent B: FAIL  ─┼─► 1 of 3 failed → no incident
Agent C: PASS  ─┘

Agent A: FAIL  ─┐
Agent B: FAIL  ─┼─► 2 of 3 failed → incident opened  (threshold = 2)
Agent C: PASS  ─┘
```

## Architecture

```
┌─────────┐        REST / WebSocket        ┌─────────────┐
│   UI    │ ─────────────────────────────► │             │
└─────────┘                                │   Backend   │
                                           │  (Go + Gin) │
┌─────────┐   bidirectional gRPC stream    │             │
│ Agent A │ ◄────────────────────────────► │             │
└─────────┘                                └──────┬──────┘
                                                  │
┌─────────┐   bidirectional gRPC stream    ┌──────▼──────┐
│ Agent B │ ◄────────────────────────────► │   MongoDB   │
└─────────┘                                └─────────────┘

┌─────────┐   bidirectional gRPC stream
│ Agent N │ ◄────────────────────────────►
└─────────┘
```

- **Backend** — Go, Gin (REST), gRPC server, MongoDB
- **Agent** — lightweight Go binary, connects outbound only, zero inbound firewall rules needed
- **UI** — React + Vite, Firebase auth

## Features (MVP)

- [ ] HTTP checks — status code, response body match, latency threshold
- [ ] Configurable check interval per check
- [ ] Multi-agent assignment per check + quorum threshold
- [ ] Uptime history and latency time series in UI
- [ ] Incident open/resolve lifecycle
- [ ] Agent self-registration via claim code

## Roadmap (post-MVP)

- [ ] TCP and DNS check types
- [ ] Webhook and email alerting
- [ ] Check groups and environments
- [ ] Kubernetes operator for agent deployment
- [ ] Public status page

## Distributed systems concepts involved

- **Coordinator-worker** — backend dispatches check jobs to agents via gRPC stream
- **Quorum-based decision** — incident raised only on majority agreement
- **Churn tolerance** — agents reconnect with exponential backoff; checks reassigned if an agent goes offline
- **Streaming aggregation** — results flow back in real time without polling
