---
sidebar_position: 5
---

# MySQL binlog backups and point-in-time recovery

Enable **MySQL binlog / point-in-time recovery (PITR)** in a MySQL task. BackupX creates a whole-instance `mysqldump` baseline, then archives all binary logs since that baseline. Each log archive depends only on its full baseline, not on intermediate backup records. Full baselines are refreshed every 24 hours by default (configurable from 1 to 720 hours).

This is logical full backup plus binlog archiving, not XtraBackup or MariaBackup physical incremental backup. Supported servers are MySQL 8.0 / 8.4; install the MySQL 8.0.26+ `mysql`, `mysqldump` and `mysqlbinlog` clients on the execution node. MariaDB is outside this mode's compatibility scope.

## Setup

- Set the database name to `*`: PITR always includes the whole instance to preserve cross-database transactions.
- Enable binary logging and use InnoDB. Avoid DDL during the full dump; nontransactional tables are not guaranteed consistent.
- Use a fixed Master or Agent execution node. Dynamic node pools are unsupported.
- Configure the task schedule for the required archive frequency. A five-minute schedule normally yields an RPO of approximately five minutes plus execution time.
- Grant normal dump privileges plus log-position, brief read-lock and replication-read privileges (typically `REPLICATION CLIENT`, `REPLICATION SLAVE`, and `RELOAD`; consult your server version). Restore requires whole-instance write privileges and permission to set `sql_log_bin`.
- Do not reset binary logs, reset GTIDs or reinitialize the source during the recovery window; doing so invalidates old log coordinates.
- Keep source binlogs longer than the full-backup interval, archive interval and expected outage combined. BackupX does not change source log retention or purge logs. Missing logs, numbering gaps and changed server identity fail the archive. After correcting retention, wait for the next periodic full dump or create a new PITR task.
- Compression, multiple storage targets, retention and encryption use the normal task controls. Remote encryption requires upgrading both Master and Agent. Protect Agent-to-MySQL connections with a trusted network or database TLS tunnel.

The default Alpine Docker image supplies the MariaDB-compatible `mysql-client` package, not Oracle MySQL clients. For PITR, select a fixed Agent with Oracle MySQL 8.0.26+ clients installed, or supply a custom execution image with those tools. The default image's existing full-dump mode is unchanged. See the [Alpine package definition](https://pkgs.alpinelinux.org/package/v3.24/main/x86/mysql-client).

## Restore

Open a successful **MySQL binlog** record and choose Restore. Enter an optional RFC3339 timestamp with a time zone, such as `2026-11-14T12:30:00+08:00`. It must fall between baseline completion and the archive cutoff, excluding the cutoff second. Leave it empty to restore to the captured binary log position.

The UI displays the available interval. Times are converted to UTC; replay uses `mysqlbinlog --stop-datetime`, applying events before the requested second. Subsecond precision is not guaranteed. The window ends at successfully archived logs and excludes later source transactions.

Restore overwrites the **entire instance** currently configured in the task, including system objects and accounts. First point the task at an isolated recovery instance and verify business data. Import is not transactional; failure does not roll back already applied changes. Automated verification is unsupported for PITR; perform regular isolated recovery drills.

The existing `POST /api/backup/records/:id/restore` endpoint accepts an optional `restoreTime` field:

```json
{"restoreTime":"2026-11-14T12:30:00+08:00"}
```

Automatic retention and manual deletion protect full baselines while retained binlog records depend on them. Effective retention can therefore exceed the configured maximum count.

## Encryption and upgrades

Agents receive only a record-specific derived key; the Master key stays on the Master. New encrypted remote artifacts use `BXENC2`, authenticating record identity, chunk order and an explicit end marker. Master can still decrypt legacy `BXENC1` archives; restore those legacy archives on Master. Back up the Master configuration encryption key: losing or replacing it makes old encrypted backups unreadable. Key rotation and automatic migration of old keys are not included.
