---
title: Job definitions
description: The YAML that describes a job — where it runs, what it runs, and what it asks for.
---

A job definition says where a job runs, what it runs, and which inputs its run form asks for. It lives
in Git as YAML; core [syncs it](../git-sync/) into Postgres.

```yaml
name: web-release
description: Builds and ships the marketing site.
queue: deploy
labels:
  region: eu
runtime: node:25-alpine
command: ./scripts/ship.sh
source:
  repo: https://github.com/acme/site.git
  commit: a91f0c
parameters:
  - name: environment
    type: select
    label: Target environment
    required: true
    default: staging
    options:
      - { value: staging, label: Staging, tone: warning }
      - { value: prod, label: Production, tone: danger }
  - name: dry_run
    type: bool
    label: Dry run
    default: true
```

One file may hold several definitions, separated by `---`.

## Keys

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | Display name. |
| `slug` | no | Stable id used in URLs and the API. Derived from `name` when left out. |
| `description` | no | Shown under the name in the panel. |
| `queue` | no | Queue the job goes on. Defaults to `default`. Only workers serving it pick the job up. |
| `labels` | no | Labels a worker must have to take the job, e.g. `region: eu`. |
| `runtime` | yes | Container image the command runs in. |
| `command` | yes | Shell command run inside the image. |
| `source.repo` | no | Git repository cloned into the job's workspace before the command runs. |
| `source.commit` | no | Commit recorded with the definition. |
| `parameters` | no | Inputs of the run form. See [Parameters](../parameters/). |

## Where the job runs

A run goes to the first worker that serves its `queue` and has every one of its `labels`. Workers
declare their queues with `LUTE_QUEUES` and their labels with `LUTE_LABELS`, or you set labels
later in the panel. See [Running a worker](../../workers/running/).

## What the command sees

Each parameter becomes an environment variable, named by its `env` key. The `runtime` image runs as
a sibling container on the worker's Docker engine, with the repository from `source.repo` checked
out into its workspace.
