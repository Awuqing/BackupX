---
sidebar_position: 2
title: 裸机部署
description: 从预编译包或源码加固部署 BackupX，Nginx 改为显式启用。
---

# 裸机部署

## 使用预编译包

```bash
# 下载对应平台的压缩包
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz.sha256
sha256sum -c backupx-linux-amd64.tar.gz.sha256

# 解压并安装
tar xzf backupx-linux-amd64.tar.gz && cd backupx-*-linux-amd64
sudo ./install.sh
```

安装脚本自动完成以下步骤：

1. 创建系统用户 `backupx`
2. 复制二进制到 `/opt/backupx/bin/backupx`，并把 Web 控制台复制到 `/opt/backupx/web`
3. 把默认配置安装到 `/etc/backupx/config.yaml`
4. 安装并启用 `backupx.service` systemd 单元
5. 默认不修改 Nginx；只有显式设置 `INSTALL_NGINX=1` 时才安装模板
6. 验证首次初始化接口就绪后才报告安装成功

可执行文件与前端资源由 root 所有，只有 `/opt/backupx/data` 允许 `backupx` 服务账户写入。`/etc/backupx/config.yaml` 以 `root:backupx`、`0640` 权限安装。

仓库提供的 Nginx 模板只是起点，可能与现有默认站点冲突。先审核域名与 TLS 策略，再显式启用：

```bash
sudo INSTALL_NGINX=1 ./install.sh
```

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
UMask=0027
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

生产环境应通过 HTTPS 暴露 BackupX，或在防火墙限制 `8340` 端口。安装器不会自动修改防火墙。

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
