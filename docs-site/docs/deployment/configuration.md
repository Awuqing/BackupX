---
sidebar_position: 4
title: Configuration Reference
description: All config.yaml server keys with defaults and matching environment variables.
---

# Configuration Reference

BackupX loads `./config.yaml` from the working directory by default. You can override the path with `--config`. Every key can also be set via a `BACKUPX_` prefixed environment variable.

## Full config reference

```yaml title="config.yaml"
server:
  host: "0.0.0.0"             # BACKUPX_SERVER_HOST
  port: 8340                  # BACKUPX_SERVER_PORT
  mode: "release"             # release | debug
  external_url: ""            # BACKUPX_SERVER_EXTERNAL_URL — stable public Master URL
  trusted_proxies:             # BACKUPX_SERVER_TRUSTED_PROXIES — exact proxy IPs/CIDRs
    - "127.0.0.1"
    - "::1"
  web_root: ""                # BACKUPX_SERVER_WEB_ROOT — built frontend directory

database:
  path: "./data/backupx.db"   # BACKUPX_DATABASE_PATH — embedded SQLite

security:
  jwt_secret: ""              # BACKUPX_SECURITY_JWT_SECRET — auto-generated if empty
  jwt_expire: "24h"           # BACKUPX_SECURITY_JWT_EXPIRE
  encryption_key: ""          # AES-256-GCM key for storage config encryption

backup:
  temp_dir: "/tmp/backupx"    # BACKUPX_BACKUP_TEMP_DIR
  max_concurrent: 2           # BACKUPX_BACKUP_MAX_CONCURRENT
  retries: 10                 # Per-upload rclone low-level retries
  bandwidth_limit: ""         # e.g. "10M" to cap transfers at 10 MB/s

log:
  level: "info"               # debug | info | warn | error
  file: "./data/backupx.log"
  max_size: 100               # MB per log file
  max_backups: 3              # rotated files retained
  max_age: 30                 # retention in days
```

## Secret generation

If `jwt_secret` or `encryption_key` is empty on first start, BackupX generates a random value and persists it to the `system_configs` table. Keep a backup of `data/backupx.db` — losing it invalidates all existing encrypted storage configurations.

## Environment variables

The environment wins when both file and env are set. All dot-paths become underscores and uppercase:

| Config key | Env variable |
|------------|--------------|
| `server.port` | `BACKUPX_SERVER_PORT` |
| `server.external_url` | `BACKUPX_SERVER_EXTERNAL_URL` |
| `server.trusted_proxies` | `BACKUPX_SERVER_TRUSTED_PROXIES` (comma-separated for env) |
| `security.jwt_secret` | `BACKUPX_SECURITY_JWT_SECRET` |
| `security.jwt_expire` | `BACKUPX_SECURITY_JWT_EXPIRE` |
| `security.encryption_key` | `BACKUPX_SECURITY_ENCRYPTION_KEY` |
| `log.level` | `BACKUPX_LOG_LEVEL` |
| `backup.max_concurrent` | `BACKUPX_BACKUP_MAX_CONCURRENT` |
| `backup.temp_dir` | `BACKUPX_BACKUP_TEMP_DIR` |
| `backup.retries` | `BACKUPX_BACKUP_RETRIES` |
| `backup.bandwidth_limit` | `BACKUPX_BACKUP_BANDWIDTH_LIMIT` |
| `log.max_size` | `BACKUPX_LOG_MAX_SIZE` |
| `log.max_backups` | `BACKUPX_LOG_MAX_BACKUPS` |
| `log.max_age` | `BACKUPX_LOG_MAX_AGE` |

## Master external URL

Set `server.external_url` when BackupX is behind Docker, Nginx, a load balancer, or any reverse proxy whose internal Host is not reachable by remote Agents:

```yaml
server:
  external_url: "https://backup.example.com"
```

This value is used when BackupX renders one-click Agent install scripts and docker-compose snippets. It must be reachable from every Agent host. Leave it empty only when `X-Forwarded-Proto` / `X-Forwarded-Host` are reliable and point to the same URL that Agents can access.

The install wizard can set an Agent-specific URL for a proxy or SSH-bastion node. That override is used by both the target-side one-time install URL and the generated Agent runtime configuration, while the browser continues to use the normal public address.

## Trusted reverse proxies

BackupX trusts forwarded client-address headers only from `server.trusted_proxies`. The default permits loopback Nginx only. If a reverse proxy runs in another container or host, add its exact IP or subnet:

```yaml
server:
  trusted_proxies:
    - "127.0.0.1"
    - "172.18.0.0/16"
```

Do not configure `0.0.0.0/0`: client addresses feed authentication throttling, install-token throttling, and audit records. Set an empty list when BackupX is exposed directly and should trust no forwarded headers.

Back up the complete data directory and configuration before changing security keys or database paths. See [Upgrade and Recovery](../operations/upgrade-recovery) for a tested snapshot and rollback sequence.
