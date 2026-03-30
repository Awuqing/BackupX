<p align="right">
  <strong>English</strong> | <a href="README.md">中文</a>
</p>
<p align="center">
  <h1 align="center">BackupX</h1>
  <p align="center">
    <strong>Self-hosted Server Backup Management Platform</strong><br>
    One binary, one command — manage all your server backups.
  </p>
  <p align="center">
    <a href="https://github.com/Awuqing/BackupX/stargazers"><img src="https://img.shields.io/github/stars/Awuqing/BackupX?style=flat-square&color=f5c542" alt="Stars"></a>
    <a href="https://github.com/Awuqing/BackupX/releases"><img src="https://img.shields.io/github/v/release/Awuqing/BackupX?style=flat-square&color=brightgreen" alt="Release"></a>
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go">
    <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
    <img src="https://img.shields.io/badge/SQLite-embedded-003B57?style=flat-square&logo=sqlite" alt="SQLite">
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Awuqing/BackupX?style=flat-square" alt="License"></a>
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
| **Backup Types** | Files/Directories (multi-source), MySQL, PostgreSQL, SQLite, SAP HANA |
| **Storage Backends** | Alibaba Cloud OSS, Tencent COS, Qiniu Kodo, S3-compatible (AWS/MinIO/R2), Google Drive, WebDAV, FTP/FTPS, Local Disk |
| **Scheduling** | Cron-based scheduling + visual editor + auto-retention policy (by days/count) |
| **Multi-Node** | Master-Agent cluster for managing backups across multiple servers |
| **Security** | JWT + bcrypt + AES-256-GCM encrypted config + optional backup encryption + audit logs |
| **Notifications** | Email / Webhook / Telegram — push on success or failure |
| **Deployment** | Single binary + embedded SQLite, Docker one-click, zero external dependencies |

---

## Quick Start

### 1. Install

**Docker (recommended, no clone needed):**

```bash
# Create a docker-compose.yml then start
docker compose up -d

# Or run directly
docker run -d --name backupx -p 8340:8340 -v backupx-data:/app/data awuqing/backupx:latest
```

> Docker Hub: [`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx) — supports linux/amd64 and linux/arm64.

<details>
<summary>docker-compose.yml reference</summary>

```yaml
services:
  backupx:
    image: awuqing/backupx:latest
    container_name: backupx
    restart: unless-stopped
    ports:
      - "8340:8340"
    volumes:
      - backupx-data:/app/data
      # Mount host directories to back up (add as needed):
      # - /var/www:/mnt/www:ro
      # - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai

volumes:
  backupx-data:
```

</details>

**Pre-built binaries (bare metal):**

Download from [Releases](https://github.com/Awuqing/BackupX/releases):

```bash
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh        # Auto-configures systemd + Nginx
```

**Build from source:**

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build               # Build frontend + backend
make docker-cn           # Or Docker build with China mirrors (goproxy.cn / npmmirror / Aliyun apk)
```

### 2. Open the Console

Visit `http://your-server:8340` in your browser. First-time access guides you through admin account creation.

### 3. Add a Storage Target

Go to **Storage Targets** → **Add**, choose a storage type and enter credentials:

| Storage Type | Required Fields |
|-------------|----------------|
| Alibaba Cloud OSS | Region + AccessKey ID/Secret + Bucket |
| Tencent Cloud COS | Region + SecretId/SecretKey + Bucket (`name-appid`) |
| Qiniu Cloud Kodo | Region + AccessKey/SecretKey + Bucket |
| S3 Compatible | Endpoint + AccessKey + Bucket |
| Google Drive | Client ID/Secret → click Authorize for OAuth |
| WebDAV | Server URL + Username/Password |
| FTP | Host + Port + Username/Password |
| Local Disk | Target directory path |

Click **Test Connection** to verify.

### 4. Create a Backup Task

Go to **Backup Tasks** → **Create**, complete 3 steps:

1. **Basic Info** — Task name, backup type, Cron expression (leave empty for manual-only)
2. **Source Config** — File backup: select source paths (supports multiple); Database: enter connection info
3. **Storage & Policy** — Select storage target(s), compression, retention days, encryption toggle

Save, then click **Run Now** to test. View real-time logs in **Backup Records**.

### 5. Set Up Notifications (Optional)

Go to **Notifications** to configure Email, Webhook, or Telegram alerts for backup success/failure.

---

## Deployment Guide

### Docker

```bash
docker compose up -d     # Using the docker-compose.yml above
```

Mount host directories for file backup (add to `volumes` in docker-compose.yml):

```yaml
volumes:
  - backupx-data:/app/data
  - /var/www:/mnt/www:ro
  - /etc/nginx:/mnt/nginx-conf:ro
```

Override config via environment variables:

```yaml
environment:
  - TZ=Asia/Shanghai
  - BACKUPX_LOG_LEVEL=debug
  - BACKUPX_BACKUP_MAX_CONCURRENT=4
```

### Bare Metal

```bash
# From pre-built package
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh

# Or from source
make build
sudo ./deploy/install.sh
```

The install script creates a system user, installs to `/opt/backupx/`, configures systemd, and sets up Nginx reverse proxy.

### Nginx Reverse Proxy (bare metal)

```nginx
server {
    listen 80;
    server_name backup.example.com;

    location / {
        root /opt/backupx/web;
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8340;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### Configuration

Config file: `./config.yaml` (or override with `BACKUPX_` prefixed env vars):

```yaml
server:
  port: 8340
database:
  path: "./data/backupx.db"
security:
  jwt_secret: ""          # Auto-generated and persisted to DB
  encryption_key: ""      # Auto-generated
backup:
  temp_dir: "/tmp/backupx"
  max_concurrent: 2
log:
  level: "info"           # debug | info | warn | error
  file: "./data/backupx.log"
```

### Password Reset

```bash
# Bare metal
./backupx reset-password --username admin --password newpass123

# Docker
docker exec -it backupx /app/bin/backupx reset-password --username admin --password newpass123
```

---

## Multi-Node Cluster

BackupX supports Master-Agent mode for managing multiple servers:

1. Web Console → **Node Management** → **Add Node** — system generates a Token
2. Deploy Agent on remote server, connect using the Token
3. Create backup tasks and assign to specific nodes — Master dispatches automatically

The visual directory browser lets you pick directories on remote Agent nodes — no manual path typing.

---

## Development

**Requirements:** Go >= 1.25 · Node.js >= 20 · npm

```bash
# Dev mode
make dev-server          # Terminal 1: backend (:8340)
make dev-web             # Terminal 2: frontend (Vite HMR)

# Test
make test                # Run all tests

# Build
make build               # Build frontend + backend
make docker              # Docker build
make docker-cn           # Docker build with China mirrors
```

### Release

```bash
git tag v1.2.3 && git push --tags
# GitHub Actions: compile dual-arch binaries → publish GitHub Release → push Docker Hub image
```

Or manually trigger the Release workflow from GitHub Actions page.

---

## API Reference

All endpoints prefixed with `/api`, authenticated via JWT Bearer Token.

| Module | Endpoint | Description |
|--------|----------|-------------|
| **Auth** | `POST /auth/setup` | Initialize admin |
| | `POST /auth/login` | Login |
| | `PUT /auth/password` | Change password |
| **Backup Tasks** | `GET\|POST /backup/tasks` | List / Create |
| | `GET\|PUT\|DELETE /backup/tasks/:id` | Detail / Update / Delete |
| | `PUT /backup/tasks/:id/toggle` | Enable / Disable |
| | `POST /backup/tasks/:id/run` | Manual run |
| **Backup Records** | `GET /backup/records` | List (with filter) |
| | `GET /backup/records/:id/logs/stream` | Real-time logs (SSE) |
| | `GET /backup/records/:id/download` | Download |
| | `POST /backup/records/:id/restore` | Restore |
| **Storage Targets** | `GET\|POST /storage-targets` | List / Add |
| | `POST /storage-targets/test` | Test connection |
| **Nodes** | `GET\|POST /nodes` | List / Add |
| | `GET /nodes/:id/fs/list` | Directory browser |
| **Notifications** | `GET\|POST /notifications` | List / Add |
| **Dashboard** | `GET /dashboard/stats` | Overview stats |
| **Audit Logs** | `GET /audit-logs` | Operation audit |
| **System** | `GET /system/info` | System info |

---

## Tech Stack

| Component | Technology |
|-----------|-----------|
| **Backend** | Go · Gin · GORM · SQLite · robfig/cron |
| **Frontend** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **Storage** | AWS SDK v2 · Google Drive API v3 · gowebdav · jlaffaye/ftp |
| **Security** | JWT · bcrypt · AES-256-GCM |

## Contributing

Issues and Pull Requests are welcome!

## License

[Apache License 2.0](LICENSE)
