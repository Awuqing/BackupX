---
sidebar_position: 2
title: 裸机部署
description: 从预编译包或源码部署 BackupX（systemd + Nginx）。
---

# 裸机部署

## 使用预编译包

```bash
# 下载对应平台的压缩包
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-v1.6.0-linux-amd64.tar.gz

# 解压并安装
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh
```

安装脚本自动完成以下步骤：

1. 创建系统用户 `backupx`
2. 复制二进制到 `/opt/backupx/`
3. 生成默认 `config.yaml`（含安全的 JWT/加密密钥）
4. 安装并启用 `backupx.service` systemd 单元
5. （可选）生成 Nginx 站点配置 — 参见 [Nginx 反向代理](./nginx)

## 从源码构建

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build
sudo ./deploy/install.sh
```

`make build` 会产出：

- `server/bin/backupx`（Go 后端，无 CGO）
- `web/dist/`（React 前端，执行 `npm run build`）

## systemd

安装后的 service 文件：

```ini title="/etc/systemd/system/backupx.service"
[Unit]
Description=BackupX backup management service
After=network.target

[Service]
Type=simple
User=backupx
WorkingDirectory=/opt/backupx
ExecStart=/opt/backupx/backupx --config /opt/backupx/config.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

常用命令：

```bash
sudo systemctl status backupx
sudo journalctl -u backupx -f    # 实时日志
sudo systemctl restart backupx
```

## 密码重置

忘记管理员密码时：

```bash
/opt/backupx/backupx reset-password \
  --username admin \
  --password 'newpass123' \
  --config /opt/backupx/config.yaml
```

Docker 等效命令：

```bash
docker exec -it backupx /app/bin/backupx reset-password --username admin --password 'newpass123'
```
