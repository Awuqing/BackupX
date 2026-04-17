---
sidebar_position: 4
title: 多节点集群
description: Master-Agent 模式 — 通过 HTTP 长轮询把备份路由到远程服务器。
---

# 多节点集群

BackupX 支持 Master-Agent 模式：备份任务可以指定在哪个节点执行，Agent 在本地完成备份并直接上传到存储。所有连接都由 Agent 主动发起，所以远程服务器只需要出站 HTTP 访问权限。

## 架构

```
[Web 控制台] ─── JWT ──→ [Master (backupx)]
                              ↑  ↓
                              │  │ HTTP 长轮询（Token 认证）
                              │  ↓
                         [Agent (backupx agent)]   ← 运行在远程服务器
                              ↓
                       [70+ 存储后端]
```

- **协议** — HTTP 长轮询，Agent 主动发起所有连接
- **心跳** — Agent 每 15s 上报一次；Master 超过 45s 未收到心跳即判为离线
- **下发** — Master 把 `run_task` 命令写入队列，Agent 轮询拉取
- **执行** — Agent 复用 BackupRunner（file / mysql / postgresql / sqlite / saphana）并直接上传到存储
- **安全** — 每个节点独立 Token；Agent 不持有 Master 的 JWT 密钥或 AES-256 加密密钥

## 使用步骤

### 1. 在 Master 创建节点

Web 控制台 → **节点管理** → **添加节点**。界面会**一次性**显示 64 字节十六进制令牌，请妥善保存。

### 2. 在远程服务器部署 Agent

把 BackupX 二进制上传到目标服务器（与 Master 同一个文件），然后用以下任一方式启动：

**方式 A：CLI 参数**

```bash
backupx agent --master http://master.example.com:8340 --token <token>
```

**方式 B：配置文件**

```yaml title="/etc/backupx/agent.yaml"
master: http://master.example.com:8340
token: <token>
heartbeatInterval: 15s
pollInterval: 5s
tempDir: /var/lib/backupx-agent
```

```bash
backupx agent --config /etc/backupx/agent.yaml
```

**方式 C：环境变量**（适合 Docker / systemd）

```bash
BACKUPX_AGENT_MASTER=http://master.example.com:8340 \
BACKUPX_AGENT_TOKEN=<token> \
backupx agent
```

连接成功后节点在列表中显示为 **在线**。

### 3. 把任务路由到该节点

在 **备份任务** 页面新建任务时选择对应节点。任务触发时：

- 本机 / 未指定（`nodeId=0`）：Master 进程内直接执行
- 远程节点：Master 写入命令队列 → Agent 拉取 → Agent 本地执行 → 上传 → 回报

## 已知限制

- **Agent 不支持加密备份**：Agent 不持有 Master 的 AES-256 密钥。`encrypt: true` 的任务路由到 Agent 时会直接上报失败
- **目录浏览超时**：远程目录浏览通过命令队列做同步 RPC，默认 15s 超时
- **派发命令超时**：Agent 领取但未完成的命令超过 10 分钟会被置 `timeout`

## CLI 参考

```
backupx agent --help
  -master string    Master URL
  -token string     Agent 认证令牌
  -config string    YAML 配置文件路径（优先级高于环境变量）
  -temp-dir string  本地临时目录（默认 /tmp/backupx-agent）
  -insecure-tls     跳过 TLS 证书校验（仅测试用）
```

## systemd 单元

```ini title="/etc/systemd/system/backupx-agent.service"
[Unit]
Description=BackupX Agent
After=network.target

[Service]
Type=simple
User=backupx
Environment="BACKUPX_AGENT_MASTER=https://master.example.com"
Environment="BACKUPX_AGENT_TOKEN=your-token"
ExecStart=/opt/backupx/backupx agent
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
```

启用并启动：

```bash
sudo systemctl enable --now backupx-agent
sudo journalctl -u backupx-agent -f
```
