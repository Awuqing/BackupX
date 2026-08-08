---
sidebar_position: 1
title: Docker 部署
description: 带健康检查和持久化数据的加固单进程 Docker 部署。
---

# Docker 部署

官方镜像 [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) 支持 `linux/amd64` 和 `linux/arm64`。

## Compose 文件

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

入口脚本仅以 root 完成旧镜像数据的所有权迁移，随后只运行一个非 root `backupx` 进程；Compose 仅保留初始化所需的所有权与 UID/GID 切换能力。后端同时提供 API 与前端静态文件，默认不挂载 Docker Socket，也不打包 Docker CLI。生产环境应把 `BACKUPX_IMAGE` 固定到明确 Release 标签。

## 备份宿主机目录

按需挂载源目录，并在任务中使用容器内路径。容器中的 `backupx` 用户必须拥有读取权限；恢复目标应使用单独且范围受限的可写挂载。特权路径优先通过远程 Agent 处理；确实需要 Master 以 root 读取时，应显式设置 `user: "0:0"` 并审核每一个挂载。

## 多节点集群

设置所有 Agent 可达的稳定地址：

```yaml
environment:
  BACKUPX_SERVER_EXTERNAL_URL: https://backup.example.com
```

跨不可信网络必须使用 HTTPS。代理、私有 CA 和 SSH 堡垒机场景见 [多节点集群](../features/multi-node)。

外部反向代理运行在其他容器时，只把准确的 Docker 网桥网段加入 `BACKUPX_SERVER_TRUSTED_PROXIES`，例如 `172.18.0.0/16`，不要信任所有地址。

## 环境变量覆盖

```yaml
environment:
  TZ: Asia/Shanghai
  BACKUPX_LOG_LEVEL: debug
  BACKUPX_BACKUP_MAX_CONCURRENT: "4"
  BACKUPX_BACKUP_TEMP_DIR: /tmp/backupx
```

镜像内部端口固定为 `8340`，只通过 `BACKUPX_PORT` 修改宿主机发布端口。

## 升级与回退准备

```bash
docker compose pull
docker compose up -d
docker compose ps
```

等待状态变为 `healthy` 后再切换流量或移除旧部署。升级前应停止 Master 后做文件级复制，或对整个 `backupx-data` 卷创建原子快照。同一个数据卷必须只运行一个活动 Master；SQLite 不支持多个 Master 容器共享 `/app/data`。
