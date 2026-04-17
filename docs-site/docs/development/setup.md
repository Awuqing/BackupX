---
sidebar_position: 1
title: Development Setup
description: Get a BackupX dev environment running — backend, frontend, tests.
---

# Development Setup

**Requirements:** Go ≥ 1.25, Node.js ≥ 20, npm.

## Clone & install

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
cd web && npm install && cd ..
```

## Dev servers

Run the backend and the Vite dev server in two terminals:

```bash
# Terminal 1: backend on :8340
make dev-server

# Terminal 2: Vite with HMR on :5173
make dev-web
```

The Vite config proxies `/api` to `http://127.0.0.1:8340` so you can open the UI at `http://localhost:5173`.

## Tests

```bash
make test           # runs Go + Web test suites
make test-server    # Go only
make test-web       # Vitest only
```

## Production build

```bash
make build          # server/bin/backupx + web/dist
make docker         # Docker image
make docker-cn      # Docker image with mainland China mirrors
```

## Tech stack

| Component | Stack |
|-----------|-------|
| **Backend** | Go · Gin · GORM · SQLite · robfig/cron · rclone |
| **Frontend** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **Storage** | rclone (70+ backends) · AWS SDK v2 · Google Drive API v3 |
| **Security** | JWT · bcrypt · AES-256-GCM |

## Project layout

```
BackupX/
├── server/             # Go backend
│   ├── cmd/backupx/    # Entry point + subcommands (agent, backint, reset-password)
│   ├── internal/
│   │   ├── agent/      # Agent CLI logic
│   │   ├── app/        # Wiring (repositories → services → handlers)
│   │   ├── backup/     # Backup runners (file / mysql / postgres / sqlite / saphana)
│   │   ├── backint/    # SAP HANA Backint protocol
│   │   ├── http/       # HTTP handlers + router
│   │   ├── model/      # GORM models
│   │   ├── repository/ # DB access
│   │   ├── service/    # Business logic
│   │   └── storage/    # Storage providers (rclone + direct SDKs)
│   └── pkg/            # Generic utilities
├── web/                # React frontend (Vite)
│   └── src/
│       ├── components/
│       ├── pages/
│       ├── services/
│       └── types/
├── docs-site/          # This documentation site (Docusaurus)
├── deploy/             # install.sh, systemd unit, nginx config
└── Makefile
```
