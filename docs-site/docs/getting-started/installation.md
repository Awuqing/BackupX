---
sidebar_position: 1
title: Installation
description: Install BackupX via Docker, prebuilt archive, or from source.
---

# Installation

BackupX ships as a single static binary. Three ways to install, pick the one that matches your environment.

## Docker (recommended)

Download the canonical hardened Compose file and start the service:

```bash
curl -fLO https://raw.githubusercontent.com/Awuqing/BackupX/main/docker-compose.yml
docker compose up -d
docker compose ps
```

The Compose definition enables init and graceful shutdown, persists `/app/data`, runs the application as an unprivileged user, drops unnecessary capabilities, and checks `/ready`. Images at [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) support `linux/amd64` and `linux/arm64`.

For production, create a protected `.env` and pin a release instead of relying on `latest`:

```dotenv
BACKUPX_IMAGE=awuqing/backupx:vX.Y.Z
BACKUPX_BIND_ADDRESS=127.0.0.1
TZ=Asia/Shanghai
```

Use the loopback binding when a reverse proxy runs on the same host. For direct access, choose the intended interface and enforce a firewall. Mount host backup sources read-only or deploy an Agent on the source host. See [Docker Deployment](../deployment/docker) for the full configuration.

## Prebuilt archive (bare metal)

Download from the [Releases page](https://github.com/Awuqing/BackupX/releases) and run the installer:

```bash
sha256sum -c backupx-v*-linux-amd64.tar.gz.sha256
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh        # creates system user, installs to /opt/backupx, sets up systemd
```

The installer:

1. Creates a `backupx` system user
2. Installs the binary to `/opt/backupx/bin/backupx` and the web console to `/opt/backupx/web`
3. Creates `/etc/backupx/config.yaml` with safe defaults
4. Installs and enables the `backupx.service` systemd unit
5. Leaves Nginx unchanged unless `INSTALL_NGINX=1` is explicitly requested
6. Waits for `/api/auth/setup/status`; if startup fails, prints systemd diagnostics and exits non-zero

## From source

Requires Go ≥ 1.25, Node.js 24 LTS, and npm 11 or later.

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
sudo ./deploy/install.sh
```

After `make build`, the binary is at `server/bin/backupx` and the built web UI is at `web/dist/`.
The installer consumes those exact paths, so no Docker runtime is required. If an existing configuration uses a non-default port, set `HEALTH_URL` for the readiness check, for example `sudo HEALTH_URL=http://127.0.0.1:9000/api/auth/setup/status ./deploy/install.sh`.

The Nginx template is opt-in because automatically installing a catch-all virtual host can intercept existing sites. Review `deploy/nginx.conf`, then use `sudo INSTALL_NGINX=1 ./deploy/install.sh` only when it matches the host.

## Verify the install

```bash
/opt/backupx/bin/backupx --version
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
```

Then open `http://your-server:8340`. Choose **English** or **中文** in the upper-right corner. A fresh database shows **System setup**, where you create the first administrator username and password. If that form does not appear, retry the status request above before attempting to sign in.
