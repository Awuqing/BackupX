---
sidebar_position: 1
title: 开发环境
description: 搭建 BackupX 本地开发环境 — 后端、前端、测试。
---

# 开发环境

**环境要求：** Go ≥ 1.25，Node.js ≥ 20，npm。

## 克隆与依赖

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
cd web && npm install && cd ..
```

## 开发服务

开两个终端分别跑后端和前端：

```bash
# 终端 1：后端，监听 :8340
make dev-server

# 终端 2：Vite HMR，监听 :5173
make dev-web
```

Vite 配置了 `/api` 代理到 `http://127.0.0.1:8340`，浏览器直接访问 `http://localhost:5173`。

## 测试

```bash
make test           # 运行 Go + Web 全部测试
make test-server    # 仅 Go
make test-web       # 仅 Vitest
```

## 生产构建

```bash
make build          # server/bin/backupx + web/dist
make docker         # Docker 镜像
make docker-cn      # 国内镜像加速构建
```

## 技术栈

| 组件 | 技术 |
|------|------|
| **后端** | Go · Gin · GORM · SQLite · robfig/cron · rclone |
| **前端** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **存储** | rclone（70+ 后端）· AWS SDK v2 · Google Drive API v3 |
| **安全** | JWT · bcrypt · AES-256-GCM |

## 目录结构

```
BackupX/
├── server/             # Go 后端
│   ├── cmd/backupx/    # 入口 + 子命令（agent / backint / reset-password）
│   ├── internal/
│   │   ├── agent/      # Agent CLI 逻辑
│   │   ├── app/        # 装配（repo → service → handler）
│   │   ├── backup/     # 备份 runner（file / mysql / postgres / sqlite / saphana）
│   │   ├── backint/    # SAP HANA Backint 协议
│   │   ├── http/       # HTTP handler + router
│   │   ├── model/      # GORM 模型
│   │   ├── repository/ # 数据访问
│   │   ├── service/    # 业务逻辑
│   │   └── storage/    # 存储 provider（rclone + 直接 SDK）
│   └── pkg/            # 通用工具
├── web/                # React 前端（Vite）
│   └── src/
│       ├── components/
│       ├── pages/
│       ├── services/
│       └── types/
├── docs-site/          # 文档站（Docusaurus）
├── deploy/             # install.sh / systemd unit / nginx config
└── Makefile
```
