---
sidebar_position: 1
title: Installation
description: Install BackupX via Docker, prebuilt archive, or from source.
---

# Installation

BackupX ships as a single static binary. Three ways to install, pick the one that matches your environment.

## Docker (recommended)

No cloning required.

```bash
docker run -d --name backupx \
  -p 8340:8340 \
  -v backupx-data:/app/data \
  awuqing/backupx:latest
```

Or use `docker compose`:

```yaml title="docker-compose.yml"
services:
  backupx:
    image: awuqing/backupx:latest
    container_name: backupx
    restart: unless-stopped
    ports:
      - "8340:8340"
    volumes:
      - backupx-data:/app/data
      # Mount host directories to back up (as needed):
      # - /var/www:/mnt/www:ro
      # - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai

volumes:
  backupx-data:
```

Images: [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) — supports `linux/amd64` and `linux/arm64`.

## Prebuilt archive (bare metal)

Download from the [Releases page](https://github.com/Awuqing/BackupX/releases) and run the installer:

```bash
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh        # creates system user, installs to /opt/backupx, sets up systemd + nginx
```

The installer:

1. Creates a `backupx` system user
2. Installs the binary to `/opt/backupx/bin/backupx` and the web console to `/opt/backupx/web`
3. Creates `/etc/backupx/config.yaml` with safe defaults
4. Installs and enables the `backupx.service` systemd unit
5. (Optional) Configures an Nginx reverse proxy
6. Waits for `/api/auth/setup/status`; if startup fails, prints systemd diagnostics and exits non-zero

## From source

Requires Go ≥ 1.25 and Node.js ≥ 20.

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
sudo ./deploy/install.sh
```

After `make build`, the binary is at `server/bin/backupx` and the built web UI is at `web/dist/`.
The installer consumes those exact paths, so no Docker runtime is required. If an existing configuration uses a non-default port, set `HEALTH_URL` for the readiness check, for example `sudo HEALTH_URL=http://127.0.0.1:9000/api/auth/setup/status ./deploy/install.sh`.

## Verify the install

```bash
/opt/backupx/bin/backupx --version
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
```

Then open `http://your-server:8340`. Choose **English** or **中文** in the upper-right corner. A fresh database shows **System setup**, where you create the first administrator username and password. If that form does not appear, retry the status request above before attempting to sign in.
