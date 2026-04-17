---
sidebar_position: 1
title: Docker Deployment
description: Production-style Docker deployment with docker compose, mounted source directories, and environment overrides.
---

# Docker Deployment

BackupX's official Docker image [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) supports multi-architecture (linux/amd64 + linux/arm64).

## Compose file

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
      # Mount host directories you want to back up:
      - /var/www:/mnt/www:ro
      - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai
      - BACKUPX_LOG_LEVEL=info
      - BACKUPX_BACKUP_MAX_CONCURRENT=2

volumes:
  backupx-data:
```

Start with:

```bash
docker compose up -d
```

## Host-directory backup

To back up files from the host, mount them into the container. When creating a file-type task in the web UI, point the source path at the mount location (e.g. `/mnt/www`). Make sure the directory is visible inside the container.

## Environment variables

All configuration keys can be overridden with the `BACKUPX_` prefix:

```yaml
environment:
  - TZ=Asia/Shanghai
  - BACKUPX_SERVER_PORT=8340
  - BACKUPX_LOG_LEVEL=debug
  - BACKUPX_BACKUP_MAX_CONCURRENT=4
  - BACKUPX_BACKUP_TEMP_DIR=/tmp/backupx
```

See the [Configuration](./configuration) page for the full list.

## Upgrades

Check **System Settings → Check Updates** in the UI to see if a new version is available, then on the host:

```bash
docker compose pull && docker compose up -d
```

No migrations needed — BackupX auto-migrates the SQLite schema on startup.
