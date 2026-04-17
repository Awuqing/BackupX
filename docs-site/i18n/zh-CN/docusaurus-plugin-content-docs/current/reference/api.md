---
sidebar_position: 1
title: API 参考
description: REST API 端点 — 统一以 /api 为前缀，使用 JWT Bearer 认证。
---

# API 参考

所有端点都以 `/api` 为前缀，使用 JWT Bearer 令牌认证（通过 `POST /api/auth/login` 获取）。Agent 专用端点使用 `X-Agent-Token` 头认证。

## 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/auth/setup/status` | 查询是否需要初始化管理员 |
| `POST` | `/api/auth/setup` | 初始化首个管理员（仅当系统无任何用户时） |
| `POST` | `/api/auth/login` | 登录，返回 JWT |
| `POST` | `/api/auth/logout` | 登出（使当前 Token 失效） |
| `GET`  | `/api/auth/profile` | 当前用户信息 |
| `PUT`  | `/api/auth/password` | 修改密码 |

## 备份任务

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/backup/tasks` | 列表 |
| `POST` | `/api/backup/tasks` | 创建 |
| `GET` | `/api/backup/tasks/:id` | 详情 |
| `PUT` | `/api/backup/tasks/:id` | 更新 |
| `DELETE` | `/api/backup/tasks/:id` | 删除 |
| `PUT` | `/api/backup/tasks/:id/toggle` | 启用 / 禁用 |
| `POST` | `/api/backup/tasks/:id/run` | 手动触发一次执行 |

## 备份记录

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/backup/records` | 列表（支持筛选） |
| `GET` | `/api/backup/records/:id` | 记录详情 |
| `GET` | `/api/backup/records/:id/logs/stream` | 实时日志（SSE） |
| `GET` | `/api/backup/records/:id/download` | 下载备份产物 |
| `POST` | `/api/backup/records/:id/restore` | 恢复到原始源 |
| `DELETE` | `/api/backup/records/:id` | 删除记录 |
| `POST` | `/api/backup/records/batch-delete` | 批量删除 |

## 存储目标

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/storage-targets` | 列表 |
| `POST` | `/api/storage-targets` | 创建 |
| `GET` | `/api/storage-targets/:id` | 详情 |
| `PUT` | `/api/storage-targets/:id` | 更新 |
| `DELETE` | `/api/storage-targets/:id` | 删除 |
| `POST` | `/api/storage-targets/test` | 用待审核配置测试连接 |
| `POST` | `/api/storage-targets/:id/test` | 重测已保存的目标 |
| `PUT` | `/api/storage-targets/:id/star` | 切换收藏状态 |
| `GET` | `/api/storage-targets/:id/usage` | 查询远端存储用量（支持此能力的后端） |
| `GET` | `/api/storage-targets/rclone/backends` | 列出可用的 rclone 后端 |
| `POST` | `/api/storage-targets/google-drive/auth-url` | 启动 Google Drive OAuth |
| `POST` | `/api/storage-targets/google-drive/complete` | 完成 OAuth 流程 |

## 节点（集群）

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/nodes` | 节点列表 |
| `POST` | `/api/nodes` | 创建节点并返回 Token |
| `GET` | `/api/nodes/:id` | 节点详情 |
| `PUT` | `/api/nodes/:id` | 重命名 |
| `DELETE` | `/api/nodes/:id` | 删除（有关联任务时会被拒绝） |
| `GET` | `/api/nodes/:id/fs/list` | 浏览目录（远程节点走 Agent 异步 RPC） |

## Agent 协议（X-Agent-Token）

Agent CLI 专用端点，通过 `X-Agent-Token` 头认证而非 JWT。

| 方法 | 端点 | 说明 |
|------|------|------|
| `POST` | `/api/agent/heartbeat` | 上报心跳（返回节点 ID） |
| `POST` | `/api/agent/commands/poll` | 领取一条待执行命令 |
| `POST` | `/api/agent/commands/:id/result` | 上报命令结果 |
| `GET` | `/api/agent/tasks/:id` | 拉取任务规格（含解密后的存储配置） |
| `POST` | `/api/agent/records/:id` | 追加日志 / 更新记录状态 |

## 通知

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/notifications` | 列表 |
| `POST` | `/api/notifications` | 创建 |
| `GET` | `/api/notifications/:id` | 详情 |
| `PUT` | `/api/notifications/:id` | 更新 |
| `DELETE` | `/api/notifications/:id` | 删除 |
| `POST` | `/api/notifications/test` | 用待审核配置测试 |
| `POST` | `/api/notifications/:id/test` | 重测已保存的通知器 |

## 仪表盘

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/dashboard/stats` | 概览统计 |
| `GET` | `/api/dashboard/timeline` | 最近活动时间线 |

## 审计 / 系统 / 设置

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/audit-logs` | 审计日志 |
| `GET` | `/api/system/info` | 系统信息 |
| `GET` | `/api/system/update-check` | 检查新版本 |
| `GET` | `/api/settings` | 系统级设置 |
| `PUT` | `/api/settings` | 更新系统设置 |

## 响应结构

成功响应统一为：

```json
{
  "code": "OK",
  "message": "",
  "data": { /* 实际数据 */ }
}
```

错误返回 HTTP 4xx/5xx，并带：

```json
{
  "code": "BACKUP_TASK_NOT_FOUND",
  "message": "备份任务不存在",
  "data": null
}
```
