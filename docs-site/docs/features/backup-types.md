---
sidebar_position: 1
title: Backup Types
description: File, MySQL, PostgreSQL, SQLite and SAP HANA — what they back up and what to configure.
---

# Backup Types

BackupX supports five built-in backup types. Type determines which runner executes the job.

## File / Directory

Tars (and optionally gzips) one or more filesystem paths.

- **Source** accepts multiple paths — one per line in the UI
- **Exclude patterns** accept gitignore-style globs
- Supports following symlinks, preserving permissions
- Output is a single `.tar` or `.tar.gz` artifact

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
