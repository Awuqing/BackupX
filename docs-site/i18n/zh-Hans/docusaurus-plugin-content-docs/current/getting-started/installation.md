---
sidebar_position: 1
title: 安装
description: 通过 Docker、预编译包或源码安装 BackupX。
---

# 安装

BackupX 以单个静态二进制发布。三种安装方式，按实际环境选一种。

## Docker（推荐）

无需克隆仓库：

```bash
docker run -d --name backupx \
  -p 8340:8340 \
  -v backupx-data:/app/data \
  awuqing/backupx:latest
```

或使用 `docker compose`：

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
      # 挂载需要备份的宿主机目录（按需添加）：
      # - /var/www:/mnt/www:ro
      # - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai

volumes:
  backupx-data:
```

Docker Hub：[`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx)，支持 linux/amd64 和 linux/arm64。

## 预编译包（裸机）

从 [Releases 页面](https://github.com/Awuqing/BackupX/releases) 下载对应平台的压缩包，执行安装脚本：

```bash
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh        # 创建系统用户、安装到 /opt/backupx、配置 systemd + Nginx
```

安装脚本会自动：

1. 创建 `backupx` 系统用户
2. 安装二进制到 `/opt/backupx/backupx`
3. 生成 `/opt/backupx/config.yaml`（含安全默认值）
4. 注册并启用 `backupx.service` systemd 单元
5. （可选）配置 Nginx 反向代理

## 从源码构建

依赖：Go ≥ 1.25，Node.js ≥ 20。

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
# 或使用国内镜像加速构建 Docker
make docker-cn
```

`make build` 完成后，二进制位于 `server/bin/backupx`，构建好的 Web UI 位于 `web/dist/`。

## 验证安装

```bash
backupx --version           # 输出如 v1.6.0
```

打开浏览器访问 `http://your-server:8340`，会进入初始化管理员账户页面。
