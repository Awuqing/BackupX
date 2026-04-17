---
id: intro
slug: /intro
sidebar_position: 1
title: Introduction
description: Overview of BackupX — a self-hosted server backup management platform.
---

# BackupX

**BackupX** is a self-hosted server backup management platform. One static binary, one command, and every backup job for every server is under control.

- **Single binary + embedded SQLite** — no external database or orchestrator required
- **Files, databases, SAP HANA** — in one place, with a visual scheduler
- **70+ storage backends** — Alibaba OSS, Tencent COS, Qiniu, S3, Google Drive, WebDAV, FTP, plus SFTP / Azure Blob / Dropbox / OneDrive and dozens more via rclone
- **Multi-node cluster** — Master-Agent mode manages backups across servers, agents run tasks locally and upload straight to storage
- **Secure by default** — JWT auth, bcrypt, AES-256-GCM encrypted config, optional backup encryption, full audit log

## Architecture at a Glance

```
[Web Console] ─── JWT ──→ [Master (backupx)]
                             │
                             │ HTTP long-poll (token auth)
                             ▼
                         [Agent (backupx agent)]
                             │
                             ▼
                     [70+ Storage Backends]
```

Tasks routed to the local Master run in-process; tasks assigned to remote nodes are dispatched through a command queue and executed by the Agent locally. Agents only ever initiate outbound HTTP — no reverse connectivity required.

## Where to Next

- **New to BackupX?** Read the [Quick Start](/docs/getting-started/quick-start) first.
- **Deploying to production?** See the [Deployment Guide](/docs/deployment/docker).
- **SAP HANA operator?** Both `hdbsql` Runner and native Backint are supported — see [SAP HANA](/docs/features/sap-hana).
- **Managing multiple servers?** See [Multi-Node Cluster](/docs/features/multi-node).
- **Integrating programmatically?** See the [API Reference](/docs/reference/api).
