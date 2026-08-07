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

## 把 B/C/D 服务器集中备份到 M

Master 作为控制面，每台源服务器安装一个 Agent。任务里的 **源服务器** 决定源路径和数据库工具在哪台机器解析，**存储目标** 决定备份产物最终保留在哪里。

BackupX 会根据目标类型选择数据路径：

| 目标 | 数据路径 |
| --- | --- |
| S3、WebDAV、FTP、云盘或其他网络后端 | Agent 直接流式上传到目标 |
| 启用 **远程备份经 Master 中转** 的 `local_disk`（例如通过 NFS 挂载的存储服务器 M） | Agent 通过认证后的 Master API 流式中转，由 Master 写入配置目录 |

中转过程不会在 Master 上额外落一份完整临时文件。把 Master 本地磁盘中的备份恢复到源 Agent 时会走反向流式通道。Agent 与 Master 之间跨越不可信网络时必须配置 HTTPS。

典型的 `A → {B,C,D} → M` 拓扑按以下步骤配置：

1. 在 A 运行 BackupX Master；如果 M 以 NFS 等文件系统提供存储，先把 M 挂载到 A。
2. 为该挂载点创建 `local_disk` 目标并保持 **远程备份经 Master 中转** 开启；如果 M 提供 S3/WebDAV，也可直接创建对应网络目标。升级前已有的本地磁盘目标会继续沿用 Agent 本机落盘，手动开启该选项后才切换到中央目录。
3. 从 **节点管理** 分别在 B、C、D 安装 Agent。
4. 为每台源服务器创建任务，在 **源服务器** 选择 B/C/D，浏览该服务器的路径，再把 M 选为存储目标。相同任务也可用源服务器池标签动态调度。
5. 在备份记录中检查逐目标结果。Master 本地磁盘目标会记录 `master_relay` 中转模式，网络后端仍为 `direct` 直传。

## 一键部署步骤

### 0. 为生产集群设置 Master 对外 URL

生成 Agent 安装命令前，请先确认 Master URL 对所有目标主机稳定可达。

如果 BackupX 部署在 Docker、Nginx、负载均衡或外层反向代理后面，请在 Master 配置 `server.external_url` 或环境变量 `BACKUPX_SERVER_EXTERNAL_URL`：

```yaml title="config.yaml"
server:
  external_url: "https://backup.example.com"
```

该 URL 会写入 systemd 单元、前台运行命令和 docker-compose 片段。如果地址不正确，Agent 可能安装成功但始终离线，因为它会持续轮询一个内网地址或仅浏览器可访问的地址。

### 1. 打开安装向导

Web 控制台 → **节点管理** → **添加节点**，打开三步向导：

- **第一步 · 节点信息**：填写节点名称；或切换"批量创建"粘贴多行名称（每行一个，最多 50 个）
- **第二步 · 部署参数**：选择安装模式（`systemd` 推荐、`Docker`、`前台运行` 调试用）、架构（默认自动检测）、Agent 版本（默认跟随 Master 版本）、有效期（5 分钟 / 15 分钟 / 1 小时 / 24 小时）、下载源（`GitHub` 直连或 `ghproxy` 镜像，国内服务器建议后者）
- **第三步 · 安装命令**：一条一键安装命令 + 实时倒计时。点击复制，粘贴到目标机以 root 权限执行。默认命令会嵌入已渲染的安装脚本，目标机无需再通过反向代理访问 `/api/install/:token`；公开安装 URL 仍作为备用路径保留。

### 2. 目标机一条命令完成

请直接使用 Web 控制台生成的命令。该命令会把安装脚本写入临时文件，校验 `BACKUPX_AGENT_INSTALL_V1` 魔数，再以 root 权限执行。

脚本会自动：

1. 检测操作系统与架构（`uname -m`）
2. 从 GitHub Release（或 ghproxy 镜像）下载匹配的 `backupx` 二进制
3. 安装到 `/opt/backupx-agent`，创建系统用户 `backupx`
4. 写入 `/etc/systemd/system/backupx-agent.service`（token 已烧入环境变量）
5. 执行 `systemctl enable --now backupx-agent`
6. 轮询 `/api/v1/agent/self`，直到 Master 确认 `status: online`（最多 30 秒）

Docker 模式使用同一组环境变量约定：`BACKUPX_AGENT_MASTER`、`BACKUPX_AGENT_TOKEN` 和 `BACKUPX_AGENT_TEMP_DIR=/var/lib/backupx-agent/tmp`。容器启动后，安装脚本同样会探测 `/api/v1/agent/self`；如果节点没有上线，会输出 `docker ps` 与 `docker logs --tail=100 backupx-agent` 排查命令，并以非零状态退出。

如果使用 URL 备用命令时 `curl` 输出 HTML，或 shell 报 `Syntax error: newline unexpected`，说明安装 URL 被 Web 控制台接管而不是转发到后端。需要确保 `/api/install/` 或 `/install/` 至少一个路径能转发到 BackupX 后端，或改用控制台生成的嵌入式命令。

脚本是幂等的：升级或重装只需重新生成一条安装命令再跑一次。一次性安装链接在 TTL 到期或被首次消费后立即作废。

### 3. 随时轮换 Agent Token

节点操作列（︙）→ **重新生成 Token**。新 Token 一次性显示，旧 Token 24 小时内仍有效，便于滚动替换无需停机。24 小时后旧 Token 被拒绝。

### 4. 批量部署

第一步选"批量创建"粘贴节点名（每行一个，最多 50 个）。第三步显示每个节点对应的命令表格，底部「导出 .sh」可打包为单个 shell 文件，方便 SSH 循环或 Ansible 任务。

### 5. 把任务路由到该节点

在 **备份任务** 页面新建任务时选择对应源服务器。任务触发时：

- 本机 / 未指定（`nodeId=0`）：Master 进程内直接执行
- 远程节点：Master 写入命令队列 → Agent 拉取 → Agent 本地执行 → 上传 → 回报

节点列表会展示 Agent 健康与命令队列状态：pending/dispatched 深度、运行中的长任务、超时数、最旧活跃命令年龄和最近 Agent 错误。同样的队列深度、运行中命令数和超时快照会导出为 Prometheus 指标：

- `backupx_agent_command_queue_depth`
- `backupx_agent_command_running`
- `backupx_agent_command_timeout_total`

## 已知限制

- **加密备份仅支持 Master 本机执行**：Agent 不持有 Master 的 AES-256 密钥。创建或更新任务时，如果 `encrypt: true` 且选择了远程节点或节点池，会在入口直接拒绝
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
