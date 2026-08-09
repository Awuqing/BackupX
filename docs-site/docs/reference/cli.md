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
| `--config <path>` | Explicit config YAML path; omitted uses the search paths below |
| `--version` | Print version and exit |

When `--config` is omitted, the server searches `./config.yaml`, `./server/config.yaml`, and `/etc/backupx/config.yaml`. `BACKUPX_*` environment variables override matching server configuration keys. See [Configuration Reference](../deployment/configuration).

## `backupx agent`

Run in Agent mode, connecting to a Master. See [Multi-Node Cluster](../features/multi-node).

```bash
backupx agent --master https://backup.example.com --token-file /etc/backupx-agent/agent.token
```

| Flag | Description |
|------|-------------|
| `--master <url>` | Master URL |
| `--token <token>` | Agent auth token |
| `--token-file <path>` | Read the Agent Token from a file; preferred for services and containers |
| `--config <path>` | Load Agent YAML; when present, environment-based Agent config is not loaded |
| `--temp-dir <path>` | Local temp directory (default `/var/lib/backupx-agent/tmp`) |
| `--proxy-url <url>` | Explicit HTTP(S) or SOCKS5(H) proxy |
| `--ca-cert <path>` | PEM CA certificate used to verify the Master |
| `--insecure-tls` | Skip TLS verification (testing only) |

Agent precedence is explicit CLI flags over a YAML file. If `--config` is not supplied, Agent settings are loaded from `BACKUPX_AGENT_MASTER`, `BACKUPX_AGENT_TOKEN`, `BACKUPX_AGENT_TOKEN_FILE`, `BACKUPX_AGENT_HEARTBEAT`, `BACKUPX_AGENT_POLL`, `BACKUPX_AGENT_TEMP_DIR`, `BACKUPX_AGENT_PROXY_URL`, `BACKUPX_AGENT_CA_CERT_FILE`, and `BACKUPX_AGENT_INSECURE_TLS`. When no explicit proxy URL is set, the Agent also honors `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`.

`--token` overrides `--token-file`. Keep long-lived tokens in a root-readable file rather than command history. A private CA and `--insecure-tls` cannot be enabled together.

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

The `-p` file must define `STORAGE_TYPE` and either `STORAGE_CONFIG_JSON` or `STORAGE_CONFIG`. Optional keys include `PARALLEL_FACTOR`, `COMPRESS`, `LOG_FILE`, `CATALOG_DB`, and `KEY_PREFIX`.

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

Run this command on the Master host with access to the configured SQLite path. Avoid placing the new password directly in retained shell history.
