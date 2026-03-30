<p align="right">
  <a href="README_EN.md">English</a> | <strong>中文</strong>
</p>
<p align="center">
  <h1 align="center">BackupX</h1>
  <p align="center">
    <strong>自托管服务器备份管理平台</strong><br>
    一个二进制，一条命令，管好你所有服务器的备份。
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
<td width="50%"><img src="screenshots/dashboard.png" alt="仪表盘"></td>
<td width="50%"><img src="screenshots/backup-tasks.png" alt="备份任务"></td>
</tr>
<tr>
<td><img src="screenshots/storage-targets.png" alt="存储目标"></td>
<td><img src="screenshots/backup-records.png" alt="备份记录"></td>
</tr>
</table>

## 功能亮点

| 能力 | 说明 |
|------|------|
| **备份类型** | 文件/目录（多源路径）、MySQL、PostgreSQL、SQLite、SAP HANA |
| **存储后端** | 阿里云 OSS、腾讯云 COS、七牛云、S3 兼容(AWS/MinIO/R2)、Google Drive、WebDAV、FTP/FTPS、本地磁盘 |
| **自动调度** | Cron 定时 + 可视化编辑器 + 自动保留策略（按天数/份数清理） |
| **多节点** | Master-Agent 集群，统一管理多台服务器的备份 |
| **安全** | JWT + bcrypt + AES-256-GCM 加密配置 + 可选备份文件加密 + 审计日志 |
| **通知** | 邮件 / Webhook / Telegram，备份成功或失败时自动推送 |
| **部署** | 单二进制 + 内嵌 SQLite，Docker 一键启动，零外部依赖 |

---

## 快速开始

### 1. 安装

**Docker（推荐，无需克隆仓库）：**

```bash
# 创建 docker-compose.yml 后一键启动
docker compose up -d

# 或直接运行
docker run -d --name backupx -p 8340:8340 -v backupx-data:/app/data awuqing/backupx:latest
```

> Docker Hub 镜像：[`awuqing/backupx`](https://hub.docker.com/r/awuqing/backupx)，支持 linux/amd64 和 linux/arm64。

<details>
<summary>docker-compose.yml 参考</summary>

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
      # 挂载需要备份的宿主机目录（按需添加）：
      # - /var/www:/mnt/www:ro
      # - /etc/nginx:/mnt/nginx-conf:ro
    environment:
      - TZ=Asia/Shanghai

volumes:
  backupx-data:
```

</details>

**预编译包（裸机部署）：**

从 [Releases](https://github.com/Awuqing/BackupX/releases) 下载对应平台的压缩包：

```bash
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh        # 自动配置 systemd + Nginx
```

**从源码构建：**

```bash
git clone https://github.com/Awuqing/BackupX.git && cd BackupX
make build               # 构建前后端
make docker-cn           # 或用国内镜像构建 Docker（goproxy.cn / npmmirror / 阿里云 apk）
```

### 2. 打开控制台

浏览器访问 `http://your-server:8340`，首次打开会引导创建管理员账户。

### 3. 添加存储目标

进入 **存储目标** 页面，点击 **添加**，选择存储类型并填写凭证：

| 存储类型 | 需要填写 |
|---------|---------|
| 阿里云 OSS | Region + AccessKey ID/Secret + Bucket |
| 腾讯云 COS | Region + SecretId/SecretKey + Bucket（格式 `name-appid`） |
| 七牛云 Kodo | Region + AccessKey/SecretKey + Bucket |
| S3 兼容 | Endpoint + AccessKey + Bucket |
| Google Drive | Client ID/Secret → 点击授权完成 OAuth |
| WebDAV | 服务器地址 + 用户名/密码 |
| FTP | 主机 + 端口 + 用户名/密码 |
| 本地磁盘 | 目标目录路径 |

> 国内云厂商只需填 Region 和 AccessKey，系统自动组装 Endpoint。

添加后点击 **测试连接** 确认配置正确。

### 4. 创建备份任务

进入 **备份任务** 页面，点击 **新建**，三步完成：

1. **基础信息** — 任务名称、备份类型、Cron 表达式（留空则仅手动执行）
2. **源配置** — 文件备份选择源路径（支持多个）、数据库备份填写连接信息
3. **存储与策略** — 选择存储目标、压缩策略、保留天数、是否加密

保存后可以点击 **立即执行** 测试，在 **备份记录** 页面实时查看执行日志。

### 5. 配置通知（可选）

进入 **通知配置** 页面，支持邮件、Webhook、Telegram 三种方式，可分别配置成功/失败时是否推送。

---

## 部署指南

### Docker 部署

```bash
docker compose up -d     # 使用上方的 docker-compose.yml
```

备份宿主机目录时需要挂载路径（在 docker-compose.yml 的 `volumes` 中添加）：

```yaml
volumes:
  - backupx-data:/app/data
  - /var/www:/mnt/www:ro              # 挂载需要备份的目录
  - /etc/nginx:/mnt/nginx-conf:ro     # 可以挂载多个
```

通过环境变量调整配置：

```yaml
environment:
  - TZ=Asia/Shanghai
  - BACKUPX_LOG_LEVEL=debug
  - BACKUPX_BACKUP_MAX_CONCURRENT=4
```

### 裸机部署

```bash
# 使用预编译包
tar xzf backupx-v*-linux-amd64.tar.gz && cd backupx-*
sudo ./install.sh

# 或从源码
make build
sudo ./deploy/install.sh
```

安装脚本自动完成：创建系统用户 → 安装二进制到 `/opt/backupx/` → 配置 systemd → 配置 Nginx 反向代理。

### Nginx 反向代理（裸机部署时）

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

### 配置文件

配置文件路径 `./config.yaml`，也可通过 `BACKUPX_` 前缀环境变量覆盖：

```yaml
server:
  port: 8340
database:
  path: "./data/backupx.db"
security:
  jwt_secret: ""          # 留空自动生成并持久化到数据库
  encryption_key: ""      # 留空自动生成
backup:
  temp_dir: "/tmp/backupx"
  max_concurrent: 2
log:
  level: "info"           # debug | info | warn | error
  file: "./data/backupx.log"
```

### 密码重置

忘记管理员密码时通过 CLI 重置：

```bash
# 裸机
./backupx reset-password --username admin --password newpass123

# Docker
docker exec -it backupx /app/bin/backupx reset-password --username admin --password newpass123
```

---

## 多节点集群

BackupX 支持 Master-Agent 模式管理多台服务器：

1. Web 控制台 → **节点管理** → **添加节点**，系统生成 Token
2. 在远程服务器部署 Agent 并使用 Token 连接 Master
3. 创建备份任务时选择对应节点，Master 自动下发任务

创建文件备份任务时，可通过可视化目录浏览器远程选择 Agent 节点上的目录，无需手动输入路径。

---

## 开发指南

**环境要求：** Go >= 1.25 · Node.js >= 20 · npm

```bash
# 开发模式
make dev-server          # 终端 1：后端（默认 :8340）
make dev-web             # 终端 2：前端（Vite HMR）

# 测试
make test                # 运行全部测试

# 构建
make build               # 前后端一起构建
make docker              # Docker 构建
make docker-cn           # 国内 Docker 构建（镜像加速）
```

### 发版

```bash
git tag v1.2.3 && git push --tags
# GitHub Actions 自动：编译双架构二进制 → 发布 GitHub Release → 推送 Docker Hub 镜像
```

也可在 GitHub Actions 页面手动触发 Release workflow。

---

## API 参考

所有接口以 `/api` 为前缀，使用 JWT Bearer Token 认证。

| 模块 | 端点 | 说明 |
|------|------|------|
| **认证** | `POST /auth/setup` | 初始化管理员 |
| | `POST /auth/login` | 登录 |
| | `PUT /auth/password` | 修改密码 |
| **备份任务** | `GET\|POST /backup/tasks` | 列表 / 创建 |
| | `GET\|PUT\|DELETE /backup/tasks/:id` | 详情 / 更新 / 删除 |
| | `PUT /backup/tasks/:id/toggle` | 启用/禁用 |
| | `POST /backup/tasks/:id/run` | 手动执行 |
| **备份记录** | `GET /backup/records` | 列表（支持筛选） |
| | `GET /backup/records/:id/logs/stream` | 实时日志 (SSE) |
| | `GET /backup/records/:id/download` | 下载 |
| | `POST /backup/records/:id/restore` | 恢复 |
| **存储目标** | `GET\|POST /storage-targets` | 列表 / 添加 |
| | `POST /storage-targets/test` | 测试连接 |
| **节点** | `GET\|POST /nodes` | 列表 / 添加 |
| | `GET /nodes/:id/fs/list` | 目录浏览 |
| **通知** | `GET\|POST /notifications` | 列表 / 添加 |
| **仪表盘** | `GET /dashboard/stats` | 概览统计 |
| **审计日志** | `GET /audit-logs` | 操作审计 |
| **系统** | `GET /system/info` | 系统信息 |

---

## 技术栈

| 组件 | 技术 |
|------|------|
| **后端** | Go · Gin · GORM · SQLite · robfig/cron |
| **前端** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **存储** | AWS SDK v2 · Google Drive API v3 · gowebdav · jlaffaye/ftp |
| **安全** | JWT · bcrypt · AES-256-GCM |

## Contributing

欢迎提交 Issue 和 Pull Request！

## License

[Apache License 2.0](LICENSE)
