---
sidebar_position: 3
title: SAP HANA 支持
description: 两种 SAP HANA 备份模式 — 控制台托管的 hdbsql Runner 和原生 Backint 协议代理。
---

# SAP HANA 支持

BackupX 提供两种 SAP HANA 备份模式，按实际运维流程选择。

## 模式一：hdbsql Runner（控制台托管）

通过 Web 控制台创建 SAP HANA 备份任务，后端调用 `hdbsql` 执行备份。适合希望 BackupX 来管理调度的场景。

**源配置支持：**

| 字段 | 可选值 | 说明 |
|------|--------|------|
| 备份类型 | `data` / `log` | 数据备份或日志备份 |
| 备份级别 | `full` / `incremental` / `differential` | 日志备份时自动禁用 |
| 并行通道数 | `1 ~ 32` | 多路径 SQL（`BACKUP DATA USING FILE ('c1', 'c2', ...)`） |
| 失败重试次数 | `1 ~ 10` | 指数退避（`5s × 尝试次数²`） |
| 实例编号 | 可选 | 从端口推断或手动指定 |

## 模式二：Backint 协议代理（HANA 原生接口）

BackupX 内置 Backint Agent，SAP HANA 通过原生 `BACKUP DATA USING BACKINT` 语法调用，数据自动路由到任意 BackupX 存储目标（S3 / OSS / COS / WebDAV / 70+ 后端）。

### 1. 参数文件

```ini title="/opt/backupx/backint_params.ini"
#STORAGE_TYPE = s3
#STORAGE_CONFIG_JSON = /opt/backupx/storage.json
#PARALLEL_FACTOR = 4
#COMPRESS = true
#KEY_PREFIX = hana-backup
#CATALOG_DB = /opt/backupx/backint_catalog.db
#LOG_FILE = /var/log/backupx/backint.log
```

### 2. 存储配置（与存储目标 schema 相同）

```json title="/opt/backupx/storage.json"
{
  "endpoint": "https://s3.amazonaws.com",
  "region": "us-east-1",
  "bucket": "hana-prod",
  "accessKeyId": "AKIA...",
  "secretAccessKey": "..."
}
```

### 3. 创建 hdbbackint 软链接

```bash
ln -s /opt/backupx/backupx /usr/sap/<SID>/SYS/global/hdb/opt/hdbbackint
```

### 4. 在 HANA `global.ini` 中启用

```ini
[backup]
data_backup_using_backint = true
catalog_backup_using_backint = true
log_backup_using_backint = true
data_backup_parameter_file = /opt/backupx/backint_params.ini
log_backup_parameter_file = /opt/backupx/backint_params.ini
```

### 5. CLI 手动调用（用于排查）

```bash
backupx backint -f backup  -i input.txt -o output.txt -p backint_params.ini
backupx backint -f restore -i input.txt -o output.txt -p backint_params.ini
backupx backint -f inquire -i input.txt -o output.txt -p backint_params.ini
backupx backint -f delete  -i input.txt -o output.txt -p backint_params.ini
```

Backint Agent 使用本地 SQLite 维护 `EBID ↔ 对象键` 目录，所有操作遵循 SAP HANA Backint 协议（`#PIPE` / `#SAVED` / `#RESTORED` / `#BACKUP` / `#NOTFOUND` / `#DELETED` / `#ERROR`）。
