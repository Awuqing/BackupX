---
sidebar_position: 1
title: API 参考
description: BackupX REST 端点、认证方式、角色边界、流式响应和公开探针。
---

# API 参考

交互式 API 以 `/api` 为根路径。大多数端点接受用户 JWT 或 API Key；Agent 协议使用节点专用 Token。公开探针和一次性安装器在文末单列。

## 认证

### 用户 JWT

通过 `POST /api/auth/login` 获取 JWT，并作为 Bearer Token 发送：

~~~bash
curl -H "Authorization: Bearer $BACKUPX_TOKEN" \
  https://backup.example.com/api/backup/tasks
~~~

根据账号和系统设置，登录过程还可能要求邮件或短信 OTP、TOTP、恢复码、可信设备 Token 或 WebAuthn。

### API Key

管理员可在控制台或通过 `POST /api/api-keys` 创建 API Key。明文 `bax_...` 只返回一次。

~~~bash
curl -H "X-Api-Key: $BACKUPX_API_KEY" \
  https://backup.example.com/api/dashboard/stats
~~~

也支持 `Authorization: Bearer bax_...`。API Key 带有 `admin`、`operator` 或 `viewer` 角色，可禁用并可设置有效期。

### Agent Token

Agent 协议 Handler 从 `X-Agent-Token` 验证节点 Token。它不是用户凭据，不能用于交互式资源 API。

### 权限标记

下表使用这些标记：

| 标记 | 所需权限 |
| --- | --- |
| 公开 | 不需要 JWT 或 API Key；安装路由仍要求一次性 Token |
| 已认证 | 任意 `viewer`、`operator` 或 `admin` |
| 运维 | `operator` 或 `admin` |
| 管理员 | 仅 `admin` |
| Agent | 有效的节点专用 Agent Token |

viewer 可使用读取端点，但不能浏览节点文件系统；operator 可以执行和修改备份资源；admin 还可管理用户、API Key、设置、节点、安装令牌和节点 Token 轮换。角色不满足时返回 HTTP 403。

## 认证与账号安全

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/auth/setup/status` | 公开 | 查询是否需要创建首个管理员 |
| `POST` | `/api/auth/setup` | 公开 | 系统无用户时创建首个管理员 |
| `POST` | `/api/auth/login` | 公开 | 完成密码或 MFA 登录并获取 JWT |
| `POST` | `/api/auth/otp/send` | 公开 | 发送已配置的登录 OTP |
| `POST` | `/api/auth/webauthn/login/options` | 公开 | 开始通行密钥登录 |
| `POST` | `/api/auth/logout` | 已认证 | 确认登出；客户端必须丢弃无状态 JWT |
| `GET` | `/api/auth/profile` | 已认证 | 读取当前账号 |
| `PUT` | `/api/auth/password` | 已认证 | 修改当前账号密码 |
| `POST` | `/api/auth/2fa/setup` | 已认证 | 准备 TOTP 注册 |
| `POST` | `/api/auth/2fa/enable` | 已认证 | 验证后启用 TOTP |
| `POST` | `/api/auth/2fa/recovery-codes` | 已认证 | 重新生成恢复码 |
| `DELETE` | `/api/auth/2fa` | 已认证 | 停用 TOTP |
| `PUT` | `/api/auth/otp/config` | 已认证 | 更新 OTP 登录配置 |
| `POST` | `/api/auth/webauthn/register/options` | 已认证 | 开始注册通行密钥 |
| `POST` | `/api/auth/webauthn/register/finish` | 已认证 | 完成通行密钥注册 |
| `GET` | `/api/auth/webauthn/credentials` | 已认证 | 列出通行密钥 |
| `DELETE` | `/api/auth/webauthn/credentials/:id` | 已认证 | 删除通行密钥 |
| `GET` | `/api/auth/trusted-devices` | 已认证 | 列出可信设备 |
| `DELETE` | `/api/auth/trusted-devices/:id` | 已认证 | 撤销可信设备 |

账号安全端点应使用交互式 JWT，不应使用自动化 API Key。

## 系统与存储目标

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/system/info` | 已认证 | 版本与系统信息 |
| `GET` | `/api/system/update-check` | 已认证 | 检查可用 Release |
| `GET` | `/api/storage-targets` | 已认证 | 存储目标列表 |
| `POST` | `/api/storage-targets` | 运维 | 创建目标 |
| `POST` | `/api/storage-targets/test` | 运维 | 测试未保存配置 |
| `GET` | `/api/storage-targets/rclone/backends` | 已认证 | 可用 rclone 后端 |
| `POST` | `/api/storage-targets/google-drive/auth-url` | 运维 | 开始 Google Drive 授权 |
| `POST` | `/api/storage-targets/google-drive/complete` | 运维 | 完成 Google Drive 授权 |
| `GET` | `/api/storage-targets/google-drive/callback` | 已认证 | 处理 OAuth 回调 |
| `GET` | `/api/storage-targets/:id` | 已认证 | 读取目标 |
| `PUT` | `/api/storage-targets/:id` | 运维 | 更新目标 |
| `DELETE` | `/api/storage-targets/:id` | 运维 | 删除目标 |
| `PUT` | `/api/storage-targets/:id/star` | 运维 | 切换收藏 |
| `POST` | `/api/storage-targets/:id/test` | 运维 | 测试已保存目标 |
| `GET` | `/api/storage-targets/:id/usage` | 已认证 | 读取已记录用量 |
| `GET` | `/api/storage-targets/:id/google-drive/profile` | 已认证 | 读取已连接 Google Drive 账号 |

## 备份任务

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/backup/tasks` | 已认证 | 任务列表 |
| `GET` | `/api/backup/tasks/tags` | 已认证 | 任务标签 |
| `GET` | `/api/backup/tasks/export` | 已认证 | 下载全部任务 JSON，或用 `?ids=1,2` 选择任务 |
| `POST` | `/api/backup/tasks/import` | 运维 | 导入任务，最大 1 MiB |
| `POST` | `/api/backup/tasks/batch/toggle` | 运维 | 批量启用或停用 |
| `POST` | `/api/backup/tasks/batch/delete` | 运维 | 批量删除 |
| `POST` | `/api/backup/tasks/batch/run` | 运维 | 批量执行 |
| `GET` | `/api/backup/tasks/:id` | 已认证 | 读取任务 |
| `POST` | `/api/backup/tasks` | 运维 | 创建任务 |
| `PUT` | `/api/backup/tasks/:id` | 运维 | 更新任务 |
| `DELETE` | `/api/backup/tasks/:id` | 运维 | 删除任务 |
| `PUT` | `/api/backup/tasks/:id/toggle` | 运维 | 启用或停用 |
| `POST` | `/api/backup/tasks/:id/run` | 运维 | 触发备份 |
| `POST` | `/api/backup/tasks/:id/verify` | 运维 | 从任务触发验证 |

任务导出会主动排除数据库密码与存储凭据，适合迁移和审阅，不是完整控制面备份。

## 备份与恢复记录

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/backup/records` | 已认证 | 列出并筛选备份记录 |
| `POST` | `/api/backup/records/batch-delete` | 运维 | 批量删除记录 |
| `GET` | `/api/backup/records/:id` | 已认证 | 读取备份记录 |
| `GET` | `/api/backup/records/:id/logs/stream` | 已认证 | 通过 SSE 输出日志 |
| `GET` | `/api/backup/records/:id/download` | 已认证 | 下载产物 |
| `GET` | `/api/backup/records/:id/contents` | 已认证 | 浏览支持类型的产物内容 |
| `POST` | `/api/backup/records/:id/restore` | 运维 | 启动恢复 |
| `POST` | `/api/backup/records/:id/replicate` | 运维 | 复制已有产物 |
| `POST` | `/api/backup/records/:id/verify` | 运维 | 验证已有产物 |
| `PUT` | `/api/backup/records/:id/lock` | 运维 | 设置保留锁 |
| `DELETE` | `/api/backup/records/:id` | 运维 | 删除记录及受管产物 |
| `GET` | `/api/restore/records` | 已认证 | 恢复记录列表 |
| `GET` | `/api/restore/records/:id` | 已认证 | 恢复记录详情 |
| `GET` | `/api/restore/records/:id/logs/stream` | 已认证 | 恢复日志 SSE |
| `GET` | `/api/replication/records` | 已认证 | 复制记录列表 |
| `GET` | `/api/replication/records/:id` | 已认证 | 复制记录详情 |
| `GET` | `/api/verify/records` | 已认证 | 验证记录列表 |
| `GET` | `/api/verify/records/:id` | 已认证 | 验证记录详情 |
| `GET` | `/api/verify/records/:id/logs/stream` | 已认证 | 验证日志 SSE |

## 模板、报表与仪表盘

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/task-templates` | 已认证 | 任务模板列表 |
| `GET` | `/api/task-templates/:id` | 已认证 | 读取任务模板 |
| `POST` | `/api/task-templates` | 运维 | 创建模板 |
| `PUT` | `/api/task-templates/:id` | 运维 | 更新模板 |
| `DELETE` | `/api/task-templates/:id` | 运维 | 删除模板 |
| `POST` | `/api/task-templates/:id/apply` | 运维 | 从模板创建任务 |
| `GET` | `/api/reports/compliance` | 已认证 | 合规证据 |
| `GET` | `/api/reports/compliance/export` | 已认证 | 导出合规 CSV |
| `GET` | `/api/dashboard/stats` | 已认证 | 汇总统计 |
| `GET` | `/api/dashboard/timeline` | 已认证 | 最近活动 |
| `GET` | `/api/dashboard/sla` | 已认证 | RPO 与 SLA 状态 |
| `GET` | `/api/dashboard/cluster` | 已认证 | 集群概览 |
| `GET` | `/api/dashboard/breakdown` | 已认证 | 任务与记录分布 |
| `GET` | `/api/dashboard/node-performance` | 已认证 | 节点性能 |

## 通知、设置与管理

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/notifications` | 已认证 | 通知渠道列表 |
| `GET` | `/api/notifications/:id` | 已认证 | 读取渠道 |
| `POST` | `/api/notifications` | 运维 | 创建渠道 |
| `PUT` | `/api/notifications/:id` | 运维 | 更新渠道 |
| `DELETE` | `/api/notifications/:id` | 运维 | 删除渠道 |
| `POST` | `/api/notifications/test` | 运维 | 测试未保存配置 |
| `POST` | `/api/notifications/:id/test` | 运维 | 测试已保存渠道 |
| `GET` | `/api/settings` | 已认证 | 读取系统设置 |
| `PUT` | `/api/settings` | 管理员 | 更新系统设置 |
| `GET` | `/api/users` | 管理员 | 用户列表 |
| `POST` | `/api/users` | 管理员 | 创建用户 |
| `PUT` | `/api/users/:id` | 管理员 | 更新用户 |
| `POST` | `/api/users/:id/2fa/reset` | 管理员 | 重置用户第二因素 |
| `DELETE` | `/api/users/:id` | 管理员 | 删除用户 |
| `GET` | `/api/api-keys` | 管理员 | API Key 列表，不返回明文 |
| `POST` | `/api/api-keys` | 管理员 | 创建 API Key，明文仅返回一次 |
| `PUT` | `/api/api-keys/:id/toggle` | 管理员 | 启用或停用 API Key |
| `DELETE` | `/api/api-keys/:id` | 管理员 | 撤销 API Key |

## 审计、事件、搜索与发现

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/audit-logs` | 已认证 | 列出并筛选审计记录 |
| `GET` | `/api/audit-logs/export` | 已认证 | 导出审计记录 |
| `GET` | `/api/events/stream` | 已认证 | 通过 SSE 输出实时应用事件 |
| `GET` | `/api/search` | 已认证 | 搜索支持的资源 |
| `POST` | `/api/database/discover` | 已认证 | 按提供的连接信息发现数据库 |

## 节点

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/nodes` | 已认证 | 节点列表 |
| `GET` | `/api/nodes/:id` | 已认证 | 节点详情 |
| `GET` | `/api/nodes/:id/fs/list` | 运维 | 浏览所选节点文件系统 |
| `POST` | `/api/nodes` | 管理员 | 创建节点 |
| `POST` | `/api/nodes/batch` | 管理员 | 批量创建最多 50 个节点 |
| `PUT` | `/api/nodes/:id` | 管理员 | 更新节点 |
| `DELETE` | `/api/nodes/:id` | 管理员 | 删除未被引用的节点 |
| `POST` | `/api/nodes/:id/install-tokens` | 管理员 | 创建一次性安装器 |
| `GET` | `/api/nodes/:id/install-script-preview` | 管理员 | 预览安装材料 |
| `POST` | `/api/nodes/:id/rotate-token` | 管理员 | 轮换长期节点 Token |

## Agent 协议

这些路由供 `backupx agent` 使用，Handler 内部通过节点 Token 认证。

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/agent/heartbeat` | Agent | 上报心跳与节点状态 |
| `POST` | `/api/agent/commands/poll` | Agent | 领取待执行命令 |
| `POST` | `/api/agent/commands/:id/result` | Agent | 上报命令结果 |
| `GET` | `/api/agent/tasks/:id` | Agent | 获取可执行任务规格 |
| `POST` | `/api/agent/records/:id` | Agent | 追加日志或更新备份状态 |
| `PUT` | `/api/agent/records/:id/artifacts/:targetId` | Agent | 向 Master 流式中转产物 |
| `GET` | `/api/agent/restores/:id/spec` | Agent | 获取恢复指令 |
| `GET` | `/api/agent/restores/:id/artifact` | Agent | 流式读取恢复产物 |
| `POST` | `/api/agent/restores/:id` | Agent | 更新恢复状态 |
| `GET` | `/api/v1/agent/self` | Agent | 安装时校验节点身份 |

## 公开运维与安装路由

| 方法 | 端点 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/health` | 公开 | 存活检查 |
| `GET` | `/api/health` | 公开 | 带 API 前缀的存活别名 |
| `GET` | `/ready` | 公开 | SQLite 就绪检查 |
| `GET` | `/api/ready` | 公开 | 带 API 前缀的就绪别名 |
| `GET` | `/metrics` | 公开 | Prometheus 指标 |
| `GET` | `/install/:token` | 公开 | 消费一次性 Agent 安装令牌 |
| `GET` | `/api/install/:token` | 公开 | 带 API 前缀的安装路由 |
| `GET` | `/install/:token/compose.yml` | 公开 | 生成 Docker Agent Compose |
| `GET` | `/api/install/:token/compose.yml` | 公开 | 带 API 前缀的 Docker Compose 路由 |

探针与指标应只对监控网段开放。安装 Token 是单次、限时秘密，不能写入公开日志。

## 响应格式

大多数 JSON 成功响应为：

~~~json
{
  "code": "OK",
  "message": "success",
  "data": {}
}
~~~

错误使用 HTTP 4xx 或 5xx，并带稳定业务码：

~~~json
{
  "code": "BACKUP_TASK_NOT_FOUND",
  "message": "备份任务不存在"
}
~~~

客户端应按 HTTP 状态和 `code` 分支，不要依赖本地化的 `message`。

产物下载、任务 JSON 导出、审计或合规导出、安装器响应和 `/metrics` 使用各自原生 Content-Type，不使用 JSON Envelope。日志与事件流使用 `text/event-stream`，反向代理必须关闭响应缓冲。
