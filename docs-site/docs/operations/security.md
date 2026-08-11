---
sidebar_position: 2
title: Security Hardening
description: Production controls for network exposure, roles, secrets, Agents, containers, and public endpoints.
---

# Security Hardening

BackupX coordinates access to source files, database credentials, storage credentials, and restore destinations. Deploy the Master as a security-sensitive control plane, not as a general public web application.

## Recommended exposure model

| Component | Inbound access | Outbound access |
| --- | --- | --- |
| Master | HTTPS from administrators and Agents; metrics only from monitoring networks | Storage providers, notification endpoints, release checks |
| Agent | No inbound port required | Master HTTPS endpoint and assigned storage targets |
| SQLite data | Local or block-backed filesystem only | None |

Bind Docker to `127.0.0.1` when a reverse proxy runs on the same host:

~~~dotenv
BACKUPX_BIND_ADDRESS=127.0.0.1
~~~

For bare metal, set `server.host` to loopback when only a local proxy should reach BackupX. Otherwise restrict TCP 8340 with the host or network firewall.

## TLS and reverse proxies

- Use HTTPS across every untrusted network segment.
- Set `server.external_url` to the stable URL that Agents can reach.
- Add only the exact proxy IP or subnet to `server.trusted_proxies`. Never trust `0.0.0.0/0`.
- Send the final HTTPS URL to Agents; the Agent does not follow redirects.
- For private PKI, install a PEM CA on the Agent and configure `caCertFile` or `--ca-cert`.
- Use `--insecure-tls` only for temporary testing.
- Keep Nginx request and response buffering disabled for relay uploads and SSE logs.

When an SSH bastion is required, bind tunnels to loopback, verify host keys, use a dedicated account and key, and make the Agent service depend on the tunnel. See [Multi-Node Cluster](../features/multi-node).

## Roles and API keys

| Role | Intended access |
| --- | --- |
| `viewer` | Read dashboards, tasks, records, reports, and audit data; cannot browse node filesystems or mutate resources |
| `operator` | Viewer access plus task, storage, notification, backup, restore, verification, and file-browse operations |
| `admin` | Operator access plus users, API keys, settings, node lifecycle, install tokens, and token rotation |

Create separate named users instead of sharing the initial administrator. Enable two-factor authentication or passkeys for privileged accounts. Review trusted devices and recovery codes periodically.

User JWTs are stateless. Logout removes the client copy but does not revoke a token that was already copied elsewhere. Set `security.jwt_expire` to the shortest practical lifetime, protect Bearer tokens, and rotate the JWT secret when all active sessions must be invalidated.

API keys use the same role checks as interactive users. Their plaintext is shown only once; the database stores a keyed hash. Give automation the lowest role it needs, set an expiry, keep the key in a secret manager, and revoke unused keys. Avoid administrator API keys for monitoring.

## Protect control-plane secrets

- Restrict `/etc/backupx/config.yaml` to `root:backupx` mode `0640` and the data directory to the service account.
- If `jwt_secret` and `encryption_key` are empty, generated values are persisted in the SQLite database. Back up the complete data directory.
- Losing or replacing the encryption key makes saved storage credentials unreadable.
- The database includes password hashes, configuration secrets, Agent tokens, API-key hashes, trusted-device state, and audit data. Encrypt snapshots and control their retention.
- Do not put tokens in shell history, issue text, screenshots, or support bundles.

Each node has an independent long-lived Agent token. The systemd installer stores it in `/etc/backupx-agent/agent.token` with mode `0600`. Rotate a token after personnel changes, host compromise, or accidental disclosure, update the token file during the overlap window, then restart the Agent.

One-time install URLs are valid for 5 minutes to 24 hours and are consumed after use. Treat the URL and the embedded fallback command as secrets: the generated installation material provisions the long-lived node token.

## Container and host permissions

The canonical Compose deployment drops all capabilities and adds back only those needed to repair legacy volume ownership and switch to the unprivileged `backupx` user. Keep `no-new-privileges` enabled and do not mount the Docker socket.

Mount backup sources read-only. Add a separate, narrowly scoped writable mount only when a restore destination requires it. Prefer a host Agent over running the Master container as root for privileged filesystem access.

The systemd Master runs as `backupx`. The Agent normally runs as root because it may back up or restore files belonging to arbitrary system users. Limit who can create tasks and protect the root-owned Agent configuration.

## Public endpoints

The following endpoints intentionally do not use BackupX JWT or API-key authentication:

- `/health` and `/api/health`
- `/ready` and `/api/ready`
- `/metrics`
- one-time `/install/:token` and `/api/install/:token` routes

Health responses expose status, version, uptime, timestamp, and readiness checks; a failed readiness check can include database error detail. `/metrics` also includes node and storage-target labels. Restrict metrics and probes to monitoring networks at the firewall or reverse proxy. Do not cache or log full install-token URLs.

## Backup encryption boundary

Encrypted backup tasks run on the Master because remote Agents never receive the Master's encryption key. Do not work around this boundary by copying the Master key to Agents. For Agent-routed tasks, rely on transport encryption and the destination provider's server-side encryption when required.

Test restores for encrypted backups after every key-management change. A backup whose key is unavailable is not recoverable.

## Audit and incident response

BackupX records privileged actions in the audit log and can forward signed audit events to an external webhook. Send high-value audit records to a separately administered SIEM or append-only store so a compromised Master cannot erase the only copy.

After suspected compromise:

1. Isolate the Master without deleting evidence.
2. Revoke exposed API keys and rotate affected Agent tokens and storage credentials.
3. Replace JWT and encryption keys only with a planned migration; changing the encryption key invalidates saved encrypted configuration.
4. Review user, trusted-device, API-key, node, settings, restore, and deletion events.
5. Recover from a known-good control-plane snapshot when integrity cannot be established.

Use [Upgrade and Recovery](./upgrade-recovery) for the paired application-and-database recovery procedure.
