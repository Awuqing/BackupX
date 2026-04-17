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
| **备份类型** | 文件/目录（多源路径）、MySQL、PostgreSQL、SQLite、SAP HANA（完整/增量/差异/日志备份 + 并行通道 + 失败重试） |
| **SAP HANA Backint 代理** | 内置 SAP HANA Backint 协议代理，HANA 原生备份接口可直接把数据路由到 BackupX 支持的任意存储后端 |
| **70+ 存储后端** | 内置阿里云 OSS / 腾讯云 COS / 七牛云 / S3 / Google Drive / WebDAV / FTP + 通过 rclone 集成 SFTP、Azure Blob、Dropbox、OneDrive 等 70+ 后端 |
| **自动调度** | Cron 定时 + 可视化编辑器 + 自动保留策略（按天数/份数清理，自动回收空目录） |
| **多节点** | Master-Agent 集群，统一管理多台服务器的备份，支持远程目录浏览与节点编辑 |
| **安全** | JWT + bcrypt + AES-256-GCM 加密配置 + 可选备份文件加密 + 完整审计日志 |
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
| SFTP / Azure / Dropbox / OneDrive 等 | 选择对应类型后填写必填项，高级配置可折叠展开 |

> 国内云厂商只需填 Region 和 AccessKey，系统自动组装 Endpoint。Rclone 类型的配置项按必填/可选分层展示，高级选项默认折叠。

添加后点击 **测试连接** 确认配置正确。

### 4. 创建备份任务

进入 **备份任务** 页面，点击 **新建**，三步完成：

1. **基础信息** — 任务名称、备份类型、Cron 表达式（留空则仅手动执行）
2. **源配置** — 文件备份选择源路径（支持多个）、数据库备份填写连接信息
3. **存储与策略** — 选择存储目标（支持多个）、压缩策略、保留天数、是否加密

保存后可以点击 **立即执行** 测试，在 **备份记录** 页面实时查看执行日志。

> 删除备份任务时会自动清理远端存储上的备份文件，但保留备份记录以供审计追溯。

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

版本更新：在 **系统设置** 页面点击「检查更新」查看是否有新版本，然后手动执行 `docker compose pull && docker compose up -d` 完成升级。

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

## SAP HANA 支持

BackupX 提供两种 SAP HANA 备份模式，按需选用：

### 模式一：hdbsql Runner（Web 控制台托管）

通过 Web 控制台创建 SAP HANA 备份任务，后端调用 `hdbsql` 执行备份，适合 BackupX 调度的周期性作业。

**源配置步骤支持：**

| 字段 | 可选值 | 说明 |
|------|--------|------|
| 备份类型 | `data` / `log` | 数据备份或日志备份 |
| 备份级别 | `full` / `incremental` / `differential` | 日志备份时自动禁用 |
| 并行通道数 | `1 ~ 32` | `BACKUP DATA USING FILE ('c1','c2',...)` 多路径并发 |
| 失败重试次数 | `1 ~ 10` | 指数退避（5s × 尝试次数²） |
| 实例编号 | 可选 | 从端口推断或手动指定 |

### 模式二：Backint 协议代理（HANA 原生接口）

BackupX 内置 Backint Agent，SAP HANA 通过原生 `BACKUP DATA USING BACKINT` 语法调用，数据自动路由到 BackupX 存储目标（S3 / OSS / COS / WebDAV / 70+ 后端）。

**1. 准备参数文件** `/opt/backupx/backint_params.ini`：

```ini
#STORAGE_TYPE = s3
#STORAGE_CONFIG_JSON = /opt/backupx/storage.json
#PARALLEL_FACTOR = 4
#COMPRESS = true
#KEY_PREFIX = hana-backup
#CATALOG_DB = /opt/backupx/backint_catalog.db
#LOG_FILE = /var/log/backupx/backint.log
```

**2. 准备存储配置** `/opt/backupx/storage.json`（与 BackupX 存储目标配置一致）：

```json
{
  "endpoint": "https://s3.amazonaws.com",
  "region": "us-east-1",
  "bucket": "hana-prod",
  "accessKeyId": "AKIA...",
  "secretAccessKey": "..."
}
```

**3. 创建 hdbbackint 软链接：**

```bash
ln -s /opt/backupx/backupx /usr/sap/<SID>/SYS/global/hdb/opt/hdbbackint
```

**4. 在 HANA `global.ini` 中启用：**

```ini
[backup]
data_backup_using_backint = true
catalog_backup_using_backint = true
log_backup_using_backint = true
data_backup_parameter_file = /opt/backupx/backint_params.ini
log_backup_parameter_file = /opt/backupx/backint_params.ini
```

**5. CLI 手动调用（用于排查）：**

```bash
backupx backint -f backup  -i input.txt -o output.txt -p backint_params.ini
backupx backint -f restore -i input.txt -o output.txt -p backint_params.ini
backupx backint -f inquire -i input.txt -o output.txt -p backint_params.ini
backupx backint -f delete  -i input.txt -o output.txt -p backint_params.ini
```

Backint Agent 使用本地 SQLite 维护 `EBID ↔ 对象键` 目录，所有操作遵循 SAP HANA Backint 协议（`#PIPE` / `#SAVED` / `#RESTORED` / `#BACKUP` / `#NOTFOUND` / `#DELETED` / `#ERROR`）。

---

## 多节点集群

BackupX 支持 Master-Agent 模式管理多台服务器：备份任务可以指定在哪个节点执行，Agent 在本地完成备份并直接上传到存储后端。

### 架构概览

```
[Web 控制台] ←── JWT ──→ [Master (backupx)]
                              ↑  ↓
                              │  │ HTTP 长轮询 (token 认证)
                              │  ↓
                          [Agent (backupx agent)]  ← 运行在远程服务器
                              ↓
                          [70+ 存储后端]
```

- **通信协议**：HTTP 长轮询，Agent 主动发起所有连接，无需 Master 反向访问
- **心跳**：Agent 每 15s 上报一次；Master 每 15s 扫描，超过 45s 未心跳判为离线
- **任务下发**：Master 通过数据库命令队列派发 `run_task`，Agent 轮询拉取
- **执行**：Agent 本地复用 BackupRunner（file / mysql / postgresql / sqlite / saphana）并直接上传到存储
- **安全**：每个节点独立 Token；Agent 不持有 Master 的 JWT 密钥和加密密钥

### 使用步骤

**1. 在 Master 创建节点并获取 Token**

Web 控制台 → **节点管理** → **添加节点**，填写节点名称并保存。界面会显示一个 64 字节十六进制令牌（仅显示一次，请妥善保存）。

**2. 在远程服务器部署 Agent**

把 BackupX 二进制上传到目标服务器（与 Master 同一个文件），然后用以下任一方式启动：

```bash
# 方式 A：CLI 参数
backupx agent --master http://master.example.com:8340 --token <token>

# 方式 B：配置文件
cat > /etc/backupx/agent.yaml <<EOF
master: http://master.example.com:8340
token: <token>
heartbeatInterval: 15s
pollInterval: 5s
tempDir: /var/lib/backupx-agent
EOF
backupx agent --config /etc/backupx/agent.yaml

# 方式 C：环境变量（适合 Docker / systemd）
BACKUPX_AGENT_MASTER=http://master.example.com:8340 \
BACKUPX_AGENT_TOKEN=<token> \
backupx agent
```

启动成功后，Master 的节点列表会把该节点标记为**在线**。

**3. 创建路由到该节点的备份任务**

在 **备份任务** 页面新建任务时选择对应节点。任务被触发后：

- 本机节点或未指定节点（`nodeId=0`）：由 Master 进程本地执行
- 远程节点：Master 写入命令队列 → Agent 轮询拉取 → 本地执行并上传 → 上报记录

### 限制说明

- **不支持加密备份**：Agent 不持有 Master 的 AES-256 加密密钥，启用 `encrypt: true` 的任务会路由到 Agent 时失败
- **目录浏览超时**：远程目录浏览通过命令队列做同步 RPC，默认 15s 超时，网络慢时可能失败
- **命令超时**：Agent 领取但未完成的命令超过 10min 会被标记为超时

### CLI 参考

```bash
backupx agent --help
  -master string    Master URL
  -token string     Agent 认证令牌
  -config string    YAML 配置文件路径（优先级高于环境变量）
  -temp-dir string  本地临时目录（默认 /tmp/backupx-agent）
  -insecure-tls     跳过 TLS 证书校验（仅测试用）
```

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
git tag v1.4.3 && git push --tags
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
| | `GET /storage-targets/rclone/backends` | Rclone 后端列表 |
| **节点** | `GET\|POST /nodes` | 列表 / 添加 |
| | `PUT /nodes/:id` | 编辑节点 |
| | `GET /nodes/:id/fs/list` | 目录浏览 |
| | `POST /agent/heartbeat` | Agent 心跳（Token 认证） |
| **通知** | `GET\|POST /notifications` | 列表 / 添加 |
| **仪表盘** | `GET /dashboard/stats` | 概览统计 |
| **审计日志** | `GET /audit-logs` | 操作审计 |
| **系统** | `GET /system/info` | 系统信息 |
| | `GET /system/update-check` | 检查版本更新 |

---

## 技术栈

| 组件 | 技术 |
|------|------|
| **后端** | Go · Gin · GORM · SQLite · robfig/cron · rclone |
| **前端** | React 18 · TypeScript · ArcoDesign · Vite · Zustand · ECharts |
| **存储** | rclone（70+ 后端）· AWS SDK v2 · Google Drive API v3 |
| **安全** | JWT · bcrypt · AES-256-GCM |

## Contributing

欢迎提交 Issue 和 Pull Request！

## License

[Apache License 2.0](LICENSE)
