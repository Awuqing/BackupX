---
sidebar_position: 1
title: API Reference
description: BackupX REST endpoints, authentication methods, role boundaries, streaming responses, and public probes.
---

# API Reference

The interactive API is rooted at `/api`. Most endpoints accept either a user JWT or an API key; Agent protocol endpoints use a node-specific token. Public probes and one-time installers are listed separately.

## Authentication

### User JWT

Obtain a JWT through `POST /api/auth/login` and send it as a Bearer token:

~~~bash
curl -H "Authorization: Bearer $BACKUPX_TOKEN" \
  https://backup.example.com/api/backup/tasks
~~~

The login flow may require OTP, TOTP, recovery code, a trusted-device token, or WebAuthn depending on account and system settings.

### API key

An administrator creates API keys in the console or through `POST /api/api-keys`. The plaintext `bax_...` value is returned only once.

~~~bash
curl -H "X-Api-Key: $BACKUPX_API_KEY" \
  https://backup.example.com/api/dashboard/stats
~~~

`Authorization: Bearer bax_...` is also accepted. API keys carry an `admin`, `operator`, or `viewer` role and can be disabled or given an expiry.

### Agent token

Agent protocol handlers authenticate the node token supplied in `X-Agent-Token`. This token is not a user credential and must not be used with the interactive resource API.

### Access labels

The tables use these labels:

| Label | Required access |
| --- | --- |
| Public | No JWT or API key; an install route still requires its one-time token |
| Auth | Any authenticated `viewer`, `operator`, or `admin` |
| Operator | `operator` or `admin` |
| Admin | `admin` only |
| Agent | Valid node-specific Agent token |

Viewers can use read endpoints except node filesystem browsing. Operators can run and mutate backup resources. Administrators additionally manage users, API keys, settings, nodes, install tokens, and node-token rotation. A rejected role returns HTTP 403.

## Authentication and account security

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/auth/setup/status` | Public | Check whether first-admin setup is required |
| `POST` | `/api/auth/setup` | Public | Create the first administrator when no user exists |
| `POST` | `/api/auth/login` | Public | Complete password or MFA login and obtain a JWT |
| `POST` | `/api/auth/otp/send` | Public | Send a configured login OTP |
| `POST` | `/api/auth/webauthn/login/options` | Public | Begin passkey login |
| `POST` | `/api/auth/logout` | Auth | Acknowledge logout; the client must discard its stateless JWT |
| `GET` | `/api/auth/profile` | Auth | Read the current account |
| `PUT` | `/api/auth/password` | Auth | Change the current account password |
| `POST` | `/api/auth/2fa/setup` | Auth | Prepare TOTP enrollment |
| `POST` | `/api/auth/2fa/enable` | Auth | Enable TOTP after verification |
| `POST` | `/api/auth/2fa/recovery-codes` | Auth | Regenerate recovery codes |
| `DELETE` | `/api/auth/2fa` | Auth | Disable TOTP |
| `PUT` | `/api/auth/otp/config` | Auth | Update OTP login configuration |
| `POST` | `/api/auth/webauthn/register/options` | Auth | Begin passkey registration |
| `POST` | `/api/auth/webauthn/register/finish` | Auth | Finish passkey registration |
| `GET` | `/api/auth/webauthn/credentials` | Auth | List passkeys |
| `DELETE` | `/api/auth/webauthn/credentials/:id` | Auth | Delete a passkey |
| `GET` | `/api/auth/trusted-devices` | Auth | List trusted devices |
| `DELETE` | `/api/auth/trusted-devices/:id` | Auth | Revoke a trusted device |

Use an interactive JWT, not an automation API key, for account-security endpoints.

## System and storage targets

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/system/info` | Auth | Version and system information |
| `GET` | `/api/system/update-check` | Auth | Check available releases |
| `GET` | `/api/storage-targets` | Auth | List storage targets |
| `POST` | `/api/storage-targets` | Operator | Create a target |
| `POST` | `/api/storage-targets/test` | Operator | Test an unsaved configuration |
| `GET` | `/api/storage-targets/rclone/backends` | Auth | List available rclone backends |
| `POST` | `/api/storage-targets/google-drive/auth-url` | Operator | Start Google Drive authorization |
| `POST` | `/api/storage-targets/google-drive/complete` | Operator | Complete Google Drive authorization |
| `GET` | `/api/storage-targets/google-drive/callback` | Auth | Handle the OAuth callback |
| `GET` | `/api/storage-targets/:id` | Auth | Read a target |
| `PUT` | `/api/storage-targets/:id` | Operator | Update a target |
| `DELETE` | `/api/storage-targets/:id` | Operator | Delete a target |
| `PUT` | `/api/storage-targets/:id/star` | Operator | Toggle favorite state |
| `POST` | `/api/storage-targets/:id/test` | Operator | Test a saved target |
| `GET` | `/api/storage-targets/:id/usage` | Auth | Read recorded usage |
| `GET` | `/api/storage-targets/:id/google-drive/profile` | Auth | Read the connected Google Drive profile |

## Backup tasks

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/backup/tasks` | Auth | List tasks |
| `GET` | `/api/backup/tasks/tags` | Auth | List task tags |
| `GET` | `/api/backup/tasks/export` | Auth | Download all task definitions, or select them with `?ids=1,2` |
| `POST` | `/api/backup/tasks/import` | Operator | Import task definitions, up to 1 MiB |
| `POST` | `/api/backup/tasks/batch/toggle` | Operator | Enable or disable tasks in bulk |
| `POST` | `/api/backup/tasks/batch/delete` | Operator | Delete tasks in bulk |
| `POST` | `/api/backup/tasks/batch/run` | Operator | Run tasks in bulk |
| `GET` | `/api/backup/tasks/:id` | Auth | Read a task |
| `POST` | `/api/backup/tasks` | Operator | Create a task |
| `PUT` | `/api/backup/tasks/:id` | Operator | Update a task |
| `DELETE` | `/api/backup/tasks/:id` | Operator | Delete a task |
| `PUT` | `/api/backup/tasks/:id/toggle` | Operator | Enable or disable a task |
| `POST` | `/api/backup/tasks/:id/run` | Operator | Trigger a backup |
| `POST` | `/api/backup/tasks/:id/verify` | Operator | Trigger verification from a task |

Task export intentionally excludes database passwords and storage credentials. It is useful for migration and review, not a complete control-plane backup.

## Backup and restore records

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/backup/records` | Auth | List and filter backup records |
| `POST` | `/api/backup/records/batch-delete` | Operator | Delete records in bulk |
| `GET` | `/api/backup/records/:id` | Auth | Read a backup record |
| `GET` | `/api/backup/records/:id/logs/stream` | Auth | Stream logs with server-sent events |
| `GET` | `/api/backup/records/:id/download` | Auth | Download an artifact |
| `GET` | `/api/backup/records/:id/contents` | Auth | Browse artifact contents where supported |
| `POST` | `/api/backup/records/:id/restore` | Operator | Start a restore |
| `POST` | `/api/backup/records/:id/replicate` | Operator | Replicate an existing artifact |
| `POST` | `/api/backup/records/:id/verify` | Operator | Verify an existing artifact |
| `PUT` | `/api/backup/records/:id/lock` | Operator | Set retention lock state |
| `DELETE` | `/api/backup/records/:id` | Operator | Delete a record and its managed artifact |
| `GET` | `/api/restore/records` | Auth | List restore records |
| `GET` | `/api/restore/records/:id` | Auth | Read a restore record |
| `GET` | `/api/restore/records/:id/logs/stream` | Auth | Stream restore logs |
| `GET` | `/api/replication/records` | Auth | List replication records |
| `GET` | `/api/replication/records/:id` | Auth | Read a replication record |
| `GET` | `/api/verify/records` | Auth | List verification records |
| `GET` | `/api/verify/records/:id` | Auth | Read a verification record |
| `GET` | `/api/verify/records/:id/logs/stream` | Auth | Stream verification logs |

## Templates, reports, and dashboard

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/task-templates` | Auth | List task templates |
| `GET` | `/api/task-templates/:id` | Auth | Read a task template |
| `POST` | `/api/task-templates` | Operator | Create a template |
| `PUT` | `/api/task-templates/:id` | Operator | Update a template |
| `DELETE` | `/api/task-templates/:id` | Operator | Delete a template |
| `POST` | `/api/task-templates/:id/apply` | Operator | Create tasks from a template |
| `GET` | `/api/reports/compliance` | Auth | Read compliance evidence |
| `GET` | `/api/reports/compliance/export` | Auth | Export compliance evidence as CSV |
| `GET` | `/api/dashboard/stats` | Auth | Summary statistics |
| `GET` | `/api/dashboard/timeline` | Auth | Recent activity |
| `GET` | `/api/dashboard/sla` | Auth | RPO and SLA status |
| `GET` | `/api/dashboard/cluster` | Auth | Cluster summary |
| `GET` | `/api/dashboard/breakdown` | Auth | Task and record breakdown |
| `GET` | `/api/dashboard/node-performance` | Auth | Per-node performance |

## Notifications, settings, and administration

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/notifications` | Auth | List notification channels |
| `GET` | `/api/notifications/:id` | Auth | Read a channel |
| `POST` | `/api/notifications` | Operator | Create a channel |
| `PUT` | `/api/notifications/:id` | Operator | Update a channel |
| `DELETE` | `/api/notifications/:id` | Operator | Delete a channel |
| `POST` | `/api/notifications/test` | Operator | Test an unsaved configuration |
| `POST` | `/api/notifications/:id/test` | Operator | Test a saved channel |
| `GET` | `/api/settings` | Auth | Read system settings |
| `PUT` | `/api/settings` | Admin | Update system settings |
| `GET` | `/api/users` | Admin | List users |
| `POST` | `/api/users` | Admin | Create a user |
| `PUT` | `/api/users/:id` | Admin | Update a user |
| `POST` | `/api/users/:id/2fa/reset` | Admin | Reset a user's second factor |
| `DELETE` | `/api/users/:id` | Admin | Delete a user |
| `GET` | `/api/api-keys` | Admin | List API keys without plaintext values |
| `POST` | `/api/api-keys` | Admin | Create an API key and return its plaintext once |
| `PUT` | `/api/api-keys/:id/toggle` | Admin | Enable or disable an API key |
| `DELETE` | `/api/api-keys/:id` | Admin | Revoke an API key |

## Audit, events, search, and discovery

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/audit-logs` | Auth | List and filter audit records |
| `GET` | `/api/audit-logs/export` | Auth | Export audit records |
| `GET` | `/api/events/stream` | Auth | Stream real-time application events with SSE |
| `GET` | `/api/search` | Auth | Search supported resources |
| `POST` | `/api/database/discover` | Auth | Discover databases from supplied connection details |

## Nodes

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/api/nodes` | Auth | List nodes |
| `GET` | `/api/nodes/:id` | Auth | Read a node |
| `GET` | `/api/nodes/:id/fs/list` | Operator | Browse the selected node filesystem |
| `POST` | `/api/nodes` | Admin | Create a node |
| `POST` | `/api/nodes/batch` | Admin | Create up to 50 nodes |
| `PUT` | `/api/nodes/:id` | Admin | Update a node |
| `DELETE` | `/api/nodes/:id` | Admin | Delete an unreferenced node |
| `POST` | `/api/nodes/:id/install-tokens` | Admin | Create a one-time installer |
| `GET` | `/api/nodes/:id/install-script-preview` | Admin | Preview generated install material |
| `POST` | `/api/nodes/:id/rotate-token` | Admin | Rotate the long-lived node token |

## Agent protocol

These routes are for the `backupx agent` process and authenticate inside the handler with the node token.

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `POST` | `/api/agent/heartbeat` | Agent | Report liveness and node state |
| `POST` | `/api/agent/commands/poll` | Agent | Claim a pending command |
| `POST` | `/api/agent/commands/:id/result` | Agent | Report a command result |
| `GET` | `/api/agent/tasks/:id` | Agent | Fetch a runnable task specification |
| `POST` | `/api/agent/records/:id` | Agent | Append logs or update backup state |
| `PUT` | `/api/agent/records/:id/artifacts/:targetId` | Agent | Stream a relayed artifact to the Master |
| `GET` | `/api/agent/restores/:id/spec` | Agent | Fetch restore instructions |
| `GET` | `/api/agent/restores/:id/artifact` | Agent | Stream a restore artifact |
| `POST` | `/api/agent/restores/:id` | Agent | Update restore state |
| `GET` | `/api/v1/agent/self` | Agent | Validate node identity during installation |

## Public operational and install routes

| Method | Endpoint | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/health` | Public | Liveness |
| `GET` | `/api/health` | Public | API-prefixed liveness alias |
| `GET` | `/ready` | Public | SQLite readiness |
| `GET` | `/api/ready` | Public | API-prefixed readiness alias |
| `GET` | `/metrics` | Public | Prometheus metrics |
| `GET` | `/install/:token` | Public | Consume a one-time Agent installer token |
| `GET` | `/api/install/:token` | Public | API-prefixed installer route |
| `GET` | `/install/:token/compose.yml` | Public | Render a Docker Agent Compose file |
| `GET` | `/api/install/:token/compose.yml` | Public | API-prefixed Docker Compose route |

Restrict probes and metrics to monitoring networks. Install tokens are single-use, time-limited secrets and must not be written to public logs.

## Response formats

Most JSON successes use:

~~~json
{
  "code": "OK",
  "message": "success",
  "data": {}
}
~~~

Errors use an HTTP 4xx or 5xx status plus a stable application code:

~~~json
{
  "code": "BACKUP_TASK_NOT_FOUND",
  "message": "备份任务不存在"
}
~~~

Clients should branch on the HTTP status and `code`, not the localized `message`.

Artifact downloads, task JSON export, audit or compliance exports, installer responses, and `/metrics` return their native content types instead of the JSON envelope. Log and event streams use `text/event-stream`; reverse proxies must keep response buffering disabled.
