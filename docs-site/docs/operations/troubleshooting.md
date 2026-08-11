---
sidebar_position: 4
title: Troubleshooting
description: A safe diagnostic sequence for the Master, reverse proxy, Agents, backup tools, and SQLite.
---

# Troubleshooting

Start with the first failing boundary and preserve evidence. Avoid deleting the database, recreating volumes, rotating every token, or reinstalling until the failure is understood.

## Fast triage

| Symptom | First check | Likely boundary |
| --- | --- | --- |
| Web console unavailable | Local `/health`, then proxy `/health` | Process, listener, firewall, proxy, or static assets |
| `/health` works but `/ready` is 503 | Service logs, database path, disk space, ownership | SQLite or data filesystem |
| Login loops or client IP is wrong | Forwarded headers and `trusted_proxies` | Reverse-proxy trust |
| Live logs stop updating | Nginx response buffering and timeout | SSE proxy path |
| Relay upload stalls or proxy disk fills | Request buffering and body-size limit | Reverse proxy |
| Agent offline | Agent service logs, final Master URL, proxy, DNS, CA | Agent-to-Master path |
| Backup starts but fails | Record log, source path, native database tool | Task runner or permissions |
| Restore fails | Record log, destination mount and write access | Storage read or destination permissions |

## Collect status without secrets

Docker Master:

~~~bash
docker compose ps
docker compose logs --tail=200 backupx
curl -i http://127.0.0.1:8340/health
curl -i http://127.0.0.1:8340/ready
~~~

Bare-metal Master:

~~~bash
sudo systemctl status backupx --no-pager
sudo journalctl -u backupx -n 200 --no-pager
sudo ss -lntp | grep 8340
curl -i http://127.0.0.1:8340/health
curl -i http://127.0.0.1:8340/ready
~~~

Systemd Agent:

~~~bash
sudo systemctl status backupx-agent --no-pager
sudo journalctl -u backupx-agent -n 200 --no-pager
sudo systemctl status backupx-agent-tunnel --no-pager
~~~

The tunnel command is relevant only to bastion deployments. Before sharing output, remove Authorization headers, API keys, Agent tokens, install URLs, database passwords, storage credentials, proxy credentials, and private paths that reveal sensitive topology.

## Web console or first setup

Check the unauthenticated setup endpoint:

~~~bash
curl -fsS http://127.0.0.1:8340/api/auth/setup/status
~~~

If the API works but the browser receives a blank page or JSON:

- Confirm the release contains web assets.
- Bare metal: verify `/opt/backupx/web` is readable and `server.web_root` is correct when explicitly set.
- Docker: confirm the official image is running and no custom mount hides the packaged web directory.
- Nginx static mode: confirm `root /opt/backupx/web` and SPA fallback are present.
- Clear an old service-worker or browser cache after a release change.

For authentication failures, verify system time before diagnosing TOTP or passkeys. Confirm the browser origin matches the final HTTPS host, and inspect the audit log for throttling, disabled users, or revoked trusted devices.

## Reverse proxy

Validate and reload Nginx:

~~~bash
sudo nginx -t
sudo systemctl reload nginx
curl -i https://backup.example.com/health
curl -i https://backup.example.com/ready
~~~

Common corrections:

- HTTP 413: set `client_max_body_size 0` for the API route.
- Relay uploads fill proxy temporary storage: set `proxy_request_buffering off`.
- SSE logs arrive in bursts or disconnect: set `proxy_buffering off`, disable proxy cache, and increase read timeout.
- One-click installer returns HTML: proxy `/api/` and retain the legacy `/install/` route.
- Agent receives a redirect: configure the final HTTPS Master URL instead of an HTTP URL.
- Audit shows the proxy address for every user: add only the real proxy IP or subnet to `server.trusted_proxies`.

Use the complete [Nginx configuration](../deployment/nginx) as the comparison baseline.

## Agent offline

An Agent normally heartbeats every 15 seconds and is marked offline after 45 seconds.

1. Confirm the Agent and optional tunnel services are active.
2. Verify the configured Master URL has no trailing redirect and resolves from the Agent host.
3. Check the explicit `proxyUrl`. Use `socks5h://` when DNS must resolve through an SSH dynamic tunnel.
4. Confirm the private CA path exists and is readable. Do not switch permanently to insecure TLS.
5. Check outbound firewall access to the Master and assigned storage backends.
6. Verify `/etc/backupx-agent/agent.token` exists with mode `0600`.
7. If a token was rotated, install the new value during the overlap window and restart the Agent.

Do not paste the token into a diagnostic command that will be saved in shell history. A 401 in Agent logs usually indicates a missing, expired-overlap, or mismatched node token; repeated connection errors indicate URL, DNS, proxy, tunnel, firewall, or CA problems.

## Backup task failures

Open the backup record and inspect its complete log before changing the task.

- File tasks resolve paths on the selected Master or Agent. Confirm the path exists in that host's namespace.
- Docker sees only mounted paths. Backup mounts should normally be read-only.
- MySQL requires `mysqldump` on the execution host's `PATH`.
- PostgreSQL requires `pg_dump` on the execution host's `PATH`.
- SAP HANA runner mode requires its configured client tools and environment.
- Confirm the service account can read sources and write the temporary directory.
- Test the selected storage target from the console.
- Check DNS, egress policy, provider quota, clock skew, and proxy settings for remote storage.

If multiple targets are configured, inspect the per-target result instead of assuming every copy failed. Preserve successful remote artifacts while correcting the failing target.

## Restore, download, or verification failures

- Confirm the remote artifact still exists and the storage credentials can read it.
- Check that the destination is mounted on the host that performs the restore.
- Use a separate writable restore path; do not make every backup-source mount writable.
- Check free space in the destination and Agent temporary directory.
- For encrypted backups, confirm the original Master encryption key is available.
- For CDC repositories, keep manifests, indexes, and shared packs together; a manifest alone is not a complete backup.

Prefer an isolated restore destination during diagnosis. Do not repeatedly restore over the production source.

## SQLite and readiness failures

When `/health` is 200 but `/ready` is 503:

1. Read the exact database error from service logs.
2. Check free disk space, inode availability, path ownership, and mount state.
3. Confirm only one Master process or container uses the data directory.
4. Keep SQLite on a local or block-backed filesystem, not a shared multi-writer or unreliable network filesystem.
5. Check whether an external backup or antivirus process is holding files for long periods.

BackupX uses a five-second SQLite busy timeout, but that does not make SQLite a clustered database. Do not fix lock errors by starting another Master. For a file-level copy, stop the service and copy the whole data directory.

## Escalation package

When opening an issue, include:

- BackupX version, installation method, operating system, and architecture.
- Whether the failure affects the Master, Agent, proxy, storage target, or one task.
- Redacted service logs covering the first failure.
- HTTP status and response body from `/health` and `/ready`.
- A minimal reproduction and whether it began after an upgrade or configuration change.
- Relevant proxy configuration with hostnames, credentials, and private addresses redacted.

Never attach `backupx.db`, `.env`, full configuration files, Agent token files, API keys, install commands, or storage credentials to a public issue.

If integrity or rollback is involved, stop making destructive changes and follow [Upgrade and Recovery](./upgrade-recovery).
