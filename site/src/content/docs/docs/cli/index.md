---
title: The lute CLI
description: Install lute, connect it to your Lute with an API key, and run a job from a terminal.
sidebar:
  label: Install and connect
---

`lute` finds a job, runs it with typed parameters, follows its log and tells you how it went. The
same commands serve a person at a terminal and a script, CI system or LLM: with a terminal you get
colour, prompts and a live log; with `--json`, or without a terminal, you get plain machine output
and nothing waits for input.

```bash
$ lute run web-release -p environment=staging --follow
✓ Queued run #a1b2c3d4 on queue deploy
  https://ci.acme.dev/jobs/web-release/builds/a1b2c3d4
> ship@1.0.0 build
vite v7 building for production...
✓ built in 4.12s
✓ Run #a1b2c3d4 passed in 38s
```

## Install

`lute` needs Node.js 22.18 or newer.

```bash
npm i -g @notna-k/lute-cli
```

In CI, run it without a global install: `npx @notna-k/lute-cli runs`.

## Connect

`lute login` asks for the URL you open the Lute panel at, and an API key. It checks the key before
saving anything, so a wrong URL or a revoked key fails right there.

```bash
$ lute login
? Lute URL  https://ci.acme.dev
? API key   ••••••••••••••••••••••
✓ Connected to Lute 0.9.0 as anton@acme.dev
  account key "laptop" · saved to the system keychain
```

Create the key in the panel under **Settings → API keys**. The key goes into the operating system's
keychain: macOS Keychain, Secret Service on Linux, or Windows Credential Manager.
`~/.config/lute/config.json` holds only the URL. No command, flag or `--debug` output prints the
key, and there is no `--api-key` flag, because a key on the command line lands in shell history and
`ps`. To log in from a script, pipe the key in:

```bash
lute login --url https://ci.acme.dev --api-key-stdin < key.txt
```

On a machine with no keychain, such as a headless Linux box, `lute login` stops and explains. Pass
`--insecure-storage` to keep the key in a `0600` file instead, or skip the login altogether and set
`LUTE_URL` and `LUTE_API_KEY`, which leaves nothing on disk.

`lute logout` removes the URL and key from this machine. The key keeps working until you revoke it
in the panel.

:::caution[What the keychain does not stop]
Any process running as you can read your keychain, and that includes an AI agent with a shell. Keeping
the key out of transcripts, logs, history and config files stops it leaking by accident; it does not
stop an agent that sets out to read it. Give an agent a narrow service key, and revoke it when you are
done.
:::

## Plain HTTP

For an `http://` URL, `lute login` shows a warning and asks `Continue over plain HTTP? [y/N]` before
it asks for the key. Only `y` continues. Your answer is saved with the URL, and every later command
prints a one-line `⚠ plain HTTP` notice to stderr.

Without a terminal, or with `LUTE_URL`, a plain-HTTP URL makes every command exit with code 2 before
it sends anything, unless you pass `--allow-insecure-http`. The flag allows exactly that: it does
not turn off certificate checks for HTTPS, and nothing does.

`localhost`, `127.0.0.1` and `::1` are exempt, because the key never leaves the machine.

## Account keys and service keys

| | Account key | Service key |
|---|---|---|
| Acts as | You | Itself |
| For | Your laptop and your own scripts | CI, cron boxes, agents |
| Who sees it | Only you | Everyone signed in to the panel |
| Runs it starts | Are yours | Show the key's name |
| When you leave | Stops working | Keeps working |

`lute whoami` shows the URL, the key's name and kind, and who it acts as.

## Server and versions

`--url` or `LUTE_URL` points one command at another Lute; a saved key is only ever sent to the URL
it was saved for. `lute version` prints the client and server versions, and warns when the server
speaks a different API level than this `lute` was built for.
