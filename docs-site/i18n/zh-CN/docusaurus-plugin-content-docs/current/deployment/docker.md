---
sidebar_position: 1
title: Docker 部署
description: 生产级 Docker 部署方案，含 compose 配置、宿主目录挂载、环境变量覆盖。
---

# Docker 部署

BackupX 官方 Docker 镜像 [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) 支持多架构（linux/amd64 + linux/arm64）。

## Compose 文件

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
      # 挂载需要备份的宿主机目录：
      - /var/www:/mnt/www:ro
      - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai
      - BACKUPX_LOG_LEVEL=info
      - BACKUPX_BACKUP_MAX_CONCURRENT=2

volumes:
  backupx-data:
```

启动：

```bash
docker compose up -d
```

## 备份宿主机目录

想备份宿主机上的文件，需要将对应路径挂载进容器。在 Web UI 创建文件类型任务时，把源路径指向挂载后的容器内路径（如 `/mnt/www`）。

## 环境变量

所有配置项都可以通过 `BACKUPX_` 前缀环境变量覆盖：

```yaml
environment:
  - TZ=Asia/Shanghai
  - BACKUPX_SERVER_PORT=8340
  - BACKUPX_LOG_LEVEL=debug
  - BACKUPX_BACKUP_MAX_CONCURRENT=4
  - BACKUPX_BACKUP_TEMP_DIR=/tmp/backupx
```

完整列表见 [配置参考](./configuration)。

## 升级

在 UI **系统设置 → 检查更新** 页面查看是否有新版，然后在宿主机上：

```bash
docker compose pull && docker compose up -d
```

无需手工迁移：BackupX 启动时自动迁移 SQLite schema。
