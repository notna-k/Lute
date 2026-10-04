---
title: Commands
description: Every lute command and its flags.
---

Run IDs are 24 hex characters, but `lute` prints the short form, `#a1b2c3d4`, and every command
accepts it, with or without the `#`.

## Global flags

| Flag | Effect |
|---|---|
| `--json` | One JSON document on stdout, in the public API's field names. Errors go to stderr as JSON. |
| `--url URL` | Talk to this Lute for one command. Same as `LUTE_URL`. |
| `--allow-insecure-http` | Send the key to a plain `http://` URL anyway. |
| `--debug` | Print each request and its status to stderr, with the key redacted. |
| `-h`, `--help` | Help for `lute` or for one command. |

## Jobs

```bash
lute jobs                    # slug, queue and last run of every job
lute jobs show web-release   # description, runtime, command and parameters
```

`lute jobs show --json` returns the parameter schema: names, types, defaults and options. That is
how a script or an LLM learns what to pass.

## Run a job

```bash
lute run web-release -p environment=staging -p regions=eu-central,us-east --follow
```

| Flag | Effect |
|---|---|
| `-p name=value` | A parameter; repeat it. A list takes `a,b`. `name=@path` reads the value from a file. |
| `--wait` | Block until the run ends. Exits 1 if it failed. |
| `--follow` | Like `--wait`, and stream the log as it grows. |
| `--idempotency-key KEY` | Repeat the command with the same key and you get the first run back, not a second one. |

Core validates every value, as the panel's run form does. A name the job does not have is refused
before anything starts. On a terminal, `lute` asks for required values you left out; without one,
the run is refused and the error names each missing parameter.

## Runs

```bash
lute runs                                # newest first
lute runs --job web-release --status failed --limit 5
lute runs show a1b2c3d4                  # status, timing, worker, parameters, panel link
lute logs a1b2c3d4                       # the whole log
lute logs a1b2c3d4 --tail 50             # the last 50 lines
lute logs a1b2c3d4 -f                    # follow until the run ends
lute cancel a1b2c3d4
lute retry a1b2c3d4                      # queue it again with the same parameters
```

`--status` takes `pending`, `running`, `done`, `failed`, `dead` or `unknown`. `--limit` and
`--offset` page through the list.

Ctrl-C stops following a log; it never cancels the run.

`lute cancel` only cancels a run no worker has taken yet. A run that already started cannot be
stopped yet, so `lute cancel` exits with code 2 and says so.

## Account

```bash
lute login [--url URL] [--api-key-stdin] [--allow-insecure-http] [--insecure-storage]
lute logout
lute whoami
lute version
```

See [Install and connect](../) for what each one keeps where.
