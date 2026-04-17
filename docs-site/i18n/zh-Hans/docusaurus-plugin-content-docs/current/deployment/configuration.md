---
sidebar_position: 4
title: 配置参考
description: server.yaml 所有配置项及对应的环境变量。
---

# 配置参考

BackupX 默认从工作目录加载 `./config.yaml`，可通过 `--config` 指定其他路径。所有配置项都可通过 `BACKUPX_` 前缀环境变量覆盖。

## 完整配置

```yaml title="config.yaml"
server:
  host: "0.0.0.0"             # BACKUPX_SERVER_HOST
  port: 8340                  # BACKUPX_SERVER_PORT
  mode: "release"             # release | debug

database:
  path: "./data/backupx.db"   # BACKUPX_DATABASE_PATH — 内嵌 SQLite

security:
  jwt_secret: ""              # BACKUPX_SECURITY_JWT_SECRET — 留空自动生成
  jwt_expires_in: "24h"
  encryption_key: ""          # 用于加密存储配置的 AES-256-GCM 密钥

backup:
  temp_dir: "/tmp/backupx"    # BACKUPX_BACKUP_TEMP_DIR
  max_concurrent: 2           # BACKUPX_BACKUP_MAX_CONCURRENT
  retries: 3                  # 单次上传的 rclone 底层重试次数
  bandwidth_limit: ""         # 例如 "10M" 表示限速 10 MB/s

log:
  level: "info"               # debug | info | warn | error
  file: "./data/backupx.log"
```

## 密钥生成

如果首次启动时 `jwt_secret` 或 `encryption_key` 为空，BackupX 会自动生成随机值并写入 `system_configs` 表。请妥善备份 `data/backupx.db`，一旦丢失将导致所有已加密的存储配置失效。

## 环境变量

文件和环境变量同时存在时，环境变量优先。配置路径转换规则：小写字母下划线 → 大写字母下划线：

| 配置项 | 环境变量 |
|--------|----------|
| `server.port` | `BACKUPX_SERVER_PORT` |
| `log.level` | `BACKUPX_LOG_LEVEL` |
| `backup.max_concurrent` | `BACKUPX_BACKUP_MAX_CONCURRENT` |
| `backup.temp_dir` | `BACKUPX_BACKUP_TEMP_DIR` |
| `backup.bandwidth_limit` | `BACKUPX_BACKUP_BANDWIDTH_LIMIT` |
