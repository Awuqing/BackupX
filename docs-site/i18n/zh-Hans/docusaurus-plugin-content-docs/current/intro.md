---
id: intro
slug: /intro
sidebar_position: 1
title: 项目简介
description: BackupX——自托管服务器备份管理平台概览。
---

# BackupX

**BackupX** 是一款自托管的服务器备份管理平台：一个二进制，一条命令，管好所有服务器的所有备份。

- **单二进制 + 内嵌 SQLite** — 不依赖外部数据库或编排器
- **文件、数据库、SAP HANA** — 统一管理，可视化调度
- **70+ 存储后端** — 阿里云 OSS、腾讯云 COS、七牛、S3、Google Drive、WebDAV、FTP，以及通过 rclone 接入的 SFTP / Azure Blob / Dropbox / OneDrive 等数十种
- **多节点集群** — Master-Agent 模式跨服务器管理备份，Agent 在本地执行并直接上传到存储
- **默认安全** — JWT 认证、bcrypt、AES-256-GCM 加密配置、可选备份加密、完整审计日志

## 架构概览

```
[Web 控制台] ─── JWT ──→ [Master (backupx)]
                             │
                             │ HTTP 长轮询（Token 认证）
                             ▼
                         [Agent (backupx agent)]
                             │
                             ▼
                       [70+ 存储后端]
```

路由到本机的任务在 Master 进程内直接执行；派到远程节点的任务通过命令队列下发，由 Agent 在本地执行。Agent 只发起出站 HTTP 连接 — 不需要任何反向连通性。

## 下一步

- **第一次使用 BackupX？** 先看 [快速开始](/docs/getting-started/quick-start)
- **生产部署？** 参考 [部署指南](/docs/deployment/docker)
- **SAP HANA 用户？** 支持 `hdbsql` Runner 和原生 Backint 两种模式 — 详见 [SAP HANA](/docs/features/sap-hana)
- **管理多台服务器？** 参考 [多节点集群](/docs/features/multi-node)
- **程序化集成？** 参考 [API 参考](/docs/reference/api)
