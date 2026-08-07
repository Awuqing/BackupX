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
2. 安装二进制到 `/opt/backupx/bin/backupx`，并把 Web 控制台安装到 `/opt/backupx/web`
3. 生成 `/etc/backupx/config.yaml`（含安全默认值）
4. 注册并启用 `backupx.service` systemd 单元
5. （可选）配置 Nginx 反向代理
6. 等待 `/api/auth/setup/status` 就绪；启动失败时输出 systemd 诊断并返回非零状态

## 从源码构建

依赖：Go ≥ 1.25，Node.js ≥ 20。

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
sudo ./deploy/install.sh
```

`make build` 完成后，二进制位于 `server/bin/backupx`，构建好的 Web UI 位于 `web/dist/`。
安装脚本会直接使用这两个路径，不需要 Docker 运行时。如果已有配置修改了默认端口，可覆盖就绪检查地址，例如：`sudo HEALTH_URL=http://127.0.0.1:9000/api/auth/setup/status ./deploy/install.sh`。

## 验证安装

```bash
/opt/backupx/bin/backupx --version
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
```

打开浏览器访问 `http://your-server:8340`，可在右上角选择 **中文** 或 **English**。全新数据库会显示“系统初始化 / System setup”，在这里创建首个管理员用户名和密码。如果没有出现初始化表单，请先重试上面的状态接口，不要直接尝试登录。
