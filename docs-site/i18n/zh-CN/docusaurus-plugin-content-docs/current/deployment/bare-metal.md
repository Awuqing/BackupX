---
sidebar_position: 2
title: 裸机部署
description: 从预编译包或源码部署 BackupX（systemd + Nginx）。
---

# 裸机部署

## 使用预编译包

```bash
# 下载对应平台的压缩包
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz

# 解压并安装
tar xzf backupx-linux-amd64.tar.gz && cd backupx-*-linux-amd64
sudo ./install.sh
```

安装脚本自动完成以下步骤：

1. 创建系统用户 `backupx`
2. 复制二进制到 `/opt/backupx/bin/backupx`，并把 Web 控制台复制到 `/opt/backupx/web`
3. 把默认配置安装到 `/etc/backupx/config.yaml`
4. 安装并启用 `backupx.service` systemd 单元
5. （可选）生成 Nginx 站点配置 — 参见 [Nginx 反向代理](./nginx)
6. 验证首次初始化接口就绪后才报告安装成功

如果要部署多节点集群，安装后请编辑 `/etc/backupx/config.yaml`，设置远程 Agent 可访问到的 Master URL：

```yaml
server:
  external_url: "https://backup.example.com"
```

修改后重启 BackupX：

```bash
sudo systemctl restart backupx
```

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
Group=backupx
WorkingDirectory=/opt/backupx
ExecStart=/opt/backupx/bin/backupx -config /etc/backupx/config.yaml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

常用命令：

```bash
sudo systemctl status backupx
sudo journalctl -u backupx -f    # 实时日志
sudo systemctl restart backupx
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
```

访问 `http://your-server:8340`，可按需切换到 English，然后在“系统初始化 / System setup”页面创建首个管理员。若监听端口不是默认值，请为安装脚本传入对应的 `HEALTH_URL`。

## 密码重置

忘记管理员密码时：

```bash
/opt/backupx/bin/backupx reset-password \
  --username admin \
  --password 'newpass123' \
  --config /etc/backupx/config.yaml
```

Docker 等效命令：

```bash
docker exec -it backupx /app/bin/backupx reset-password --username admin --password 'newpass123'
```
