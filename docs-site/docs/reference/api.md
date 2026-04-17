---
sidebar_position: 1
title: API Reference
description: REST API endpoints — all under /api with JWT Bearer authentication.
---

# API Reference

All endpoints are prefixed with `/api` and authenticated with a JWT Bearer token, obtained via `POST /api/auth/login`. Agent endpoints use `X-Agent-Token` instead.

## Authentication

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/setup` | Initialize the first admin (only when no user exists) |
| `POST /api/auth/login` | Log in and receive a JWT |
| `PUT  /api/auth/password` | Change password |

## Backup tasks

| Endpoint | Description |
|----------|-------------|
| `GET /api/backup/tasks` | List tasks |
| `POST /api/backup/tasks` | Create |
| `GET /api/backup/tasks/:id` | Detail |
| `PUT /api/backup/tasks/:id` | Update |
| `DELETE /api/backup/tasks/:id` | Delete |
| `PUT /api/backup/tasks/:id/toggle` | Enable / disable |
| `POST /api/backup/tasks/:id/run` | Manual run |

## Backup records

| Endpoint | Description |
|----------|-------------|
| `GET /api/backup/records` | List records with filters |
| `GET /api/backup/records/:id/logs/stream` | Live logs (SSE) |
| `GET /api/backup/records/:id/download` | Download artifact |
| `POST /api/backup/records/:id/restore` | Restore into the original source |

## Storage targets

| Endpoint | Description |
|----------|-------------|
| `GET /api/storage-targets` | List |
| `POST /api/storage-targets` | Create |
| `POST /api/storage-targets/test` | Test connection with pending config |
| `GET /api/storage-targets/rclone/backends` | List all available rclone backends |

## Nodes (cluster)

| Endpoint | Description |
|----------|-------------|
| `GET /api/nodes` | List nodes |
| `POST /api/nodes` | Create a node and return token |
| `PUT /api/nodes/:id` | Rename |
| `DELETE /api/nodes/:id` | Delete (rejected if tasks are attached) |
| `GET /api/nodes/:id/fs/list` | Browse directory (remote node = async RPC) |

## Agent protocol (X-Agent-Token)

| Endpoint | Description |
|----------|-------------|
| `POST /api/agent/heartbeat` | Report liveness |
| `POST /api/agent/commands/poll` | Claim one pending command |
| `POST /api/agent/commands/:id/result` | Report command result |
| `GET /api/agent/tasks/:id` | Fetch task spec with decrypted storage configs |
| `POST /api/agent/records/:id` | Append logs / update record status |

## Notifications

| Endpoint | Description |
|----------|-------------|
| `GET /api/notifications` | List |
| `POST /api/notifications` | Create |

## Dashboard / audit / system

| Endpoint | Description |
|----------|-------------|
| `GET /api/dashboard/stats` | Overview statistics |
| `GET /api/audit-logs` | Audit log list |
| `GET /api/system/info` | System information |
| `GET /api/system/update-check` | Check for a newer release |
