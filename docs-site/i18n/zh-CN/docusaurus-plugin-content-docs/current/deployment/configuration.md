---
sidebar_position: 4
title: 配置参考
description: config.yaml 全部服务端配置项及对应的环境变量。
---

# 配置参考

BackupX 默认从工作目录加载 `./config.yaml`，可通过 `--config` 指定其他路径。所有配置项都可通过 `BACKUPX_` 前缀环境变量覆盖。

## 完整配置

```yaml title="config.yaml"
server:
  host: "0.0.0.0"             # BACKUPX_SERVER_HOST
  port: 8340                  # BACKUPX_SERVER_PORT
  mode: "release"             # release | debug
  external_url: ""            # BACKUPX_SERVER_EXTERNAL_URL — 稳定的 Master 对外 URL
  trusted_proxies:             # BACKUPX_SERVER_TRUSTED_PROXIES — 准确的代理 IP/CIDR
    - "127.0.0.1"
    - "::1"
  web_root: ""                # BACKUPX_SERVER_WEB_ROOT — 前端构建目录

database:
  path: "./data/backupx.db"   # BACKUPX_DATABASE_PATH — 内嵌 SQLite

security:
  jwt_secret: ""              # BACKUPX_SECURITY_JWT_SECRET — 留空自动生成
  jwt_expire: "24h"           # BACKUPX_SECURITY_JWT_EXPIRE
  encryption_key: ""          # 用于加密存储配置的 AES-256-GCM 密钥

backup:
  temp_dir: "/tmp/backupx"    # BACKUPX_BACKUP_TEMP_DIR
  max_concurrent: 2           # BACKUPX_BACKUP_MAX_CONCURRENT
  retries: 10                 # 单次上传的 rclone 底层重试次数
  bandwidth_limit: ""         # 例如 "10M" 表示限速 10 MB/s

log:
  level: "info"               # debug | info | warn | error
  file: "./data/backupx.log"
  max_size: 100               # 单个日志文件上限，单位 MB
  max_backups: 3              # 保留的轮转文件数
  max_age: 30                 # 保留天数
```

## 密钥生成

如果首次启动时 `jwt_secret` 或 `encryption_key` 为空，BackupX 会自动生成随机值并写入 `system_configs` 表。请妥善备份 `data/backupx.db`，一旦丢失将导致所有已加密的存储配置失效。

## 环境变量

文件和环境变量同时存在时，环境变量优先。配置路径转换规则：小写字母下划线 → 大写字母下划线：

| 配置项 | 环境变量 |
|--------|----------|
| `server.port` | `BACKUPX_SERVER_PORT` |
| `server.external_url` | `BACKUPX_SERVER_EXTERNAL_URL` |
| `server.trusted_proxies` | `BACKUPX_SERVER_TRUSTED_PROXIES`（环境变量使用逗号分隔） |
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

## Master 对外 URL

当 BackupX 部署在 Docker、Nginx、负载均衡或多层反向代理后面，且后端收到的内部 Host 不是远程 Agent 可访问地址时，请配置 `server.external_url`：

```yaml
server:
  external_url: "https://backup.example.com"
```

BackupX 会用这个地址渲染一键 Agent 安装脚本和 docker-compose 片段。该地址必须能被所有 Agent 主机访问。只有在 `X-Forwarded-Proto` / `X-Forwarded-Host` 可靠且正好指向 Agent 可访问地址时，才建议留空。

代理或 SSH 堡垒机场景可在安装向导中为单个 Agent 设置覆盖地址。目标侧的一次性安装链接与生成的 Agent 运行配置都会使用这个地址，浏览器仍使用正常的公开地址。

## 可信反向代理

BackupX 只接受 `server.trusted_proxies` 中来源提供的客户端转发头。默认仅允许本机 Nginx。代理运行在其他容器或主机时，加入准确 IP 或网段：

```yaml
server:
  trusted_proxies:
    - "127.0.0.1"
    - "172.18.0.0/16"
```

不要配置 `0.0.0.0/0`，因为登录限流、安装令牌限流和审计日志都依赖客户端地址。BackupX 直接暴露且不应信任任何转发头时可设置空列表。

修改安全密钥或数据库路径前，应同时备份完整数据目录和配置文件。经过验证的快照与回滚流程见[升级与恢复](../operations/upgrade-recovery)。
