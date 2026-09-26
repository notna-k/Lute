# Running a Lute worker

A worker is the `lute-worker` image running on a Linux host with **rootless Docker**. It dials
core over gRPC, runs each job as a sibling container on the same engine, and keeps its identity
and job logs in a host directory you mount.

Everything runs as one unprivileged user. Rootless Docker maps root in a container to that user,
so a leaked socket or an escaped job gets that user's rights, not root's.

## 1. Host setup (once, as root)

Debian and Ubuntu shown; other distros differ only in the package manager.

```bash
sudo apt-get install -y uidmap docker-ce-rootless-extras
sudo useradd -m ci && sudo loginctl enable-linger ci

# Let the user's containers use CPU, memory and pid limits (cgroup v2 delegation).
sudo mkdir -p /etc/systemd/system/user@.service.d
printf '[Service]\nDelegate=cpu cpuset io memory pids\n' | sudo tee /etc/systemd/system/user@.service.d/delegate.conf
sudo systemctl daemon-reload
```

Ubuntu 23.10 and later restrict unprivileged user namespaces with AppArmor; follow Docker's
rootless docs to add the `rootlesskit` profile.

## 2. Rootless Docker (as the `ci` user)

Log in with a real session (`ssh ci@host` or `sudo machinectl shell ci@`); `systemctl --user`
does not work under `sudo -u`.

```bash
dockerd-rootless-setuptool.sh install
systemctl --user enable --now docker
mkdir -p ~/.local/share/lute-worker
```

## 3. Run the worker

In the panel, open **Workers → Add worker**, pick or create a registration token, and copy the
command. It looks like this:

```bash
docker run -d --name lute-worker --restart unless-stopped \
  --stop-timeout 1800 \
  -v "$XDG_RUNTIME_DIR/docker.sock:/var/run/docker.sock" \
  -v "$HOME/.local/share/lute-worker:/var/lib/lute-worker" \
  -e LUTE_SERVER=lute.example.com:50051 \
  -e LUTE_TOKEN=lute_rt_... \
  ghcr.io/notna-k/lute-worker:0.2
```

On its first start the worker registers with the token and writes `state.json` into the data
dir. After that the token is not needed: restarts, reboots and image updates keep the same
worker. Revoking the token stops new registrations only.

Core's gRPC port speaks plaintext. Across any network you do not trust, put a TLS-terminating
proxy in front of it and add `-e LUTE_TLS=1`, or the token and the worker secret travel in the clear.

The name defaults to the engine's host name; a second worker on the same host needs
`-e LUTE_NAME=...`, since names are unique across Lute.

## Settings

| Variable | Default | Meaning |
|---|---|---|
| `LUTE_SERVER` | required | core's gRPC address, `host:port` |
| `LUTE_TLS` | `0` | dial core with TLS; set it when a proxy terminates TLS in front of core |
| `LUTE_TOKEN` | first start only | registration token |
| `LUTE_NAME` | engine host name | worker name |
| `LUTE_QUEUES` | `default` | comma-separated queues |
| `LUTE_LABELS` | empty | `k=v,k=v`, sent at registration |
| `LUTE_CONCURRENCY` | `4` | jobs at once |
| `LUTE_DATA_DIR` | `/var/lib/lute-worker` | identity, job logs, job metadata |
| `LUTE_REQUIRE_MOUNT` | `1` in the image | exit unless the data dir is a mount point |
| `LUTE_ALLOW_ROOTFUL` | `0` | allow a rootful engine (dev, CI) |
| `LUTE_DRAIN_TIMEOUT` | `30m` | how long a stop waits for running jobs |
| `LUTE_JOB_MEMORY` / `LUTE_JOB_CPUS` | unset | per-job limits, e.g. `2g` / `1.5` |
| `LUTE_GIT_IMAGE` | `alpine/git:2.52.0` | image that clones repositories |
| `LUTE_LOG_RETENTION` | `720h` | job logs older than this are pruned |

Every variable also has a flag: `lute-worker run -h`.

## The data dir

```
~/.local/share/lute-worker/
├── .lute-data          marker; the worker refuses a non-empty dir without it
├── state.json          worker id and secret (0600)
└── jobs/<id>/
    ├── log             the job's output
    └── meta.json       image and digest, repository and commit, exit code, times
```

Delete `state.json` to make the worker forget its identity; it registers again on its next start,
which needs `LUTE_TOKEN` and a free name.

## Updating

```bash
docker pull ghcr.io/notna-k/lute-worker:0.2
docker rm -f lute-worker   # after `docker stop`, see below
docker run ...             # the same command
```

or `docker compose pull && docker compose up -d`. Stopping the container sends SIGTERM: the worker
stops taking jobs, finishes the running ones (up to `LUTE_DRAIN_TIMEOUT`), and exits. A second
SIGTERM cancels them. Keep `--stop-timeout` at least as long as the drain timeout.

The panel marks a worker *outdated* when it is older than core. Core refuses an agent whose
protocol is too old and names the image to pull.

## Deleting a worker

**Delete** in the panel lets the worker finish its running jobs, then stops its container. The
container, `state.json` and the job logs stay where they are. A worker deleted while offline stops
itself the next time it starts.

## Troubleshooting

`docker logs lute-worker` says why it stopped. Exit code 78 means something needs fixing; Docker
keeps retrying with growing delays meanwhile.

| Message | Fix |
|---|---|
| `/var/lib/lute-worker is not mounted` | add the `-v ~/.local/share/lute-worker:/var/lib/lute-worker` mount |
| `holds other files but no .lute-data marker` | mount an empty directory, not `$HOME` |
| `the Docker engine is rootful` | mount the rootless socket (`$XDG_RUNTIME_DIR/docker.sock`) |
| `cannot enforce memory, cpu limits` | set up cgroup delegation (step 1) or unset the job limits |
| `registration token is invalid or revoked` | create a token in the panel |
| `a worker named ... already exists` | set `LUTE_NAME` |
| `worker was deleted` | delete `state.json` and start it with a token to register again |
