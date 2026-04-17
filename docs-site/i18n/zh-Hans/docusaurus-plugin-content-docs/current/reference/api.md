---
sidebar_position: 1
title: API 参考
description: REST API 端点 — 统一以 /api 为前缀，使用 JWT Bearer 认证。
---

# API 参考

所有端点都以 `/api` 为前缀，使用 JWT Bearer 令牌认证（通过 `POST /api/auth/login` 获取）。Agent 专用端点使用 `X-Agent-Token` 头认证。

## 认证

| 端点 | 说明 |
|------|------|
| `POST /api/auth/setup` | 初始化首个管理员（仅当系统无任何用户时） |
| `POST /api/auth/login` | 登录，返回 JWT |
| `PUT  /api/auth/password` | 修改密码 |

## 备份任务

| 端点 | 说明 |
|------|------|
| `GET /api/backup/tasks` | 列表 |
| `POST /api/backup/tasks` | 创建 |
| `GET /api/backup/tasks/:id` | 详情 |
| `PUT /api/backup/tasks/:id` | 更新 |
| `DELETE /api/backup/tasks/:id` | 删除 |
| `PUT /api/backup/tasks/:id/toggle` | 启用 / 禁用 |
| `POST /api/backup/tasks/:id/run` | 手动执行 |

## 备份记录

| 端点 | 说明 |
|------|------|
| `GET /api/backup/records` | 列表（支持筛选） |
| `GET /api/backup/records/:id/logs/stream` | 实时日志（SSE） |
| `GET /api/backup/records/:id/download` | 下载备份 |
| `POST /api/backup/records/:id/restore` | 恢复到原始源 |

## 存储目标

| 端点 | 说明 |
|------|------|
| `GET /api/storage-targets` | 列表 |
| `POST /api/storage-targets` | 添加 |
| `POST /api/storage-targets/test` | 用待审核配置测试连接 |
| `GET /api/storage-targets/rclone/backends` | 列出可用 rclone 后端 |

## 节点（集群）

| 端点 | 说明 |
|------|------|
| `GET /api/nodes` | 节点列表 |
| `POST /api/nodes` | 创建节点并返回 Token |
| `PUT /api/nodes/:id` | 重命名 |
| `DELETE /api/nodes/:id` | 删除（有关联任务时会被拒绝） |
| `GET /api/nodes/:id/fs/list` | 浏览目录（远程节点走异步 RPC） |

## Agent 协议（X-Agent-Token）

| 端点 | 说明 |
|------|------|
| `POST /api/agent/heartbeat` | 上报心跳 |
| `POST /api/agent/commands/poll` | 领取一条待执行命令 |
| `POST /api/agent/commands/:id/result` | 上报命令结果 |
| `GET /api/agent/tasks/:id` | 拉取任务规格（含解密后的存储配置） |
| `POST /api/agent/records/:id` | 追加日志 / 更新记录状态 |

## 通知

| 端点 | 说明 |
|------|------|
| `GET /api/notifications` | 列表 |
| `POST /api/notifications` | 创建 |

## 仪表盘 / 审计 / 系统

| 端点 | 说明 |
|------|------|
| `GET /api/dashboard/stats` | 概览统计 |
| `GET /api/audit-logs` | 审计日志 |
| `GET /api/system/info` | 系统信息 |
| `GET /api/system/update-check` | 检查是否有新版本 |
