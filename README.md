# Lute

Lute is an attempt at an alternative to [Jenkins](https://github.com/jenkinsci/jenkins), fixing the
problems it has:

- Stability and predictability
- Configuration as Code out of the box
- Rich, easy-to-use form builders instead of tons of poorly supported plugins
- Stateless core
- Modern SDKs instead of legacy Groovy pipelines
- ... and many more

**[Website](https://notna-k.github.io/Lute/) · [Docs](https://notna-k.github.io/Lute/docs/) · [API reference](https://notna-k.github.io/Lute/docs/api/)**

> **Status: early work in progress.** Lute is a weekend project, it has no users yet and no stable
> release. Expect breaking changes without deprecation.

![The Lute panel — job definitions synced from Git](docs/media/jobs.png)

![The Lute panel — job definition editor](docs/media/job-definition.png)

## How it works

Three pieces, each one doing one thing:

```
browser ──▶ admin (nginx SPA) ──/api──▶ core (HTTP + WS) ──▶ postgres
workers ───────────────── gRPC ───────▶ core (:50051)
```

- **core** — stateless Go backend. REST + WebSocket for the panel, gRPC for workers. Everything
  durable (domain data *and* the job queue) lives in Postgres, so core can be restarted or scaled
  out freely.
- **admin** — the web panel. Trigger jobs, watch builds stream in, manage workers.
- **worker** — a container image that runs on rootless Docker. It enrols with a registration token,
  then runs jobs as sibling containers for the queues and labels it advertises
  ([Running a worker](https://notna-k.github.io/Lute/docs/workers/running/)).

**Git is the source of truth for job definitions.** A directory of YAML from your repository is
synced into Postgres on startup and on demand; the panel shows what drifted from Git and can export
the current state back as YAML to commit.

A job definition declares its queue, labels, runtime, command and a typed parameter schema — the
panel renders the run form from that schema, so there is no plugin to install for a select box or a
secret field:

```yaml
name: web-release
description: Builds and ships the marketing site.
queue: deploy
labels:
  region: eu
runtime: node:25-alpine
command: ./scripts/ship.sh
parameters:
  - name: environment
    type: select
    label: Target environment
    env: ENVIRONMENT
    required: true
    default: staging
    options:
      - { value: staging, label: Staging, tone: warning }
      - { value: prod, label: Production, tone: danger }
  - name: dry_run
    type: bool
    label: Dry run
    env: DRY_RUN
    default: true
```

## Quick start

All you need is Docker with Docker Compose. [`examples/`](examples/) has ready-to-run setups:

| Example | What it runs |
|---|---|
| [`quickstart/`](examples/quickstart/) | Everything on one machine, to try Lute out |
| [`server/`](examples/server/) | Postgres, core and the panel, for a real install |
| [`worker/`](examples/worker/) | A worker on a build machine with rootless Docker |

To try it, take the quickstart:

```bash
git clone https://github.com/notna-k/Lute.git
cd Lute/examples/quickstart
cp .env.example .env     # set JWT_SECRET (>= 32 bytes), ADMIN_EMAIL, ADMIN_PASSWORD
docker compose up -d
```

Open http://localhost:8080, sign in with `ADMIN_EMAIL` / `ADMIN_PASSWORD`, and run the `hello`
job. Its definition is in [`examples/quickstart/jobdefs/`](examples/quickstart/jobdefs/); add your
own YAML there and press **Sync from Git**.

For a real install, run [`server/`](examples/server/) on one machine and
[`worker/`](examples/worker/) on each build machine. [`examples/README.md`](examples/README.md)
walks through both, and the [docs](https://notna-k.github.io/Lute/docs/) cover
[job definitions](https://notna-k.github.io/Lute/docs/jobs/definitions/),
[workers](https://notna-k.github.io/Lute/docs/workers/running/) and
[every setting](https://notna-k.github.io/Lute/docs/configuration/).

## Contributing

Issues, ideas and pull requests are welcome. [CONTRIBUTING.md](CONTRIBUTING.md) covers the
development setup and tests.

## License

[MIT](LICENSE) © Anton Mahdysyuk
