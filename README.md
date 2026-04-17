<p align="right">
  <strong>English</strong> | <a href="README.zh-CN.md">中文</a>
</p>
<p align="center">
  <h1 align="center">BackupX</h1>
  <p align="center">
    <strong>Self-hosted server backup management</strong><br>
    One binary, one command — manage every backup of every server.
  </p>
  <p align="center">
    <a href="https://github.com/Awuqing/BackupX/stargazers"><img src="https://img.shields.io/github/stars/Awuqing/BackupX?style=flat-square&color=f5c542" alt="Stars"></a>
    <a href="https://github.com/Awuqing/BackupX/releases"><img src="https://img.shields.io/github/v/release/Awuqing/BackupX?style=flat-square&color=brightgreen" alt="Release"></a>
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go">
    <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
    <img src="https://img.shields.io/badge/SQLite-embedded-003B57?style=flat-square&logo=sqlite" alt="SQLite">
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Awuqing/BackupX?style=flat-square" alt="License"></a>
  </p>
  <p align="center">
    <a href="https://awuqing.github.io/BackupX/"><strong>Docs</strong></a> ·
    <a href="https://github.com/Awuqing/BackupX/releases"><strong>Downloads</strong></a> ·
    <a href="https://hub.docker.com/r/awuqing/backupx"><strong>Docker Hub</strong></a>
  </p>
</p>

---

<table>
<tr>
<td width="50%"><img src="screenshots/dashboard.png" alt="Dashboard"></td>
<td width="50%"><img src="screenshots/backup-tasks.png" alt="Backup Tasks"></td>
</tr>
<tr>
<td><img src="screenshots/storage-targets.png" alt="Storage Targets"></td>
<td><img src="screenshots/backup-records.png" alt="Backup Records"></td>
</tr>
</table>

## Highlights

| Capability | Details |
|-----------|---------|
| **Backup Types** | Files/directories (multi-source), MySQL, PostgreSQL, SQLite, SAP HANA (full / incremental / differential / log + parallel channels + retry) |
| **SAP HANA Backint Agent** | Built-in Backint protocol — HANA's native interface routes data directly to any BackupX storage backend |
| **70+ Storage Backends** | Alibaba OSS, Tencent COS, Qiniu, S3, Google Drive, WebDAV, FTP + SFTP, Azure Blob, Dropbox, OneDrive and dozens more via rclone |
| **Scheduling** | Cron + visual editor + auto-retention (by days/count + empty-directory cleanup) |
| **Multi-Node Cluster** | Master-Agent mode via HTTP long-polling — Agents run tasks locally, upload straight to storage, no reverse connectivity required |
| **Security** | JWT + bcrypt + AES-256-GCM encrypted config + optional backup encryption + full audit log |
| **Notifications** | Email / Webhook / Telegram on success or failure |
| **Deployment** | Single binary + embedded SQLite, Docker one-click, zero external dependencies |

## Quick Start

```bash
# Docker (recommended)
docker run -d --name backupx -p 8340:8340 -v backupx-data:/app/data awuqing/backupx:latest

# Or prebuilt archive
curl -LO https://github.com/Awuqing/BackupX/releases/latest/download/backupx-linux-amd64.tar.gz
tar xzf backupx-*.tar.gz && cd backupx-* && sudo ./install.sh
```

Open `http://your-server:8340`, create the admin account, then follow the [5-minute Quick Start](https://awuqing.github.io/BackupX/docs/getting-started/quick-start).

## Documentation

The full docs live at **https://awuqing.github.io/BackupX/** — Getting Started, Deployment, SAP HANA, Multi-Node Cluster, API reference, and more. Switch to Chinese via the language dropdown in the top-right nav.

Quick links:

- [Quick Start](https://awuqing.github.io/BackupX/docs/getting-started/quick-start) — first backup in five minutes
- [Installation](https://awuqing.github.io/BackupX/docs/getting-started/installation) — Docker / bare metal / source
- [Multi-Node Cluster](https://awuqing.github.io/BackupX/docs/features/multi-node) — deploy the Agent on remote servers
- [SAP HANA Support](https://awuqing.github.io/BackupX/docs/features/sap-hana) — hdbsql Runner and native Backint
- [API Reference](https://awuqing.github.io/BackupX/docs/reference/api) — REST endpoints

## Development

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make dev-server          # Terminal 1 — backend (:8340)
make dev-web             # Terminal 2 — frontend (Vite HMR)
make test                # run all tests
make build               # produce server/bin/backupx + web/dist
```

See the [development guide](https://awuqing.github.io/BackupX/docs/development/setup) for more.

## Contributing

Issues and pull requests welcome. Please read the [contributing guide](https://awuqing.github.io/BackupX/docs/development/contributing) before opening a PR — commit messages and PRs on this project are written in Chinese.

## License

[Apache License 2.0](LICENSE)
