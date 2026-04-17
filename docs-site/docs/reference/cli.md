---
sidebar_position: 2
title: CLI Reference
description: backupx subcommands — server, agent, backint, reset-password.
---

# CLI Reference

The `backupx` binary ships several subcommands. Running `backupx` with no subcommand starts the main server process.

## `backupx` (default: server)

```bash
backupx --config /opt/backupx/config.yaml
backupx --version
```

| Flag | Description |
|------|-------------|
| `--config <path>` | Path to config YAML (default: `./config.yaml`) |
| `--version` | Print version and exit |

## `backupx agent`

Run in Agent mode, connecting to a Master. See [Multi-Node Cluster](../features/multi-node).

```bash
backupx agent --master http://master:8340 --token <token>
```

| Flag | Description |
|------|-------------|
| `--master <url>` | Master URL |
| `--token <token>` | Agent auth token |
| `--config <path>` | YAML config (takes precedence over env) |
| `--temp-dir <path>` | Local temp directory (default `/tmp/backupx-agent`) |
| `--insecure-tls` | Skip TLS verification (testing only) |

Environment variables: `BACKUPX_AGENT_MASTER`, `BACKUPX_AGENT_TOKEN`, `BACKUPX_AGENT_HEARTBEAT`, `BACKUPX_AGENT_POLL`, `BACKUPX_AGENT_TEMP_DIR`, `BACKUPX_AGENT_INSECURE_TLS`.

## `backupx backint`

SAP HANA Backint protocol agent. See [SAP HANA Support](../features/sap-hana).

```bash
backupx backint -f <function> -i <input> -o <output> -p <params>
```

| Flag | Description |
|------|-------------|
| `-f <fn>` | `backup` / `restore` / `inquire` / `delete` |
| `-i <path>` | Input file |
| `-o <path>` | Output file |
| `-p <path>` | Parameter file |
| `-u / -c / -l / -v` | Accepted and ignored for SAP compatibility |

## `backupx reset-password`

Reset an admin password directly in the SQLite database. No server restart needed.

```bash
backupx reset-password --username admin --password 'newpass123' [--config /path/to/config.yaml]
```

| Flag | Description |
|------|-------------|
| `--username` | Target username (default: `admin`) |
| `--password` | New password (min 8 chars, required) |
| `--config` | Config path (used to locate the database file) |
