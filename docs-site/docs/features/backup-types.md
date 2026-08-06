---
sidebar_position: 1
title: Backup Types
description: File, MySQL, PostgreSQL, SQLite and SAP HANA — what they back up and what to configure.
---

# Backup Types

BackupX supports five built-in backup types. Type determines which runner executes the job.

When a task is routed to a remote Agent, the source tools and paths are resolved on that Agent host. Multi-target uploads are still tracked per storage target; if at least one target succeeds, the backup record is marked successful and the per-target result table shows partial failures.

## File / Directory

File tasks offer three backup modes:

- **Full archive** — writes a self-contained tar artifact on every run
- **Differential archive** — writes only changes since the current full baseline and periodically refreshes that baseline
- **CDC repository** — splits content with stable 512 KiB / 1 MiB / 4 MiB boundaries, stores new chunks in immutable 32 MiB packs, and writes a small snapshot manifest for each run

The CDC repository deduplicates identical content across files and snapshots. Restore, selective restore, verification, download-as-tar, retention, and garbage collection all resolve data through the repository index. Compression and encryption are applied per chunk; encrypted repositories use keyed chunk IDs so plaintext hashes are not exposed.

Repository mode currently uses a single-writer index and therefore runs on the Master only. To keep repository copies on multiple backends, select multiple primary storage targets on the task. Object-level replication is intentionally disabled because a snapshot manifest without its shared packs and indexes is not a complete backup.

Common file-task options:

- **Source** accepts multiple paths — one per line in the UI
- **Exclude patterns** accept gitignore-style globs
- Supports following symlinks, preserving permissions
- Full and differential modes output `.tar`, `.tar.gz`, or `.tar.zst` artifacts

## MySQL

Uses `mysqldump` under the hood. Requires `mysqldump` to be on `$PATH` of the host running the task (Master or Agent).

- **Host / port / user / password / database** — multi-database allowed (comma-separated)
- Output: `.sql` or `.sql.gz`
- Default flags: `--single-transaction --routines --triggers --events`

## PostgreSQL

Uses `pg_dump`. Same connection fields as MySQL plus database name.

## SQLite

Copies the database file directly (with a consistency snapshot). No external tool required.

## SAP HANA

Two modes are supported — see the dedicated [SAP HANA](./sap-hana) page.

## Deletion behavior

When a task is deleted, BackupX removes backup artifacts from every storage target but preserves backup records for audit. Task deletion also tears down the cron schedule entry.
