---
sidebar_position: 1
title: API Reference
description: REST API endpoints — all under /api with JWT Bearer authentication.
---

# API Reference

All endpoints are prefixed with `/api` and authenticated with a JWT Bearer token, obtained via `POST /api/auth/login`. Agent endpoints use `X-Agent-Token` instead.

## Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/auth/setup/status` | Check whether admin initialization is needed |
| `POST` | `/api/auth/setup` | Initialize the first admin (only when no user exists) |
| `POST` | `/api/auth/login` | Log in and receive a JWT |
| `POST` | `/api/auth/logout` | Log out (invalidate current token) |
| `GET`  | `/api/auth/profile` | Current user profile |
| `PUT`  | `/api/auth/password` | Change password |

## Backup Tasks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/backup/tasks` | List tasks |
| `POST` | `/api/backup/tasks` | Create |
| `GET` | `/api/backup/tasks/:id` | Detail |
| `PUT` | `/api/backup/tasks/:id` | Update |
| `DELETE` | `/api/backup/tasks/:id` | Delete |
| `PUT` | `/api/backup/tasks/:id/toggle` | Enable / disable |
| `POST` | `/api/backup/tasks/:id/run` | Trigger a manual run |

## Backup Records

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/backup/records` | List records with filters |
| `GET` | `/api/backup/records/:id` | Record detail |
| `GET` | `/api/backup/records/:id/logs/stream` | Live logs (SSE) |
| `GET` | `/api/backup/records/:id/download` | Download the artifact |
| `POST` | `/api/backup/records/:id/restore` | Restore to the original source |
| `DELETE` | `/api/backup/records/:id` | Delete a record |
| `POST` | `/api/backup/records/batch-delete` | Bulk delete |

## Storage Targets

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/storage-targets` | List |
| `POST` | `/api/storage-targets` | Create |
| `GET` | `/api/storage-targets/:id` | Detail |
| `PUT` | `/api/storage-targets/:id` | Update |
| `DELETE` | `/api/storage-targets/:id` | Delete |
| `POST` | `/api/storage-targets/test` | Test connection with pending config |
| `POST` | `/api/storage-targets/:id/test` | Re-test a saved target |
| `PUT` | `/api/storage-targets/:id/star` | Toggle favourite |
| `GET` | `/api/storage-targets/:id/usage` | Query remote usage (where supported) |
| `GET` | `/api/storage-targets/rclone/backends` | List all available rclone backends |
| `POST` | `/api/storage-targets/google-drive/auth-url` | Start Google Drive OAuth |
| `POST` | `/api/storage-targets/google-drive/complete` | Complete OAuth flow |

## Nodes (Cluster)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/nodes` | List nodes |
| `POST` | `/api/nodes` | Create a node and return its token |
| `GET` | `/api/nodes/:id` | Node detail |
| `PUT` | `/api/nodes/:id` | Rename |
| `DELETE` | `/api/nodes/:id` | Delete (rejected if tasks are still attached) |
| `GET` | `/api/nodes/:id/fs/list` | Browse a directory (remote nodes use an async RPC via Agent) |

## Agent Protocol (X-Agent-Token)

Dedicated endpoints for the Agent CLI. Authenticated via the `X-Agent-Token` header instead of JWT.

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/agent/heartbeat` | Report liveness; returns the node ID |
| `POST` | `/api/agent/commands/poll` | Claim one pending command |
| `POST` | `/api/agent/commands/:id/result` | Report command result |
| `GET` | `/api/agent/tasks/:id` | Fetch task spec with decrypted storage configs |
| `POST` | `/api/agent/records/:id` | Append logs / update record status |

## Notifications

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/notifications` | List |
| `POST` | `/api/notifications` | Create |
| `GET` | `/api/notifications/:id` | Detail |
| `PUT` | `/api/notifications/:id` | Update |
| `DELETE` | `/api/notifications/:id` | Delete |
| `POST` | `/api/notifications/test` | Test with pending config |
| `POST` | `/api/notifications/:id/test` | Re-test a saved notifier |

## Dashboard

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/dashboard/stats` | Overview statistics |
| `GET` | `/api/dashboard/timeline` | Recent activity timeline |

## Audit / System / Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/audit-logs` | Audit log list |
| `GET` | `/api/system/info` | System information |
| `GET` | `/api/system/update-check` | Check for a newer release |
| `GET` | `/api/settings` | System-level settings |
| `PUT` | `/api/settings` | Update system settings |

## Response Envelope

All successful responses follow the shape:

```json
{
  "code": "OK",
  "message": "",
  "data": { /* actual payload */ }
}
```

Errors return an HTTP 4xx/5xx plus:

```json
{
  "code": "BACKUP_TASK_NOT_FOUND",
  "message": "备份任务不存在",
  "data": null
}
```
