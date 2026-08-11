---
sidebar_position: 1
title: Upgrade and Recovery
description: Back up the control plane, upgrade safely, roll back as a unit, and recover a failed Master.
---

# Upgrade and Recovery

Backup artifacts and the BackupX control plane are different recovery domains. Object storage may still contain every archive while a lost Master database removes users, encrypted storage credentials, schedules, records, node tokens, and audit history. Protect both.

## Non-negotiable rules

1. Run exactly one active Master against a data directory or SQLite database.
2. Snapshot the complete data directory and configuration while the Master is stopped, or use a storage-level atomic snapshot.
3. Keep the old application version and its pre-upgrade data snapshot together. Schema migration happens at startup, so switching only the binary or image back is not a safe rollback.
4. Store control-plane snapshots outside the Master host and test restoring them.
5. Let active backup and restore jobs finish before stopping the Master.

| Deployment | Persistent control-plane data | Configuration and release state |
| --- | --- | --- |
| Docker | `/app/data` in the `backupx-data` volume | Compose file, protected `.env`, pinned image tag or digest |
| Bare metal | `/opt/backupx/data` | `/etc/backupx`, `/opt/backupx/bin`, `/opt/backupx/web`, systemd unit |

The SQLite database contains generated JWT and encryption keys when they are not supplied in configuration. Treat every control-plane snapshot as a secret.

## Change checklist

Before an upgrade, host migration, or security-key change:

- Record the current BackupX version and the exact image digest or release checksum.
- Confirm `/ready` returns HTTP 200 and review recent failures.
- Wait for running backup, restore, verification, and replication work to finish.
- Test at least one storage target and confirm Agents are online.
- Create a full control-plane snapshot and copy it off-host.
- Optionally export task definitions for human review. Task export excludes database passwords and storage credentials, so it is not a replacement for the database snapshot.
- Define the rollback decision and maintenance-window deadline before starting.

## Snapshot a Docker deployment

This example creates a consistent file-level copy without requiring access to Docker's volume directory:

~~~bash
snapshot="backupx-control-plane-$(date -u +%Y%m%dT%H%M%SZ)"
install -d -m 0700 "$snapshot"

docker compose stop backupx
docker cp backupx:/app/data "$snapshot/data"
cp docker-compose.yml "$snapshot/"
if [ -f .env ]; then cp .env "$snapshot/"; fi
docker compose start backupx

tar -czf "$snapshot.tar.gz" "$snapshot"
sha256sum "$snapshot.tar.gz" > "$snapshot.tar.gz.sha256"
curl -fsS http://127.0.0.1:8340/ready
~~~

If copying fails, start the stopped service before investigating. Protect the archive because `.env` and the database can contain credentials. A block-volume or storage-provider snapshot is also valid when it is atomic across the whole volume.

## Snapshot a bare-metal deployment

~~~bash
snapshot="/var/backups/backupx/backupx-control-plane-$(date -u +%Y%m%dT%H%M%SZ).tar.gz"
sudo install -d -m 0700 /var/backups/backupx

sudo systemctl stop backupx
sudo tar --acls --xattrs -C / -czf "$snapshot" \
  etc/backupx \
  etc/systemd/system/backupx.service \
  opt/backupx/bin \
  opt/backupx/web \
  opt/backupx/data
sudo systemctl start backupx

sudo sha256sum "$snapshot" | sudo tee "$snapshot.sha256"
curl -fsS http://127.0.0.1:8340/ready
~~~

Copy the archive and checksum to protected off-host storage. Do not copy only `backupx.db` while the service is running.

## Upgrade Docker

1. Put a release tag or immutable digest in `BACKUPX_IMAGE`. Do not use `latest` for a controlled production upgrade.
2. Create and verify the pre-upgrade snapshot.
3. Pull and recreate the service:

~~~bash
docker compose pull backupx
docker compose up -d backupx
docker compose ps
docker compose logs --tail=100 backupx
curl -fsS http://127.0.0.1:8340/ready
~~~

4. Sign in, test a storage target, confirm Agent heartbeats, and run one small backup plus a restore or verification drill.
5. Keep the old image reference and snapshot until the observation window ends.

Upgrade Agents after the Master, in small batches. Keep the node-specific proxy, private-CA, token-file, and bastion configuration unchanged unless that configuration is the purpose of the change.

## Upgrade bare metal

Download the target release and checksum, verify them, then extract the archive. The installer preserves an existing `/etc/backupx/config.yaml`, replaces the binary, web assets, and systemd unit, and restarts the service.

~~~bash
sha256sum -c backupx-vX.Y.Z-linux-amd64.tar.gz.sha256
tar xzf backupx-vX.Y.Z-linux-amd64.tar.gz
cd backupx-vX.Y.Z-linux-amd64
sudo ./install.sh

sudo systemctl status backupx --no-pager
curl -fsS http://127.0.0.1:8340/ready
~~~

Create the stopped-service snapshot before running the installer. Use the same post-upgrade application checks as Docker.

## Roll back

Rollback is a paired operation: restore both the previous application release and the snapshot created immediately before the upgrade.

For Docker, preserve the failed volume for analysis and restore the snapshot into a new empty volume. Point Compose at that volume and the previous image tag, then start exactly one Master. For bare metal, stop the service, preserve the failed state, restore the old configuration, binary, web assets, data directory, and unit from the same archive, reload systemd, and start the service.

After rollback:

~~~bash
curl -fsS http://127.0.0.1:8340/health
curl -fsS http://127.0.0.1:8340/ready
~~~

Then verify login, storage access, schedules, Agent heartbeats, a backup, and a non-destructive restore drill. Do not delete the failed state until the incident is understood.

## Recover a lost Master

1. Provision a replacement host with the same architecture and the exact application version recorded with the snapshot.
2. Keep the replacement isolated from production traffic and ensure the old Master cannot start.
3. Restore configuration and the complete data directory with their original permissions.
4. Start one Master and check `/ready` locally.
5. Move the stable DNS name or virtual IP only after local validation.
6. Confirm users, storage targets, tasks, records, notifications, and audit history.
7. Existing Agents reconnect automatically when the restored database contains their matching tokens. Investigate and rotate tokens that may have been exposed.
8. Run a small backup and a restore or verification drill before ending the incident.

External backup artifacts are not recreated by restoring the control plane; they remain on their configured storage targets. Conversely, task JSON export is useful for rebuilding schedules but omits secrets, storage definitions, and some node bindings. Use it only as an additional recovery aid.

## Test the recovery plan

At least quarterly, restore a recent snapshot into an isolated network, start the recorded BackupX version, and verify:

- `/ready` becomes healthy without contacting the production Master.
- An administrator can sign in and encrypted storage configurations can be read.
- Task, node, record, and audit counts are plausible.
- A storage target can be tested without writing production data.
- A selected backup can be verified or restored to an isolated destination.

Record restore duration and the newest recoverable snapshot time. Those measured values are the real control-plane RTO and RPO.
