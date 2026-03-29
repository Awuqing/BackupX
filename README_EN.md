<p align="right">
  <strong>English</strong> | <a href="README.md">中文</a>
</p>
<p align="center">
  <h1 align="center">🛡️ BackupX</h1>
  <p align="center">
    <strong>Self-hosted Server Backup Management Platform with Web UI</strong>
  </p>
  <p align="center">
    <a href="#features">Features</a> •
    <a href="#quick-start">Quick Start</a> •
    <a href="#configuration">Configuration</a> •
    <a href="#architecture">Architecture</a> •
    <a href="#cluster-mode">Cluster</a> •
    <a href="#development">Development</a> •
    <a href="#api-reference">API</a>
  </p>
  <p align="center">
    <a href="https://github.com/Awuqing/BackupX/stargazers"><img src="https://img.shields.io/github/stars/Awuqing/BackupX?style=flat-square&color=f5c542" alt="Stars"></a>
    <a href="https://github.com/Awuqing/BackupX/releases"><img src="https://img.shields.io/github/v/release/Awuqing/BackupX?style=flat-square&color=brightgreen" alt="Release"></a>
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go" alt="Go">
    <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
    <img src="https://img.shields.io/badge/TypeScript-5-3178C6?style=flat-square&logo=typescript" alt="TypeScript">
    <img src="https://img.shields.io/badge/SQLite-embedded-003B57?style=flat-square&logo=sqlite" alt="SQLite">
    <a href="LICENSE"><img src="https://img.shields.io/github/license/Awuqing/BackupX?style=flat-square" alt="License"></a>
    <a href="https://github.com/Awuqing/BackupX/issues"><img src="https://img.shields.io/github/issues/Awuqing/BackupX?style=flat-square" alt="Issues"></a>
  </p>
</p>

---

BackupX is a self-hosted backup management platform for **Linux / macOS servers**. Through an enterprise-grade Web console, you can easily configure directory backups, database backups, and securely store backup files to Alibaba Cloud OSS, Tencent Cloud COS, Qiniu Cloud Kodo, Google Drive, S3-compatible storage, WebDAV, FTP/FTPS, or local disk.

Supports **multi-node cluster management** for unified control of backup tasks across different servers.

> **For**: Individual developers / small teams / DevOps with Linux servers

## Screenshots

### Login
![Login](screenshots/login.png)

### Dashboard
![Dashboard](screenshots/dashboard.png)

### Backup Tasks
![Backup Tasks](screenshots/backup-tasks.png)

### Backup Records
![Backup Records](screenshots/backup-records.png)

### Storage Targets
![Storage Targets](screenshots/storage-targets.png)

### Node Management
![Node Management](screenshots/nodes.png)

### Notification Settings
![Notification Settings](screenshots/notifications.png)

### System Settings
![System Settings](screenshots/settings.png)

## Features

### 📦 Multiple Backup Types
- **Files / Directories** — Custom exclude rules (e.g. `node_modules`, `*.log`)
- **MySQL** — Via native `mysqldump` tool
- **SQLite** — Safe file copy
- **PostgreSQL** — Via native `pg_dump` tool
- **SAP HANA** — Via native `hdbsql` tool (multi-tenant database support)

### ☁️ Multi-Cloud Storage Backends
| Provider | Type | Description |
|----------|------|-------------|
| 🇨🇳 **Alibaba Cloud OSS** | `aliyun_oss` | Auto endpoint assembly, internal network support |
| 🇨🇳 **Tencent Cloud COS** | `tencent_cos` | Auto endpoint assembly |
| 🇨🇳 **Qiniu Cloud Kodo** | `qiniu_kodo` | 6 region precise mapping |
| 🌍 **S3 Compatible** | `s3` | AWS S3 / MinIO / Cloudflare R2, etc. |
| 🌍 **Google Drive** | `google_drive` | Full OAuth 2.0 flow |
| 🌍 **WebDAV** | `webdav` | Nextcloud / Nutstore, etc. |
| 🌍 **FTP / FTPS** | `ftp` | Standard FTP protocol with Explicit TLS support |
| 💾 **Local Disk** | `local_disk` | Backup to local server directory |

> Chinese cloud providers only require **Region** and **AccessKey** — the system auto-assembles the endpoint. Powered by the S3 engine under the hood with zero extra dependencies.

### 🖥️ Cluster Management (Master-Agent)
- **Node Management** — Register remote server nodes with Token authentication
- **Local Node** — Auto-created, zero-friction upgrade for single-machine users
- **Directory Browser** — Visual file tree selector for backup source paths
- **Agent Heartbeat** — Real-time node online status monitoring
- **Task Tags** — Categorize and manage backup tasks by tags/nodes

### ⏰ Automation & Scheduling
- Cron expression scheduling
- Visual Cron editor
- Auto-retention policy (by days / by count)
- Max concurrent backup limit

### 🔐 Security
- JWT authentication + bcrypt password hashing
- AES-256-GCM encrypted sensitive config storage (DB passwords, OAuth tokens)
- Optional backup file encryption
- Login rate limiting (brute force protection)
- Node Token authentication (one-time display, secure transport)

### 📊 Monitoring & Notifications
- Dashboard stats (success rate, storage usage, backup trend charts)
- Email / Webhook / Telegram notifications
- Real-time backup execution logs (SSE)

### 🌐 Other
- Chinese & English i18n
- Zero external dependencies (embedded SQLite, single binary deployment)
- Docker / Docker Compose one-click deployment
- systemd service support

## Quick Start

### Docker Deployment (Recommended)

```bash
# Clone the project
git clone https://github.com/Awuqing/BackupX.git
cd BackupX

# Start with one command
docker compose up -d
```

To back up host directories, mount them in `docker-compose.yml`:

```yaml
volumes:
  - backupx-data:/app/data
  - /path/to/backup/source:/mnt/source:ro
```

### Build from Source

```bash
# Clone the project
git clone https://github.com/Awuqing/BackupX.git
cd BackupX

# Build frontend and backend
make build

# Start the backend service (default port :8340)
cd server && ./bin/backupx
```

### Access Web UI

Open `http://your-server:8340` in your browser. First-time use will guide you through creating an admin account.

## Configuration

The config file defaults to `./config.yaml`. Settings can also be overridden via `BACKUPX_` prefixed environment variables.

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8340
  mode: "release"              # debug | release

database:
  path: "./data/backupx.db"    # SQLite database path

security:
  jwt_secret: ""               # Leave empty to auto-generate
  jwt_expire: "24h"
  encryption_key: ""           # AES encryption key, auto-generated if empty

backup:
  temp_dir: "/tmp/backupx"     # Backup temp directory
  max_concurrent: 2            # Max concurrent backups

log:
  level: "info"                # debug | info | warn | error
  file: "./data/backupx.log"
  max_size: 100                # Max log file size (MB)
  max_backups: 3               # Number of old log files to retain
  max_age: 30                  # Log retention days
```

> 💡 `jwt_secret` and `encryption_key` are auto-generated on first startup and persisted to the database.

## Architecture

```
                        ┌─────────────────────┐
                        │   Nginx (Reverse     │
                        │   Proxy)             │
                        │  / → Static Files    │
                        │  /api → :8340        │
                        └─────────┬───────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────┐
│              BackupX Master (Go API Server)           │
│                      :8340                            │
│                                                      │
│  ┌──────┐  ┌────────────┐  ┌───────────────────────┐│
│  │ Auth │  │Backup Engine│  │  Storage Registry     ││
│  └──────┘  └──────┬─────┘  │  ┌─────────────────┐  ││
│                   │        │  │ Alibaba Cloud    │  ││
│  ┌──────────┐    │        │  │ Tencent Cloud    │  ││
│  │ Cron     │◄───┘        │  │ Qiniu Cloud      │  ││
│  │Scheduler │             │  │ S3 Compatible    │  ││
│  └──────────┘             │  │ Google Drive     │  ││
│                           │  │ WebDAV           │  ││
│                           │  │ FTP / FTPS       │  ││
│  ┌──────────┐             │  │ Local Disk       │  ││
│  │ Notify   │             │  └─────────────────┘  ││
│  │ Module   │             └───────────────────────┘│
│  └──────────┘                                      │
│                                                      │
│  ┌──────────────┐   ┌────────────────────┐          │
│  │ Node Manager │   │ SQLite (backupx.db)│          │
│  └──────┬───────┘   └────────────────────┘          │
└─────────┼────────────────────────────────────────────┘
          │ Heartbeat / Task Dispatch
          ▼
┌──────────────────┐  ┌──────────────────┐
│   Agent Node A   │  │   Agent Node B   │
│   (Remote Server)│  │   (Remote Server)│
└──────────────────┘  └──────────────────┘
```

### Tech Stack

| Component | Technology |
|-----------|-----------|
| **Backend** | Go · Gin · GORM · SQLite · robfig/cron |
| **Frontend** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **Storage** | AWS SDK v2 (S3/OSS/COS/Kodo) · Google Drive API v3 · gowebdav · jlaffaye/ftp |
| **Security** | JWT · bcrypt · AES-256-GCM |
| **Logging** | zap + lumberjack (auto-rotation) |

## Cluster Mode

BackupX supports **Master-Agent** mode for managing backup tasks across multiple servers.

### How It Works

1. **Master** is the server running the BackupX Web console
2. **Agent** is deployed on remote servers that need to be backed up
3. Agents register with the Master using a Token and send periodic heartbeats
4. Master dispatches backup tasks to the corresponding Agent for execution

### Adding Nodes

```bash
# In Web Console → Node Management → Add Node
# The system generates a unique 64-character hex Token

# Configure the Agent on the remote server
./backupx-agent --master http://master-server:8340 --token <your-token>
```

### Directory Probe API

Master provides `GET /api/nodes/:id/fs/list?path=/` to remotely browse a node's file system. The frontend uses a tree selector to browse the target machine's directory structure when creating backup tasks.

## Project Structure

```
BackupX/
├── server/                        # Go backend
│   ├── cmd/backupx/               #   Entry point
│   ├── internal/
│   │   ├── app/                   #   App assembly (DI)
│   │   ├── apperror/              #   Unified error types
│   │   ├── backup/                #   Backup engine (file/mysql/sqlite/pgsql/saphana)
│   │   │   └── retention/         #     Retention policy
│   │   ├── config/                #   Config loading (viper)
│   │   ├── database/              #   Database init + migrations
│   │   ├── http/                  #   HTTP handlers + routes + middleware
│   │   ├── httpapi/               #   HTTP API helpers
│   │   ├── logger/                #   Logger init (zap + lumberjack)
│   │   ├── model/                 #   GORM data models
│   │   ├── notify/                #   Notifications (email/webhook/telegram)
│   │   ├── repository/            #   Data access layer
│   │   ├── scheduler/             #   Cron scheduler
│   │   ├── security/              #   JWT + rate limiting
│   │   ├── service/               #   Business logic
│   │   └── storage/               #   Storage backends (plugin interface)
│   │       ├── aliyun/            #     Alibaba Cloud OSS
│   │       ├── tencent/           #     Tencent Cloud COS
│   │       ├── qiniu/             #     Qiniu Cloud Kodo
│   │       ├── s3/                #     S3 Compatible core
│   │       ├── s3provider/        #     S3 Provider helper
│   │       ├── googledrive/       #     Google Drive
│   │       ├── webdav/            #     WebDAV core
│   │       ├── webdavprovider/    #     WebDAV Provider helper
│   │       ├── localdisk/         #     Local disk
│   │       ├── ftp/               #     FTP / FTPS
│   │       └── codec/             #     Config codec
│   └── pkg/                       #   Utilities (compress/crypto/response)
├── web/                           # React frontend
│   └── src/
│       ├── components/            #   Shared components (CronEditor/FormDrawer/...)
│       ├── hooks/                 #   Custom Hooks
│       ├── layouts/               #   Layout components (AppLayout)
│       ├── pages/                 #   Page modules
│       │   ├── dashboard/         #     Dashboard
│       │   ├── backup-tasks/      #     Backup tasks
│       │   ├── backup-records/    #     Backup records
│       │   ├── storage-targets/   #     Storage targets
│       │   ├── nodes/             #     Node management
│       │   ├── notifications/     #     Notification settings
│       │   ├── settings/          #     System settings
│       │   └── login/             #     Login page
│       ├── services/              #   API request wrappers
│       ├── stores/                #   Zustand state management
│       ├── styles/                #   Global styles
│       ├── types/                 #   TypeScript type definitions
│       ├── utils/                 #   Utility functions
│       ├── locales/               #   i18n language packs (zh-CN / en-US)
│       └── router/                #   Route configuration
├── deploy/                        # Deployment configs
│   ├── nginx.conf                 #   Nginx reference config
│   ├── backupx.service            #   systemd service unit
│   ├── install.sh                 #   One-click install script
│   └── docker/                    #   Docker deployment configs
│       ├── nginx.conf             #     In-container Nginx config
│       └── entrypoint.sh          #     Container entrypoint script
├── .github/                       # GitHub configuration
│   ├── workflows/ci.yml           #   CI workflow
│   ├── workflows/release.yml      #   Release workflow
│   └── ISSUE_TEMPLATE/            #   Issue templates
├── Dockerfile                     # Docker multi-stage build
├── docker-compose.yml             # Docker Compose config
└── Makefile                       # Build commands
```

## Development

### Prerequisites

- **Go** ≥ 1.21
- **Node.js** ≥ 18
- **npm**

### Dev Mode

```bash
# Terminal 1: Start backend (use air for hot-reload)
make dev-server

# Terminal 2: Start frontend (Vite HMR)
make dev-web
```

### Run Tests

```bash
# Run all tests
make test

# Backend only
make test-server    # go test ./...

# Frontend only
make test-web       # npm run test
```

### Build

```bash
# Build frontend and backend
make build

# Clean build artifacts
make clean
```

## Deployment

### One-Click Install (Recommended)

```bash
# Build first
make build

# Run install script as root
sudo ./deploy/install.sh
```

The install script will automatically:
1. Create a `backupx` system user
2. Install the binary to `/opt/backupx/bin/`
3. Deploy the frontend to `/opt/backupx/web/`
4. Generate config at `/etc/backupx/config.yaml`
5. Register and start the systemd service
6. Configure Nginx reverse proxy (if installed)

### Docker Deployment

```bash
# Using docker compose
docker compose up -d

# Or build and run manually
docker build -t backupx .
docker run -d --name backupx -p 8340:8340 -v backupx-data:/app/data backupx
```

Override configuration via environment variables:

```bash
docker run -d --name backupx \
  -p 8340:8340 \
  -v backupx-data:/app/data \
  -e TZ=Asia/Shanghai \
  -e BACKUPX_LOG_LEVEL=debug \
  -e BACKUPX_BACKUP_MAX_CONCURRENT=4 \
  backupx
```

### Manual Deployment

```bash
# 1. Build
cd server && go build -o backupx ./cmd/backupx
cd ../web && npm run build

# 2. Deploy files
scp server/backupx your-server:/opt/backupx/bin/
scp -r web/dist/ your-server:/opt/backupx/web/
scp server/config.example.yaml your-server:/etc/backupx/config.yaml

# 3. Start
ssh your-server '/opt/backupx/bin/backupx -config /etc/backupx/config.yaml'
```

### Nginx Config Example

```nginx
server {
    listen 80;
    server_name backup.example.com;

    # Frontend static files
    location / {
        root /opt/backupx/web;
        try_files $uri $uri/ /index.html;
    }

    # API reverse proxy
    location /api/ {
        proxy_pass http://127.0.0.1:8340;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## API Reference

All APIs are prefixed with `/api` and use JWT Bearer Token authentication (unless noted otherwise).

| Module | Endpoint | Description |
|--------|----------|-------------|
| **Auth** | `POST /api/auth/setup` | Initialize admin (first time) |
| | `POST /api/auth/login` | Login to get Token |
| | `POST /api/auth/logout` | Logout |
| | `GET /api/auth/profile` | Current user info |
| | `PUT /api/auth/password` | Change password |
| **Backup Tasks** | `GET/POST /api/backup/tasks` | List / Create tasks |
| | `GET/PUT/DELETE /api/backup/tasks/:id` | Detail / Update / Delete |
| | `PUT /api/backup/tasks/:id/toggle` | Enable / Disable |
| | `POST /api/backup/tasks/:id/run` | Trigger manual execution |
| **Backup Records** | `GET /api/backup/records` | List records (with filter) |
| | `GET /api/backup/records/:id` | Record detail |
| | `GET /api/backup/records/:id/logs/stream` | Real-time execution logs (SSE) |
| | `GET /api/backup/records/:id/download` | Download backup file |
| | `POST /api/backup/records/:id/restore` | Restore backup |
| **Storage Targets** | `GET/POST /api/storage-targets` | List / Add targets |
| | `GET/PUT/DELETE /api/storage-targets/:id` | Detail / Update / Delete |
| | `POST /api/storage-targets/test` | Test connection |
| | `POST /api/storage-targets/:id/test` | Test saved connection |
| | `GET /api/storage-targets/:id/usage` | Query usage |
| **Nodes** | `GET/POST /api/nodes` | List / Add nodes |
| | `GET/DELETE /api/nodes/:id` | Detail / Delete |
| | `GET /api/nodes/:id/fs/list` | Directory browser |
| | `POST /api/agent/heartbeat` | Agent heartbeat ⚡ |
| **Notifications** | `GET/POST /api/notifications` | List / Add |
| | `POST /api/notifications/test` | Test notification |
| | `POST /api/notifications/:id/test` | Test saved notification |
| **Dashboard** | `GET /api/dashboard/stats` | Overview statistics |
| | `GET /api/dashboard/timeline` | Backup trend timeline |
| **System** | `GET /api/system/info` | System info (version/disk) |
| | `GET/PUT /api/settings` | System settings |

> ⚡ `POST /api/agent/heartbeat` is a public endpoint authenticated via Node Token instead of JWT.

## Cloud Storage Setup Guide

### Alibaba Cloud OSS

1. Log in to [Alibaba Cloud Console](https://oss.console.aliyun.com/), create a Bucket
2. Go to RAM Console to create an AccessKey
3. Select "Alibaba Cloud OSS" when adding a storage target in BackupX
4. Enter the Region (e.g. `cn-hangzhou`) and AccessKey — the system auto-assembles the endpoint

### Tencent Cloud COS

1. Log in to [Tencent Cloud Console](https://console.cloud.tencent.com/cos), create a bucket
2. Go to API Key Management to create SecretId/SecretKey
3. Bucket name format is `BucketName-APPID` (e.g. `backup-1250000000`)

### Qiniu Cloud Kodo

1. Log in to [Qiniu Cloud Console](https://portal.qiniu.com/), create a storage space
2. Supported regions: `z0` (East China) / `cn-east-2` (East China-Zhejiang 2) / `z1` (North China) / `z2` (South China) / `na0` (North America) / `as0` (Southeast Asia)

### Google Drive

1. Go to [Google Cloud Console](https://console.cloud.google.com/) and create a project
2. Enable the **Google Drive API**
3. Create an **OAuth 2.0 Client ID** (Web application type)
4. Add redirect URI: `http://your-server/api/storage-targets/google-drive/callback`
5. Enter the Client ID / Secret in BackupX storage management and click Authorize

## Contributing

Issues and Pull Requests are welcome!

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the [Apache License 2.0](LICENSE).

---

<p align="center">
  Made with ❤️ for self-hosters
</p>
