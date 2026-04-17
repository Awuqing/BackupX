---
sidebar_position: 3
title: SAP HANA Support
description: Two SAP HANA backup modes — managed hdbsql runner and native Backint protocol agent.
---

# SAP HANA Support

BackupX provides two SAP HANA backup modes. Pick whichever fits your operations workflow.

## Mode 1: hdbsql Runner (console-managed)

Create a SAP HANA backup task in the Web console. The backend invokes `hdbsql` to execute the backup. Use this when BackupX should own the schedule.

**Source configuration supports:**

| Field | Options | Description |
|-------|---------|-------------|
| Backup type | `data` / `log` | Data or log backup |
| Backup level | `full` / `incremental` / `differential` | Auto-disabled for log backups |
| Parallel channels | `1 ~ 32` | Multi-path SQL (`BACKUP DATA USING FILE ('c1', 'c2', ...)`) |
| Retry count | `1 ~ 10` | Exponential backoff (`5s × attempt²`) |
| Instance number | Optional | Inferred from port or specified manually |

## Mode 2: Backint Protocol Agent (HANA native)

BackupX ships a built-in Backint Agent. SAP HANA calls it via the native `BACKUP DATA USING BACKINT` syntax, and data is routed automatically to any BackupX storage target (S3 / OSS / COS / WebDAV / 70+ backends).

### 1. Parameter file

```ini title="/opt/backupx/backint_params.ini"
#STORAGE_TYPE = s3
#STORAGE_CONFIG_JSON = /opt/backupx/storage.json
#PARALLEL_FACTOR = 4
#COMPRESS = true
#KEY_PREFIX = hana-backup
#CATALOG_DB = /opt/backupx/backint_catalog.db
#LOG_FILE = /var/log/backupx/backint.log
```

### 2. Storage config (same schema as storage targets)

```json title="/opt/backupx/storage.json"
{
  "endpoint": "https://s3.amazonaws.com",
  "region": "us-east-1",
  "bucket": "hana-prod",
  "accessKeyId": "AKIA...",
  "secretAccessKey": "..."
}
```

### 3. Create the hdbbackint symlink

```bash
ln -s /opt/backupx/backupx /usr/sap/<SID>/SYS/global/hdb/opt/hdbbackint
```

### 4. Enable Backint in HANA `global.ini`

```ini
[backup]
data_backup_using_backint = true
catalog_backup_using_backint = true
log_backup_using_backint = true
data_backup_parameter_file = /opt/backupx/backint_params.ini
log_backup_parameter_file = /opt/backupx/backint_params.ini
```

### 5. Manual CLI invocation (troubleshooting)

```bash
backupx backint -f backup  -i input.txt -o output.txt -p backint_params.ini
backupx backint -f restore -i input.txt -o output.txt -p backint_params.ini
backupx backint -f inquire -i input.txt -o output.txt -p backint_params.ini
backupx backint -f delete  -i input.txt -o output.txt -p backint_params.ini
```

The Backint Agent maintains an `EBID ↔ object-key` catalog in a local SQLite DB. All operations follow the SAP HANA Backint protocol (`#PIPE` / `#SAVED` / `#RESTORED` / `#BACKUP` / `#NOTFOUND` / `#DELETED` / `#ERROR`).
