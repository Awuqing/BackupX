---
sidebar_position: 1
title: 备份类型
description: 文件、MySQL、PostgreSQL、SQLite 和 SAP HANA — 各自的能力与配置说明。
---

# 备份类型

BackupX 支持五种内置备份类型，类型决定了用哪个 runner 执行。

## 文件 / 目录

打包（可选 gzip）一个或多个文件系统路径。

- **源路径** 支持多个（UI 中每行一个）
- **排除模式** 支持 gitignore 风格的通配符
- 可选跟随符号链接、保留权限
- 输出单个 `.tar` 或 `.tar.gz`

## MySQL

底层使用 `mysqldump`，需要执行任务的主机（Master 或 Agent）的 `$PATH` 中有 `mysqldump`。

- **主机 / 端口 / 用户 / 密码 / 数据库** — 支持多库（英文逗号分隔）
- 输出：`.sql` 或 `.sql.gz`
- 默认参数：`--single-transaction --routines --triggers --events`

## PostgreSQL

底层使用 `pg_dump`，连接字段与 MySQL 一致加数据库名。

## SQLite

直接复制数据库文件（使用一致性快照），无需外部工具。

## SAP HANA

支持两种模式 — 详见 [SAP HANA](./sap-hana) 专题页。

## 删除行为

删除备份任务时，BackupX 会从所有存储目标上移除备份产物，但保留备份记录以供审计。删除任务同时拆除其 Cron 定时调度。
