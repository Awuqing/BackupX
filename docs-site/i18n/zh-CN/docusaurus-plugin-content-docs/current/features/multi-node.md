---
sidebar_position: 4
title: 多节点集群
description: 通过直连 HTTPS、正向代理或 SSH 堡垒机部署 BackupX Agent。
---

# 多节点集群

BackupX 使用一个单活 Master 作为控制面，在每台源服务器运行 Agent。所有连接都由 Agent 主动发起：每 15 秒上报心跳，每 5 秒轮询命令，不需要为 Agent 开放入站端口。

## 架构与边界

```text
[Web 控制台] ────────> [单活 Master + SQLite]
                              ^
                              | Agent 主动 HTTP(S) 轮询
                    +---------+---------+
                    |         |         |
                 [Agent B] [Agent C] [Agent D]
                    |         |         |
                    +----> 存储目标
```

- 每个节点有独立 Agent Token，Agent 不持有 Master 的 JWT 密钥或配置加密密钥。
- Master 超过 45 秒未收到心跳即把节点标记为离线。
- Master 持久化命令，Agent 领取后在本机执行。
- 网络存储通常由 Agent 直传；Master 本地存储可显式启用认证流式中转。

:::warning Master 只能单活
内置 SQLite 不是共享多写数据库。同一个数据目录只能运行一个 Master。控制面高可用应采用主备主机、持久卷快照以及稳定 DNS 或虚拟 IP，故障时确保旧 Master 停止后再启动备用实例。不要让多个 Master 副本同时挂载 `/app/data` 或同一个 `backupx.db`。
:::

BackupX 会设置 5 秒 SQLite busy timeout，并为命令队列建立查询索引，降低 Agent 并发轮询及任务更新时的锁竞争。数据库应位于本地文件系统或块存储。采用文件复制备份控制面时，先停止 Master 再复制整个数据目录；运行期间不要只复制 `backupx.db`。

## 选择网络路径

| 场景 | Agent Master 地址 | Agent 代理 URL | 说明 |
| --- | --- | --- | --- |
| 可路由内网或公网服务 | `https://backup.example.com` | 留空 | 推荐，只需放行出站 TCP 443 |
| 企业正向代理 | `https://backup.example.com` | `http://proxy.internal:3128` | 支持 HTTP(S) 与 SOCKS5(H) |
| 通过堡垒机建立 SSH 动态转发 | `https://backup.internal` | `socks5h://127.0.0.1:1080` | 保留 TLS 主机名，并通过隧道解析内网 DNS |
| SSH 固定本地转发 | `http://127.0.0.1:18340` | 留空 | HTTP 链路位于 SSH 内，只能绑定回环地址 |

私有 PKI 场景请填写目标节点上预置的 PEM CA 证书绝对路径。生产环境不要使用 `--insecure-tls`。

未配置显式代理时，Agent 到 Master 的 HTTP 流量会遵循 `HTTP_PROXY`、`HTTPS_PROXY` 和 `NO_PROXY`。systemd 服务通常不会继承交互式 Shell 环境，因此 systemd 部署应在安装向导或 Agent YAML 中明确配置代理。

## 准备 Master

生成命令前先设置稳定地址：

```yaml title="/etc/backupx/config.yaml"
server:
  external_url: "https://backup.example.com"
  trusted_proxies:
    - "127.0.0.1"
    - "::1"
    # 代理不在本机时，只加入准确的代理 IP 或网段。
    # - "172.18.0.0/16"
```

`external_url` 是默认安装入口和 Agent 运行地址。受限节点可以让目标机侧生成的安装 URL 与 Agent 运行地址同时改用隧道或内网地址，浏览器仍继续访问公网地址。

跨不可信网络必须使用 HTTPS。Master 中转上传还要求反向代理关闭请求缓冲并允许大请求体，详见 [Nginx 反向代理](../deployment/nginx)。

Agent 必须直接配置最终 API 地址，不能依赖 HTTP 跳转到 HTTPS。Agent 会主动拒绝重定向，避免认证 Token 被转发到非预期主机。

## 部署 Agent

打开 **节点管理 → 添加节点**：

1. 输入单个节点名，或在批量模式输入最多 50 个名称。
2. 选择 systemd、Docker 或前台模式，以及架构、Agent Release、命令有效期和下载源。
3. 选择 **直连** 或 **代理或堡垒机**。受限网络可填写节点专用 Master 地址、代理 URL 或私有 CA 路径。
4. 把生成的命令复制到目标机，以 root 权限执行。

备份和恢复宿主机文件时推荐 systemd，因为 Agent 需要访问任意本地路径。Docker Agent 只能看到显式挂载的目录；分配文件任务前，应使用只读备份源 volume，并为恢复目标单独配置范围受限的可写挂载。

主命令通过一次性入口下载安装器，并在执行前校验脚本标记。向导会把所选 Agent 地址、显式代理和私有 CA 同时绑定到下载命令与安装后的 Agent 配置。如果目标网络仍无法访问安装入口，使用页面单独展示的嵌入式备用命令。嵌入式命令包含长期节点 Token，必须按密钥管理。

安装器会：

1. 检测 `linux/amd64` 或 `linux/arm64`。
2. 配置显式代理时始终通过该代理下载 Release；否则使用主机的正常直连或环境代理路径，并在该版本提供 SHA-256 旁车文件时进行校验。
3. 以 `0600` 权限写入 `/etc/backupx-agent/config.yaml` 和 `/etc/backupx-agent/agent.token`。
4. 不把 Token 写入 systemd unit 或 Docker 环境元数据。
5. 启动 Agent，并在 30 秒内轮询 `/api/v1/agent/self`。
6. 节点未上线时返回非零状态，并输出 systemd 或 Docker 排查命令。

旧版本如果没有校验文件，会显示兼容性警告后继续安装；新版本应始终发布并校验该文件。

### systemd 安装结果

```yaml title="/etc/backupx-agent/config.yaml"
master: "https://backup.example.com"
tokenFile: "/etc/backupx-agent/agent.token"
heartbeatInterval: "15s"
pollInterval: "5s"
tempDir: "/var/lib/backupx-agent/tmp"
proxyUrl: ""
caCertFile: ""
```

```ini title="/etc/systemd/system/backupx-agent.service"
[Unit]
Description=BackupX Agent
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=10

[Service]
Type=simple
ExecStart=/opt/backupx-agent/backupx agent --config /etc/backupx-agent/config.yaml
Restart=on-failure
RestartSec=10s
TimeoutStopSec=30s
UMask=0077
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Agent 以 root 运行，因为文件备份和恢复路径可能属于任意系统用户。应严格限制谁能创建任务，以及谁能修改 root 所有的 Agent 配置。

## SSH 堡垒机示例

内网 Master 使用 HTTPS 时优先采用 SOCKS 隧道，这样 Master 主机名与证书校验保持不变。

先创建专用 SSH 账户，预置私钥和已经人工核对指纹的 `known_hosts`，再创建：

```sshconfig title="/etc/backupx-agent/ssh_config"
Host backupx-bastion
    HostName bastion.example.com
    User backupx-tunnel
    IdentityFile /etc/backupx-agent/tunnel_ed25519
    IdentitiesOnly yes
    BatchMode yes
    UserKnownHostsFile /etc/backupx-agent/known_hosts
    StrictHostKeyChecking yes
    DynamicForward 127.0.0.1:1080
    ExitOnForwardFailure yes
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

```ini title="/etc/systemd/system/backupx-agent-tunnel.service"
[Unit]
Description=BackupX Agent SSH tunnel
After=network-online.target
Wants=network-online.target
Before=backupx-agent.service

[Service]
Type=simple
ExecStart=/usr/bin/ssh -NT -F /etc/backupx-agent/ssh_config backupx-bastion
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

再添加依赖覆写，让隧道不可用时 Agent 关闭失败而不是绕过堡垒机：

```ini title="/etc/systemd/system/backupx-agent.service.d/tunnel.conf"
[Unit]
Requires=backupx-agent-tunnel.service
After=backupx-agent-tunnel.service
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now backupx-agent-tunnel backupx-agent
```

在安装向导中保留内网 HTTPS Master 地址，把代理填写为 `socks5h://127.0.0.1:1080`。启用服务前必须通过独立渠道核对堡垒机 Host Key。

## 集中存储数据路径

| 目标 | 数据路径 |
| --- | --- |
| S3、WebDAV、FTP、云盘或其他网络后端 | Agent 直接流式上传到目标 |
| 启用 **远程备份经 Master 中转** 的 `local_disk` | Agent 通过认证 Master API 流式上传，Master 写入本地挂载 |

中转不会在 Master 上额外创建一份完整临时副本，恢复时走反向流式通道。Nginx 必须关闭请求缓冲，才能保持该特性。

## 运维

```bash
sudo systemctl status backupx-agent
sudo journalctl -u backupx-agent -n 100 --no-pager
sudo /opt/backupx-agent/backupx agent --config /etc/backupx-agent/config.yaml
```

从节点操作菜单轮换 Token 后，在 24 小时重叠窗口内更新 `/etc/backupx-agent/agent.token` 并重启服务。

建议监控：

- `backupx_agent_command_queue_depth`
- `backupx_agent_command_running`
- `backupx_agent_command_timeout_total`
- `backupx_node_online`

## CLI 参考

```text
backupx agent --help
  -master string       Master 地址
  -token string        Agent Token
  -token-file string   从文件读取 Agent Token
  -config string       YAML 配置文件路径
  -temp-dir string     本地临时目录
  -proxy-url string    HTTP(S) 或 SOCKS5(H) 代理
  -ca-cert string      用于校验 Master 的 PEM CA 证书
  -insecure-tls        跳过 TLS 校验（仅测试）
```

环境变量：`BACKUPX_AGENT_MASTER`、`BACKUPX_AGENT_TOKEN`、`BACKUPX_AGENT_TOKEN_FILE`、`BACKUPX_AGENT_HEARTBEAT`、`BACKUPX_AGENT_POLL`、`BACKUPX_AGENT_TEMP_DIR`、`BACKUPX_AGENT_PROXY_URL`、`BACKUPX_AGENT_CA_CERT_FILE`、`BACKUPX_AGENT_INSECURE_TLS`。

## 已知限制

- Master 使用内置 SQLite，只支持单活。
- 加密备份仅支持 Master 本机执行，因为 Agent 不持有 Master 加密密钥。
- 远程目录浏览是同步队列 RPC，默认超时 15 秒。
- Agent 领取后长期不更新的命令会由 Master 超时监控处理。
