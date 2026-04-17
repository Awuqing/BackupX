---
sidebar_position: 2
title: Quick Start
description: Set up BackupX, add a storage target, create your first backup task.
---

# Quick Start

After [installation](./installation), get a first backup running in five minutes.

## 1. Open the console

Browse to `http://your-server:8340`. The first time, you'll be guided through creating an admin account.

## 2. Add a storage target

Navigate to **Storage Targets → Add**. Pick a type and fill the required fields:

| Type | Fields |
|------|--------|
| Alibaba OSS | Region + AccessKey ID/Secret + Bucket |
| Tencent COS | Region + SecretId/SecretKey + Bucket (format `name-appid`) |
| Qiniu Kodo | Region + AccessKey/SecretKey + Bucket |
| S3-compatible | Endpoint + AccessKey + Bucket |
| Google Drive | Client ID/Secret → click "Authorize" for OAuth flow |
| WebDAV | URL + username/password |
| FTP | Host + port + username/password |
| Local disk | Target directory |
| SFTP / Azure / Dropbox / OneDrive | Type-specific required fields; advanced options collapsed |

:::tip
For mainland China cloud vendors you only fill Region and AccessKey — BackupX assembles the endpoint automatically. Rclone-style providers separate required fields from advanced ones, with advanced collapsed by default.
:::

Click **Test Connection** to verify.

## 3. Create a backup task

Go to **Backup Tasks → New**. Three steps:

1. **Basic info** — name, type, cron expression (leave empty for manual-only)
2. **Source** — paths for file backup (multi-source supported), or connection info for databases
3. **Storage & policy** — pick target(s), compression, retention days, encryption on/off

Save, then click **Run Now** to trigger a test. Live logs stream on the **Backup Records** page.

:::note
Deleting a task also removes remote backup files to prevent orphans, but records are kept for audit.
:::

## 4. Configure notifications (optional)

**Notifications** page supports email, webhook, and Telegram. Configure per-channel rules for success/failure events.

## Next up

- Explore [backup types](/docs/features/backup-types) and [storage backends](/docs/features/storage-backends)
- Running SAP HANA? See [SAP HANA Support](/docs/features/sap-hana)
- Managing many servers? See [Multi-Node Cluster](/docs/features/multi-node)
