# 一键部署 Agent 设计文档

- **Issue**: [#43 Feature: 一键部署 Agent](https://github.com/Awuqing/BackupX/issues/43)
- **日期**: 2026-04-19
- **状态**: 设计已评审，待实施
- **相关版本**: BackupX v1.7.x（基于 v1.6.0 多节点能力迭代）

## 1. 背景与目标

### 1.1 现状

`docs-site/docs/features/multi-node.md` 描述的多节点集群功能已于 v1.6.0 上线。当前新增一个远程 Agent 的流程：

1. Web 控制台"添加节点" → 输入名称 → 后端生成 64 字节 hex token 并**仅显示一次**
2. 管理员手动复制 token，登录目标机，上传 `backupx` 二进制
3. 拼装 `backupx agent --master URL --token XXX` 命令手动执行
4. 如需守护进程，复制 `docs-site/docs/features/multi-node.md` 里 systemd unit 模板，自行填入 token

问题：在 Issue #43 描述的"大量远端 Agent 节点场景"下，步骤 2-4 存在大量手工劳动且易错。

### 1.2 目标

参考 Komari 的节点向导体验，提供：

- **UI 向导**：勾选参数（模式 / 架构 / 版本 / 批量）→ 生成一行 `curl | sudo sh` 命令
- **一次性安装链接**：token 不出现在命令行，15 min 后自动失效
- **幂等安装脚本**：支持 systemd / Docker / 前台三种模式，重跑可升级
- **批量创建**：一次生成 N 个节点 + N 条命令，支持导出 `.sh`

### 1.3 非目标（V1 不做）

- Windows / macOS 节点（现有二进制矩阵仅 `linux/{amd64,arm64}`）
- 一键卸载命令（通过手动 `systemctl disable --now` + 删目录即可）
- Agent 自动升级守护（重跑脚本即升级，足够满足 V1）
- 跨 Master 漂移（节点固定绑 Master）

## 2. 总体架构

```
┌───────────────────────────────────────────┐
│  Web Console (React + Arco Design)        │
│  节点管理页 → "添加节点" 向导               │
│    步骤1: 名称 (支持多行批量)               │
│    步骤2: 勾选 模式/架构/版本              │
│    步骤3: 显示一行命令 + 脚本预览 Tabs     │
└──────────────┬────────────────────────────┘
               │ JWT (管理员)
               ▼
┌───────────────────────────────────────────┐
│  Master (Go + Gin)                         │
│                                            │
│  POST /api/nodes/batch   ← 批量创建节点    │
│  POST /api/nodes/:id/install-tokens        │
│    → 返回 15min 一次性 install token       │
│  POST /api/nodes/:id/rotate-token          │
│    → 生成新 agent token，旧 24h TTL        │
│                                            │
│  (下面两个端点不走 JWT 中间件)              │
│  GET  /install/:install_token              │
│    → 渲染 shell 脚本（token、master烧入）  │
│    → 消费即作废                            │
│  GET  /install/:install_token/compose.yml  │
│    → 返回 docker-compose.yml 片段          │
│  GET  /api/v1/agent/self                   │
│    → Agent token 认证，供脚本探活          │
└──────────────┬────────────────────────────┘
               │ curl | sudo sh
               ▼
┌───────────────────────────────────────────┐
│  目标主机 (Linux)                          │
│  agent-install.sh 内部流程：                │
│   1. 检测 OS/Arch                          │
│   2. 下载二进制 (GitHub / ghproxy 镜像)    │
│   3. 写 systemd unit 或 docker run         │
│   4. 启动并等待 master 确认 online         │
└───────────────────────────────────────────┘
```

### 2.1 新增 vs 复用模块

| 类型 | 新增 | 已存在（扩展） |
|---|---|---|
| 后端 Go 包 | `internal/installtoken`（CRUD + GC + TTL）<br/>`internal/installscript`（模板渲染） | `internal/service/node_service.go`（批量创建、轮换 token）<br/>`internal/service/agent_service.go`（Self 端点） |
| 路由 | `install_handler.go` 公开路由分组 | `router.go` 注册 |
| 数据表 | `agent_install_tokens` | `nodes` 表加 2 列（`prev_token`, `prev_token_expires`） |
| 模板 | `deploy/agent-install.sh.tmpl`<br/>`deploy/agent-compose.yml.tmpl`（嵌入二进制） | — |
| 前端页面 | `web/src/pages/nodes/AgentInstallWizard.tsx` | `NodesPage.tsx`（替换旧 Modal） |
| 前端服务 | `web/src/services/nodes.ts` 新 API 函数 | 现有 listNodes/createNode 保留 |
| 文档 | 更新 `docs-site/docs/features/multi-node.md` 中英双份 | — |

## 3. 数据模型

### 3.1 新增表 `agent_install_tokens`

```go
// server/internal/model/agent_install_token.go
type AgentInstallToken struct {
    ID          uint       `gorm:"primaryKey"`
    Token       string     `gorm:"size:64;uniqueIndex;not null"` // 32 字节 hex (crypto/rand)
    NodeID      uint       `gorm:"not null;index"`
    Mode        string     `gorm:"size:16;not null"`              // systemd|docker|foreground
    Arch        string     `gorm:"size:16;not null"`              // amd64|arm64|auto
    AgentVer    string     `gorm:"size:32;not null"`              // 如 v1.7.0
    DownloadSrc string     `gorm:"size:16;not null;default:'github'"` // github|ghproxy
    ExpiresAt   time.Time  `gorm:"not null;index"`
    ConsumedAt  *time.Time                                        // 非空即作废
    CreatedByID uint       `gorm:"not null"`                      // 审计
    CreatedAt   time.Time
}

func (AgentInstallToken) TableName() string { return "agent_install_tokens" }
```

AutoMigrate 在 `initialize/gorm.go` 中追加。后台 GC 每 1h 扫描 `ExpiresAt < now - 7d` 记录并硬删除。

### 3.2 扩展 `nodes` 表

```go
// server/internal/model/node.go - 新增两列
PrevToken         string     `gorm:"size:128;index" json:"-"`
PrevTokenExpires  *time.Time `gorm:"column:prev_token_expires" json:"-"`
```

`NodeRepository.FindByToken`：先查 `token`，未命中时查 `prev_token AND prev_token_expires > now`。

## 4. API 契约

所有写接口走 `middleware.RequireAuth`（JWT），除 `/install/:token` 与 `/install/:token/compose.yml` 公开。

### 4.1 批量创建节点

```
POST /api/nodes/batch
Authorization: Bearer <JWT>
{
  "names": ["prod-db-01", "prod-db-02", "prod-web-01"]
}

→ 200
{
  "data": [
    { "id": 42, "name": "prod-db-01" },
    { "id": 43, "name": "prod-db-02" },
    { "id": 44, "name": "prod-web-01" }
  ]
}
```

验证：`len(names) in [1, 50]`、去重、每项长度 1-128、不允许空白。事务内创建，任一失败全回滚。响应**不含 agent token**，前端随后为每个节点单独请求 install-token。

### 4.2 生成一次性安装令牌

```
POST /api/nodes/42/install-tokens
Authorization: Bearer <JWT>
{
  "mode": "systemd",        // systemd|docker|foreground
  "arch": "auto",           // amd64|arm64|auto
  "agentVersion": "v1.7.0", // 默认 Master 自身版本
  "downloadSrc": "github",  // github|ghproxy
  "ttlSeconds": 900         // 默认 900，范围 300-86400
}

→ 200
{
  "data": {
    "installToken": "Xk3p9...vM",
    "expiresAt": "2026-04-19T13:15:00Z",
    "url": "https://master.example.com/install/Xk3p9...vM",
    "composeUrl": "https://master.example.com/install/Xk3p9...vM/compose.yml"
  }
}
```

- `composeUrl` **仅当 `mode=docker` 时返回**；其他模式该字段为空字符串或省略
- 速率限制：每节点 60s 内最多 5 次（内存令牌桶）
- 审计：`audit_log` 记录 `resource=install_token, action=create, target=nodeId`

### 4.3 轮换 Agent Token

```
POST /api/nodes/42/rotate-token
Authorization: Bearer <JWT>

→ 200
{
  "data": { "newToken": "...（64 hex）..." }
}
```

逻辑：`node.prev_token = node.token; node.prev_token_expires = now + 24h; node.token = newToken`。UI 提示"24h 内旧 Token 仍可认证"，便于滚动部署。

**对已发未消费的 install token 的影响**：install token 尚未消费时，token.agent_token 字段不存在（脚本渲染时从 `node.Token` 动态读），因此轮换后新生成的脚本使用新 token，旧 token 自动退役。已被消费（已安装）的 Agent 通过 prev_token 机制继续可用 24h。

### 4.4 公开端点：渲染安装脚本

```
GET /install/:installToken

→ 200 (text/x-shellscript; charset=utf-8)
#!/bin/sh
...（见 §5 模板）
```

**内容按 token.Mode 分派**：

| `token.Mode` | 返回内容 | Content-Type |
|---|---|---|
| `systemd` | §5.2 完整 systemd 安装脚本 | `text/x-shellscript` |
| `foreground` | §5.3 前台运行脚本 | `text/x-shellscript` |
| `docker` | §5.4 内嵌 `docker run` 的 shell 脚本 | `text/x-shellscript` |

- **不走** JWT 中间件；限流：单 IP 20 req/min（滑动窗口）
- 查询 `agent_install_tokens`：不存在/已过期/已消费 → 返回 410 Gone（纯文本错误，便于终端显示）
- 查询成功：`ConsumedAt = now` 后提交事务；然后渲染脚本
- 审计：`audit_log` 记录 `resource=install_token, action=consume, ip=<remote>`

### 4.5 公开端点：Docker Compose 片段（仅 Docker 模式可选）

```
GET /install/:installToken/compose.yml

→ 200 (text/yaml)
version: "3.8"
services:
  ...
```

- **仅当 `token.Mode == docker` 时有效**；其他 Mode 返回 400 `MODE_MISMATCH`
- `/install/:token` 与 `/install/:token/compose.yml` **共享同一枚 token 的消费状态**：任一端点首次命中即消费；另一端点随后访问 410（用户二选一）
- 规则同 §4.4（限流 + 审计）

### 4.6 Agent 探活端点

```
GET /api/v1/agent/self
X-Agent-Token: <agent_token>

→ 200
{
  "data": {
    "id": 42,
    "name": "prod-db-01",
    "status": "online",
    "lastSeen": "2026-04-19T13:02:10Z"
  }
}
```

走 Agent token 认证（`AgentService.AuthenticatedNode`），供 `agent-install.sh` 末尾轮询确认上线。

### 4.7 管理员脚本预览端点（不消费 install token）

```
GET /api/nodes/:id/install-script-preview?mode=systemd&arch=auto&agentVersion=v1.7.0&downloadSrc=github
Authorization: Bearer <JWT>

→ 200 (text/x-shellscript; charset=utf-8)
#!/bin/sh
# 预览脚本（未绑定 install token，token 占位为 <AGENT_TOKEN>）
...
```

- 仅管理员可用，走 JWT 中间件
- 渲染时 `AGENT_TOKEN` 字段使用 `<AGENT_TOKEN>` 字面量占位（不暴露真实 token）
- 前端 Step 3 "展开脚本预览" 调此端点；与 install token 的创建/消费完全解耦

## 5. 安装脚本模板

文件 `deploy/agent-install.sh.tmpl`，通过 `embed` 打包进二进制：

```go
//go:embed agent-install.sh.tmpl
var agentInstallTmpl string
```

### 5.1 模板变量

```go
type InstallScriptContext struct {
    MasterURL      string // 从请求 Host + X-Forwarded-Proto 推导，或系统配置覆盖
    AgentToken     string // node.Token（真正的长期凭证）
    AgentVersion   string // 如 v1.7.0
    Arch           string // amd64|arm64|auto
    Mode           string // systemd|docker|foreground
    DownloadBase   string // github 或 ghproxy 渲染期决定
    InstallPrefix  string // 默认 /opt/backupx-agent
}
```

`DownloadBase` 映射：

| `downloadSrc` | 渲染值 |
|---|---|
| `github` | `https://github.com/Awuqing/BackupX/releases/download` |
| `ghproxy` | `https://ghproxy.com/https://github.com/Awuqing/BackupX/releases/download` |

### 5.2 systemd 模式脚本骨架

```bash
#!/bin/sh
set -eu
MASTER_URL="{{.MasterURL}}"
AGENT_TOKEN="{{.AgentToken}}"
AGENT_VERSION="{{.AgentVersion}}"
DOWNLOAD_BASE="{{.DownloadBase}}"
INSTALL_PREFIX="{{.InstallPrefix}}"
ARCH="{{.Arch}}"

# 1. 前置检查
[ "$(id -u)" -eq 0 ] || { echo "请使用 root 或 sudo 执行" >&2; exit 1; }
command -v systemctl >/dev/null || { echo "不支持非 systemd 系统" >&2; exit 1; }
command -v curl >/dev/null || command -v wget >/dev/null \
    || { echo "需要 curl 或 wget" >&2; exit 1; }

# 2. 架构检测
if [ "$ARCH" = "auto" ]; then
    case "$(uname -m)" in
        x86_64|amd64)   ARCH=amd64 ;;
        aarch64|arm64)  ARCH=arm64 ;;
        *) echo "不支持的架构：$(uname -m)" >&2; exit 1 ;;
    esac
fi

# 3. 下载二进制
ARCHIVE="backupx-${AGENT_VERSION}-linux-${ARCH}.tar.gz"
URL="${DOWNLOAD_BASE}/${AGENT_VERSION}/${ARCHIVE}"
TMPDIR="$(mktemp -d)"; trap 'rm -rf "$TMPDIR"' EXIT
echo "[1/4] 下载 ${URL}"
if command -v curl >/dev/null; then
    curl -fsSL "$URL" -o "$TMPDIR/pkg.tar.gz"
else
    wget -qO "$TMPDIR/pkg.tar.gz" "$URL"
fi
tar xzf "$TMPDIR/pkg.tar.gz" -C "$TMPDIR"

# 4. 安装
echo "[2/4] 安装到 ${INSTALL_PREFIX}"
id backupx >/dev/null 2>&1 || useradd --system --home-dir "$INSTALL_PREFIX" --shell /usr/sbin/nologin backupx
install -d -o backupx -g backupx "$INSTALL_PREFIX" /var/lib/backupx-agent
install -m 0755 "$TMPDIR/backupx-${AGENT_VERSION}-linux-${ARCH}/backupx" "$INSTALL_PREFIX/backupx"

# 5. systemd unit
echo "[3/4] 配置 systemd"
cat > /etc/systemd/system/backupx-agent.service <<UNIT
[Unit]
Description=BackupX Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=backupx
Environment="BACKUPX_AGENT_MASTER=${MASTER_URL}"
Environment="BACKUPX_AGENT_TOKEN=${AGENT_TOKEN}"
ExecStart=${INSTALL_PREFIX}/backupx agent --temp-dir /var/lib/backupx-agent
Restart=on-failure
RestartSec=10s
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now backupx-agent

# 6. 等待上线
echo "[4/4] 等待节点上线"
for i in $(seq 1 15); do
    sleep 2
    if curl -fsSL -H "X-Agent-Token: ${AGENT_TOKEN}" "${MASTER_URL}/api/v1/agent/self" 2>/dev/null \
        | grep -q '"status":"online"'; then
        echo "✓ 节点已上线"
        exit 0
    fi
done
echo "⚠ 30s 内未收到上线心跳，请检查防火墙或 journalctl -u backupx-agent"
exit 2
```

### 5.3 foreground 模式差异

跳过步骤 5（systemd unit），第 6 步替换为：

```bash
echo "前台运行（Ctrl+C 退出）"
exec "${INSTALL_PREFIX}/backupx" agent
```

### 5.4 Docker Compose 模板（`deploy/agent-compose.yml.tmpl`）

```yaml
version: "3.8"
services:
  backupx-agent:
    image: awuqing/backupx:{{.AgentVersion}}
    command: ["agent"]
    restart: unless-stopped
    environment:
      BACKUPX_AGENT_MASTER: "{{.MasterURL}}"
      BACKUPX_AGENT_TOKEN: "{{.AgentToken}}"
    volumes:
      - /var/lib/backupx-agent:/tmp/backupx-agent
```

UI 同时给出一行等价命令：

```bash
docker run -d --restart=always \
  -e BACKUPX_AGENT_MASTER=<MASTER> \
  -e BACKUPX_AGENT_TOKEN=<TOKEN> \
  -v /var/lib/backupx-agent:/tmp/backupx-agent \
  awuqing/backupx:<VER> agent
```

### 5.5 幂等性

- 同一 install token 被 `curl` 消费两次？第二次返回 410 Gone
- 不同 install token 在同机重跑？允许：用户脚本覆盖二进制 + `systemctl restart`，systemd unit 被完整重写（用户自定义修改**会丢失** —— 文档需提示）
- `useradd` / `install -d`：`|| true` 或依赖现有判断，已幂等

## 6. 前端 UI 设计

### 6.1 文件结构

```
web/src/pages/nodes/
  NodesPage.tsx              (改：操作列菜单、嵌入 Wizard)
  AgentInstallWizard.tsx     (新：三步向导主体)
  BatchCommandTable.tsx      (新：批量模式结果表)
  wizard/
    Step1NodeName.tsx
    Step2DeployOptions.tsx
    Step3CommandPreview.tsx

web/src/services/
  nodes.ts                   (改：新增 batchCreateNodes / createInstallToken / rotateNodeToken)
```

### 6.2 向导步骤

**Step 1 · 节点信息**

- 单选 Tab：`单节点` / `批量创建`
- 单节点：`Input`（必填，1-128 字符）
- 批量：`TextArea`（每行一名称，最多 50 行，前端去重）

**Step 2 · 部署参数**

| 字段 | 控件 | 默认 |
|---|---|---|
| 安装模式 | `Radio.Group` systemd/docker/foreground | systemd |
| 架构 | `Select` amd64/arm64/auto | auto |
| Agent 版本 | `Select` 从 `/api/system/releases` 拉最近 5 个 tag，默认 Master 版本 | Master 版本 |
| 有效期 | `Select` 5m/15m/1h/24h | 15m |
| 下载源 | `Radio.Group` github/ghproxy | github |

**Step 3 · 部署命令**

- 单节点：显示一行命令 + `[复制]` + `[展开脚本预览 ▾]`
- 批量：`Table` 每行 {节点名, 命令, 复制按钮}；底部 `[一键导出 .sh]`（拼接所有命令为 heredoc 脚本）
- 倒计时：`setInterval(1s)` 显示 `mm:ss`，到期命令变灰 + "重新生成"按钮

### 6.3 NodesPage 操作列改造

```
操作: 编辑名称 | 生成安装命令 | 重新生成 Token | 删除
```

"生成安装命令"复用向导从 Step 2 开始（跳过 Step 1）。"重新生成 Token"：Popconfirm → 调 `/rotate-token` → 展示新 token 与 24h 过渡提示。

### 6.4 状态与安全

- Wizard state 仅 `useState`，关闭丢弃；不写 localStorage
- install token 显示后立即给"复制"按钮，CSS `user-select: all` 便于复制
- 未消费的 install token 无显式"撤销"接口；依赖 TTL 自然过期

### 6.5 i18n

V1 **不接入 i18n** —— 现有 `NodesPage.tsx` 采用硬编码中文（同页面其他文案均未使用 `useTranslation`），为保持一致性，Wizard 亦使用硬编码中文。待整个 Nodes 页面做 i18n 改造时（单独 PR），再统一提取 `nodes.wizard.*` 键。

## 7. 安全考量

- **install token 独立于 agent token**：即便命令泄露，攻击者只能在 TTL 内拉取一次脚本；拉取后立即作废
- **脚本内容包含 agent token 明文**：这是必要权衡（Agent 启动需要）；UI 文案提示"请在受控终端执行，勿在共享屏幕操作"
- **限流**：`/install/:token` 全局 IP 限流 20 req/min 防扫描；install token 生成接口按节点 5/min
- **审计**：三个关键动作全部落 audit_log
  - `install_token.create`（管理员生成）
  - `install_token.consume`（目标机消费，记 IP）
  - `node.rotate_token`（管理员轮换）
- **MasterURL 推导**：`Scheme + Host`（来自 `X-Forwarded-Proto` / `X-Forwarded-Host`，fallback `c.Request.Host`），允许系统配置项 `system_config.masterExternalUrl` 硬编码覆盖（避免反代下协议错误）
- **二进制完整性**（V2 增强，本期不做）：可选脚本内 `sha256sum` 校验 Release 附带的 `.sha256`

## 8. 测试策略

### 8.1 单元测试

| 包 | 测试点 |
|---|---|
| `installtoken` | 生成 → 查询 → 消费 → 再查询 410；过期记录被 GC 删除 |
| `installscript` | 三种模式的模板渲染快照（golden file）；下载源 github/ghproxy 切换 |
| `node_service` | 批量创建事务回滚；token 轮换后 `prev_token` 仍可认证，24h 后失效 |

### 8.2 集成测试

- `router_test.go` 扩展：
  - `POST /api/nodes/batch` 去重、长度限制
  - `POST /api/nodes/:id/install-tokens` 限流触发
  - `GET /install/:token` 消费语义 + 410 响应
  - `GET /install/:token/compose.yml` 幂等消费与 systemd 互斥（同 token 只能消费一种模式）

### 8.3 端到端测试（本地手动 + CI 可选）

- 启动 Master（Docker）
- 启动 Linux 容器（Ubuntu 22.04，内含 systemd）
- 容器内 `curl -fsSL http://master/install/<token> | sh`
- 断言：30s 内 `GET /api/nodes` 返回该节点 `status=online`

### 8.4 手动验收

- [ ] systemd 模式：clean 系统、已装老版本（重跑升级）、破坏性（手改 unit → 重跑覆盖）
- [ ] Docker 模式：从 UI 拷 compose → `docker-compose up -d`
- [ ] foreground 模式：SSH 里 `curl | sh` 直接运行
- [ ] ghproxy 模式：国内 VPS 能下载
- [ ] 批量创建 20 节点，脚本导出后逐机执行
- [ ] install token 过期后 UI 重新生成

## 9. 分阶段发布

合 main 即 GA（不走 feature flag），通过模块独立性支持单 PR 或拆分 PR：

1. **Phase 1 — 后端**（独立可合入）
   - 表迁移 + `installtoken` 包 + `installscript` 包
   - install 路由 + agent self 端点
   - node_service 扩展（batch + rotate）
   - 单元测试 + 集成测试

2. **Phase 2 — 前端**（依赖 Phase 1 合入）
   - AgentInstallWizard + 三子步骤
   - NodesPage 操作列改造
   - services 层 API 函数
   - i18n 键补齐

3. **Phase 3 — 文档**
   - 更新 `docs-site/docs/features/multi-node.md`（中英）
   - 增加向导截图到 `screenshots/`
   - README 多节点段落引用新流程

## 10. 兼容性与回滚

- **老 Agent**：通过手动 token 启动的 Agent 继续用 `POST /api/nodes/heartbeat` 上线。本次零破坏
- **老 API**：`POST /api/nodes` 单节点创建保留，仅 UI 不再暴露
- **回滚**：向导上线后如发现阻断问题，前端切回原 Modal；后端新端点不影响老流程。DB 迁移可逆：
  ```sql
  DROP TABLE agent_install_tokens;
  ALTER TABLE nodes DROP COLUMN prev_token, DROP COLUMN prev_token_expires;
  ```

## 11. 开放问题与后续

- V2 候选：Windows / macOS 节点支持、一键卸载、二进制 sha256 校验、Master 代理下载（离线内网场景）
- `MasterURL` 在复杂反代（多级 L7）下的推导鲁棒性：V1 用 `X-Forwarded-*`，允许 `system_config` 硬编码；若反馈频繁出错，V2 提供"部署时测试 URL"按钮
- Agent 版本的 `/api/system/releases` 端点是否复用已有"系统更新检查"逻辑（commit `ae658f1`）？**是**：复用 `system_service.CheckUpdate` 的 GitHub Release 查询缓存

## 12. 变更影响摘要

| 文件 / 模块 | 变更类型 |
|---|---|
| `server/internal/model/node.go` | 新增 2 列 |
| `server/internal/model/agent_install_token.go` | 新增文件 |
| `server/internal/repository/node_repository.go` | `FindByToken` 扩展查 `prev_token` |
| `server/internal/repository/agent_install_token_repository.go` | 新增文件 |
| `server/internal/service/node_service.go` | 新增 `BatchCreate` / `RotateToken` |
| `server/internal/service/install_token_service.go` | 新增文件 |
| `server/internal/installscript/renderer.go` | 新增包 |
| `server/internal/http/install_handler.go` | 新增文件 |
| `server/internal/http/node_handler.go` | 新增 batch / rotate / install-token 端点 |
| `server/internal/http/agent_handler.go` | 新增 `Self` 端点 |
| `server/internal/http/router.go` | 注册新路由（公开分组 + JWT 分组） |
| `server/internal/initialize/gorm.go` | AutoMigrate 新表 |
| `deploy/agent-install.sh.tmpl` | 新增文件 |
| `deploy/agent-compose.yml.tmpl` | 新增文件 |
| `web/src/pages/nodes/NodesPage.tsx` | 操作列改造 |
| `web/src/pages/nodes/AgentInstallWizard.tsx` | 新增文件 |
| `web/src/pages/nodes/BatchCommandTable.tsx` | 新增文件 |
| `web/src/pages/nodes/wizard/*` | 新增 3 个子步骤 |
| `web/src/services/nodes.ts` | 新增 3 个函数 |
| ~~`web/src/locales/{zh-CN,en}/nodes.json`~~ | ~~新增 wizard.* 键~~（V1 使用硬编码中文，见 §6.5） |
| `docs-site/docs/features/multi-node.md` | 重写"Walkthrough"章节 |
| `docs-site/i18n/zh-CN/docusaurus-plugin-content-docs/current/features/multi-node.md` | 同步中文 |
