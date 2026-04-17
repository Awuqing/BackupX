---
sidebar_position: 4
title: Configuration Reference
description: All server.yaml configuration keys with defaults and matching environment variables.
---

# Configuration Reference

BackupX loads `./config.yaml` from the working directory by default. You can override the path with `--config`. Every key can also be set via a `BACKUPX_` prefixed environment variable.

## Full config reference

```yaml title="config.yaml"
server:
  host: "0.0.0.0"             # BACKUPX_SERVER_HOST
  port: 8340                  # BACKUPX_SERVER_PORT
  mode: "release"             # release | debug

database:
  path: "./data/backupx.db"   # BACKUPX_DATABASE_PATH — embedded SQLite

security:
  jwt_secret: ""              # BACKUPX_SECURITY_JWT_SECRET — auto-generated if empty
  jwt_expires_in: "24h"
  encryption_key: ""          # AES-256-GCM key for storage config encryption

backup:
  temp_dir: "/tmp/backupx"    # BACKUPX_BACKUP_TEMP_DIR
  max_concurrent: 2           # BACKUPX_BACKUP_MAX_CONCURRENT
  retries: 3                  # Per-upload rclone low-level retries
  bandwidth_limit: ""         # e.g. "10M" to cap transfers at 10 MB/s

log:
  level: "info"               # debug | info | warn | error
  file: "./data/backupx.log"
```

## Secret generation

If `jwt_secret` or `encryption_key` is empty on first start, BackupX generates a random value and persists it to the `system_configs` table. Keep a backup of `data/backupx.db` — losing it invalidates all existing encrypted storage configurations.

## Environment variables

The environment wins when both file and env are set. All dot-paths become underscores and uppercase:

| Config key | Env variable |
|------------|--------------|
| `server.port` | `BACKUPX_SERVER_PORT` |
| `log.level` | `BACKUPX_LOG_LEVEL` |
| `backup.max_concurrent` | `BACKUPX_BACKUP_MAX_CONCURRENT` |
| `backup.temp_dir` | `BACKUPX_BACKUP_TEMP_DIR` |
| `backup.bandwidth_limit` | `BACKUPX_BACKUP_BANDWIDTH_LIMIT` |
