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
2. Installs binary to `/opt/backupx/backupx`
3. Creates `/opt/backupx/config.yaml` with safe defaults
4. Installs and enables the `backupx.service` systemd unit
5. (Optional) Configures an Nginx reverse proxy

## From source

Requires Go ≥ 1.25 and Node.js ≥ 20.

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
# or, for builds behind the great firewall
make docker-cn
```

After `make build`, the binary is at `server/bin/backupx` and the built web UI is at `web/dist/`.

## Verify the install

```bash
backupx --version           # e.g. v1.6.0
```

Then open `http://your-server:8340` to see the initial admin setup screen.
