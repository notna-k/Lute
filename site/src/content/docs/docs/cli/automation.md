---
title: Scripts, CI and LLMs
description: The output contract scripts rely on, and how to use lute in CI.
---

Scripts and LLMs depend on how `lute` reports results, so the contract below is versioned with the
CLI and covered by its tests.

## In CI

Give CI a **service key**: it belongs to the Lute instance, not to a person, so it keeps working when
whoever set up the pipeline leaves. Set two variables and run; no `lute login`, nothing on disk.

```yaml
# GitHub Actions
- name: Deploy to staging
  env:
    LUTE_URL: https://ci.acme.dev
    LUTE_API_KEY: ${{ secrets.LUTE_API_KEY }}
  run: |
    npx @notna-k/lute-cli run web-release \
      -p environment=staging \
      --idempotency-key "deploy-${{ github.sha }}" \
      --follow
```

`--follow` fails the step when the run fails. `--idempotency-key` makes a re-run of the workflow
pick up the run it already started instead of starting another.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. With `--wait` or `--follow`: the run passed. |
| 1 | With `--wait` or `--follow`: the run failed, timed out or was cancelled. |
| 2 | Bad usage or rejected input: an unknown flag, an invalid parameter, an action that is not possible, plain HTTP without consent. |
| 3 | No URL or key configured, or the key is invalid or revoked. |
| 4 | Not found: the job or the run. |
| 5 | Lute is unreachable or answered with a server error. Safe to retry. |
| 130 | Interrupted with Ctrl-C. |

## Output

- `--json` prints exactly one JSON document to stdout, using the [public API's](../../api/) field
  names. `lute run --wait --json` prints the finished run.
- `logs --follow --json` and `run --follow --json` print NDJSON: one `{"line": "…"}` per log line,
  then a final `{"run": {…}}`.
- Errors go to stderr. With `--json` they use the API's error shape,
  `{"error": {"code", "message", "fields"}}`, and a rejected parameter is named in `fields`.
- Warnings, progress and hints go to stderr, so stdout carries only the result.
- Without a terminal there is no colour, no spinner and no prompt. `NO_COLOR` is respected.

```bash
$ lute jobs show web-release --json | jq -r '.parameters[].name'
environment
regions
dry_run

$ lute run web-release -p environment=prod -p dry_run=true --wait --json
{
  "id": "66fb1c2d3e4f5a6b7c8d9e0f",
  "job": "web-release",
  "status": "failed",
  …
}
$ echo $?
1
```

## LLM agents

An agent with a shell can drive Lute through `lute` alone: `lute jobs --json` to find work,
`lute jobs show <slug> --json` to learn the parameters, `lute run … --wait --json` to run it, and
the exit code to know what happened. Give it a service key of its own, so its runs are easy to tell
apart and the key is easy to revoke.
