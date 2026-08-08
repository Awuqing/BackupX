---
sidebar_position: 1
title: Docker Deployment
description: Hardened single-process Docker deployment with health checks and persistent data.
---

# Docker Deployment

The official [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) image supports `linux/amd64` and `linux/arm64`.

## Compose file

```yaml title="docker-compose.yml"
services:
  backupx:
    image: ${BACKUPX_IMAGE:-awuqing/backupx:latest}
    container_name: backupx
    restart: unless-stopped
    init: true
    stop_grace_period: 30s
    ports:
      - "${BACKUPX_BIND_ADDRESS:-0.0.0.0}:${BACKUPX_PORT:-8340}:8340"
    volumes:
      - backupx-data:/app/data
      # - /var/www:/mnt/www:ro
      # - /etc/nginx:/mnt/nginx-conf:ro
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - DAC_OVERRIDE
      - SETGID
      - SETUID
    environment:
      TZ: Asia/Shanghai
      # BACKUPX_SERVER_EXTERNAL_URL: https://backup.example.com
      BACKUPX_LOG_LEVEL: info
      BACKUPX_BACKUP_MAX_CONCURRENT: "2"
    healthcheck:
      test: ["CMD", "su-exec", "backupx:backupx", "wget", "-q", "-T", "3", "-O", "/dev/null", "http://127.0.0.1:8340/ready"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s

volumes:
  backupx-data:
```

```bash
docker compose up -d
docker compose ps
```

The entrypoint uses root only to migrate ownership of data written by older images, then starts one unprivileged `backupx` process. Compose retains only the ownership and UID/GID transition capabilities needed for that initialization. The backend serves both the API and built web assets; the image neither mounts the Docker socket nor bundles a Docker CLI. Pin `BACKUPX_IMAGE` to a release tag in production.

## Host-directory backups

Mount each source directory and use its container path in the task. The container's `backupx` user must be able to read it; restore destinations need a separate, narrowly scoped writable mount. Prefer a remote Agent for privileged host paths. If a Master-side task truly requires root, make that exception explicit with `user: "0:0"` and review every mount.

## Multi-node cluster

Set the stable URL that Agents can reach:

```yaml
environment:
  BACKUPX_SERVER_EXTERNAL_URL: https://backup.example.com
```

Use HTTPS across untrusted networks. Proxy, private-CA, and SSH-bastion deployments are covered in [Multi-Node Cluster](../features/multi-node).

If an external reverse proxy is in another container, add only its bridge subnet to `BACKUPX_SERVER_TRUSTED_PROXIES`, for example `172.18.0.0/16`. Do not trust every address.

## Environment overrides

```yaml
environment:
  TZ: Asia/Shanghai
  BACKUPX_LOG_LEVEL: debug
  BACKUPX_BACKUP_MAX_CONCURRENT: "4"
  BACKUPX_BACKUP_TEMP_DIR: /tmp/backupx
```

The image's internal port is fixed at `8340`; change only the published host port with `BACKUPX_PORT`.

## Upgrade and rollback preparation

```bash
docker compose pull
docker compose up -d
docker compose ps
```

Wait for `healthy` before switching traffic or removing an old deployment. Before upgrades, stop the Master for a file-level copy or take an atomic snapshot of the entire `backupx-data` volume. Keep exactly one active Master for a data volume; SQLite does not support multiple Master containers sharing `/app/data`.
