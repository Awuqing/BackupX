---
sidebar_position: 2
title: 安全加固
description: 生产环境的网络暴露、角色、密钥、Agent、容器和公开端点控制。
---

# 安全加固

BackupX 统一接触源文件、数据库凭据、存储凭据和恢复目标，应把 Master 作为安全敏感的控制面部署，而不是普通的公开 Web 应用。

## 推荐暴露模型

| 组件 | 入站访问 | 出站访问 |
| --- | --- | --- |
| Master | 管理员与 Agent 的 HTTPS；指标只对监控网段开放 | 存储提供商、通知端点、版本检查 |
| Agent | 不需要入站端口 | Master HTTPS 与分配的存储目标 |
| SQLite 数据 | 仅本地或块存储文件系统 | 无 |

反向代理与 Docker 位于同一主机时，把 Docker 绑定到 `127.0.0.1`：

~~~dotenv
BACKUPX_BIND_ADDRESS=127.0.0.1
~~~

裸机仅允许本机代理访问时，把 `server.host` 设置为回环地址；其他情况应使用主机或网络防火墙限制 TCP 8340。

## TLS 与反向代理

- 所有不可信网络段都使用 HTTPS。
- `server.external_url` 设置为 Agent 可访问的稳定地址。
- `server.trusted_proxies` 只加入准确代理 IP 或网段，禁止信任 `0.0.0.0/0`。
- Agent 使用最终 HTTPS 地址，不要依赖重定向。
- 私有 PKI 应向 Agent 下发 PEM CA，并配置 `caCertFile` 或 `--ca-cert`。
- `--insecure-tls` 只用于临时测试。
- Master 中转上传和 SSE 日志需要关闭 Nginx 请求与响应缓冲。

必须经过 SSH 堡垒机时，将隧道绑定到回环地址，严格校验主机密钥，使用专用账号与密钥，并让 Agent 服务依赖隧道。详见[多节点集群](../features/multi-node)。

## 角色与 API Key

| 角色 | 预期权限 |
| --- | --- |
| `viewer` | 读取仪表盘、任务、记录、报表与审计数据；不能浏览节点文件系统或修改资源 |
| `operator` | viewer 权限，加上任务、存储、通知、备份、恢复、验证和文件浏览操作 |
| `admin` | operator 权限，加上用户、API Key、设置、节点生命周期、安装令牌和 Token 轮换 |

为每位人员创建独立命名账号，不共享初始管理员。特权账号应启用双因素认证或通行密钥，并定期检查可信设备与恢复码。

用户 JWT 是无状态令牌。登出只会删除客户端副本，无法撤销已被复制到其他位置的 Token。应把 `security.jwt_expire` 设置为可接受的最短时长，保护 Bearer Token；必须使全部会话失效时轮换 JWT 密钥。

API Key 与交互式用户使用相同的角色检查。明文只在创建时显示一次，数据库只保存带密钥哈希。自动化应使用最低必要角色、设置有效期、保存在密钥管理系统，并及时撤销闲置 Key。监控不应使用管理员 Key。

## 保护控制面密钥

- `/etc/backupx/config.yaml` 应为 `root:backupx`、模式 `0640`，数据目录只允许服务账号访问。
- `jwt_secret` 和 `encryption_key` 留空时，自动生成值会写入 SQLite 数据库，因此必须备份完整数据目录。
- 加密密钥丢失或替换后，已有存储凭据将无法解密。
- 数据库包含密码哈希、配置密钥、Agent Token、API Key 哈希、可信设备状态和审计数据。快照应加密并设置保留策略。
- 不要把 Token 写入 shell 历史、Issue、截图或支持包。

每个节点有独立的长期 Agent Token。systemd 安装器把它保存到 `/etc/backupx-agent/agent.token`，模式为 `0600`。人员变更、主机入侵或意外泄露后应轮换 Token，在重叠窗口内更新文件并重启 Agent。

一次性安装 URL 有效期为 5 分钟至 24 小时，使用后立即失效。URL 与内嵌备用命令都应视为秘密，因为生成的安装材料会配置长期节点 Token。

## 容器与主机权限

正式 Compose 会删除全部能力，只添加旧数据卷所有权迁移与切换到非特权 `backupx` 用户所需的能力。保留 `no-new-privileges`，不要挂载 Docker Socket。

备份源应只读挂载；只有恢复目标确实需要时才添加独立、范围明确的可写挂载。需要高权限文件访问时，优先部署宿主机 Agent，而不是让 Master 容器以 root 运行。

systemd Master 以 `backupx` 运行。Agent 通常以 root 运行，因为它可能备份或恢复属于任意系统用户的文件。应限制任务创建权限并保护 root 所有的 Agent 配置。

## 公开端点

以下端点有意不使用 BackupX JWT 或 API Key 认证：

- `/health` 与 `/api/health`
- `/ready` 与 `/api/ready`
- `/metrics`
- 一次性 `/install/:token` 与 `/api/install/:token` 路由

健康响应包含状态、版本、运行时间、时间戳和就绪检查；就绪失败时可能带有数据库错误细节。`/metrics` 还会包含节点与存储目标标签。应在防火墙或反向代理只允许监控网段访问探针与指标，不要缓存或记录完整安装令牌 URL。

## 备份加密边界

加密备份任务只能在 Master 执行，因为远程 Agent 不会收到 Master 加密密钥。不要通过复制 Master 密钥到 Agent 来绕过这个边界。Agent 任务需要加密时，应根据要求使用传输层加密和存储提供商的服务端加密。

每次调整密钥管理后都应验证加密备份恢复。缺少密钥的备份不可恢复。

## 审计与事故响应

BackupX 会记录特权操作，并可把签名审计事件转发到外部 Webhook。高价值审计记录应发送到独立管理的 SIEM 或追加写存储，避免受损 Master 删除唯一副本。

怀疑入侵时：

1. 隔离 Master，但不要删除证据。
2. 撤销泄露的 API Key，轮换受影响的 Agent Token 与存储凭据。
3. JWT 与加密密钥只能按计划迁移；直接更换加密密钥会使已保存加密配置失效。
4. 审查用户、可信设备、API Key、节点、设置、恢复与删除事件。
5. 无法确认完整性时，从已知可信的控制面快照恢复。

应用与数据库配套恢复流程见[升级与恢复](./upgrade-recovery)。
