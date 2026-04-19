# 一键部署 Agent 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 BackupX 节点管理页引入 Komari 风格一键部署向导：勾选 mode/arch/version → 一次性 install token → 目标机 `curl | sudo sh` 完成安装。支持批量创建与 agent token 轮换。

**Architecture:** Master 端新增 `agent_install_tokens` 表 + 两组路由（JWT 保护的管理端 + 匿名的 `/install/:token` 公开端）。通过嵌入 shell/yaml 模板动态渲染安装脚本。前端替换旧 Modal 为三步向导 + 批量结果表。

**Tech Stack:**
- Backend: Go 1.25, Gin, GORM + SQLite, `go:embed` 模板
- Frontend: React 18 + TypeScript + Arco Design + Vite
- Testing: `go test`（Go），Vitest（前端 — 可选）

**Spec**: [docs/superpowers/specs/2026-04-19-one-click-agent-deploy-design.md](../specs/2026-04-19-one-click-agent-deploy-design.md)

**Issue**: [#43 Feature: 一键部署 Agent](https://github.com/Awuqing/BackupX/issues/43)

**项目约定（重要）**：按 `CLAUDE.md`，未经用户明确要求**不执行 git commit/push**。本计划中的 "Commit" 步骤遵循 TDD 纪律，执行时需用户确认后再运行。

**执行阶段**：
- Phase 1（任务 1-13）：后端 — 可独立合入
- Phase 2（任务 14-20）：前端 — 依赖 Phase 1 合入
- Phase 3（任务 21-22）：文档

---

## 文件结构

```
server/
├── internal/
│   ├── model/
│   │   ├── agent_install_token.go        [新增]
│   │   └── node.go                       [改：加 2 列]
│   ├── repository/
│   │   ├── agent_install_token_repository.go       [新增]
│   │   ├── agent_install_token_repository_test.go  [新增]
│   │   └── node_repository.go            [改：FindByToken 扩展]
│   ├── service/
│   │   ├── install_token_service.go      [新增]
│   │   ├── install_token_service_test.go [新增]
│   │   ├── node_service.go               [改：BatchCreate + RotateToken + Self]
│   │   ├── node_service_test.go          [新增：轮换+批量测试]
│   │   └── agent_service.go              [改：AuthenticatedNodeSummary]
│   ├── installscript/
│   │   ├── renderer.go                   [新增]
│   │   ├── renderer_test.go              [新增]
│   │   └── testdata/                     [新增 golden files]
│   ├── http/
│   │   ├── install_handler.go            [新增]
│   │   ├── node_handler.go               [改：Batch + InstallTokens + Rotate + Preview]
│   │   ├── agent_handler.go              [改：Self 端点]
│   │   ├── router.go                     [改：注册新路由]
│   │   └── install_flow_test.go          [新增：端到端集成]
│   ├── database/database.go              [改：AutoMigrate 新表]
│   └── app/app.go                        [改：wire install_token_service + GC]
├── deploy/
│   ├── agent-install.sh.tmpl             [新增]
│   └── agent-compose.yml.tmpl            [新增]
web/
└── src/
    ├── types/nodes.ts                    [改：新类型]
    ├── services/nodes.ts                 [改：新 API 函数]
    └── pages/nodes/
        ├── NodesPage.tsx                 [改：操作列 + 调 Wizard]
        ├── AgentInstallWizard.tsx        [新增]
        ├── BatchCommandTable.tsx         [新增]
        └── wizard/
            ├── Step1NodeName.tsx         [新增]
            ├── Step2DeployOptions.tsx    [新增]
            └── Step3CommandPreview.tsx   [新增]
docs-site/docs/features/multi-node.md                                       [改]
docs-site/i18n/zh-CN/docusaurus-plugin-content-docs/current/features/multi-node.md  [改]
```

---

# Phase 1 — 后端

## Task 1: 数据模型 `AgentInstallToken` 与 Node 列扩展

**Files:**
- Create: `server/internal/model/agent_install_token.go`
- Modify: `server/internal/model/node.go`
- Modify: `server/internal/database/database.go`

- [ ] **Step 1: 写 Node model 扩展**

Append to `server/internal/model/node.go`:

```go
// 附加字段（V1.7 多节点一键部署：支持 agent token 轮换）
//
// 轮换流程：rotate 时把旧 Token 复制到 PrevToken，并设置 PrevTokenExpires=now+24h。
// Agent 认证先查 Token，未命中则退回查 PrevToken（且未过期）。
```

（声明加字段，实际字段加到 Node struct 内）。修改 `Node` struct，在 `UpdatedAt` 之前插入：

```go
PrevToken        string     `gorm:"size:128;index" json:"-"`
PrevTokenExpires *time.Time `gorm:"column:prev_token_expires" json:"-"`
```

- [ ] **Step 2: 新建 AgentInstallToken 模型**

Create `server/internal/model/agent_install_token.go`:

```go
package model

import "time"

// AgentInstallToken 一次性安装令牌，用于 /install/:token 公开端点。
//
// 生命周期：创建 → 消费（ConsumedAt 非空即作废）→ 超过 ExpiresAt 后被 GC 硬删除。
type AgentInstallToken struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Token       string     `gorm:"size:64;uniqueIndex;not null" json:"token"`
	NodeID      uint       `gorm:"not null;index" json:"nodeId"`
	Mode        string     `gorm:"size:16;not null" json:"mode"`        // systemd|docker|foreground
	Arch        string     `gorm:"size:16;not null" json:"arch"`        // amd64|arm64|auto
	AgentVer    string     `gorm:"size:32;not null" json:"agentVersion"`
	DownloadSrc string     `gorm:"size:16;not null;default:'github'" json:"downloadSrc"`
	ExpiresAt   time.Time  `gorm:"not null;index" json:"expiresAt"`
	ConsumedAt  *time.Time `json:"consumedAt,omitempty"`
	CreatedByID uint       `gorm:"not null" json:"createdById"`
	CreatedAt   time.Time  `json:"createdAt"`
}

func (AgentInstallToken) TableName() string { return "agent_install_tokens" }

// 合法模式/架构常量
const (
	InstallModeSystemd    = "systemd"
	InstallModeDocker     = "docker"
	InstallModeForeground = "foreground"

	InstallArchAmd64 = "amd64"
	InstallArchArm64 = "arm64"
	InstallArchAuto  = "auto"

	InstallSourceGitHub  = "github"
	InstallSourceGhproxy = "ghproxy"
)
```

- [ ] **Step 3: AutoMigrate 新表**

Modify `server/internal/database/database.go` line 26 — 在 `AutoMigrate` 调用末尾加 `&model.AgentInstallToken{}`:

```go
if err := db.AutoMigrate(
    &model.User{}, &model.SystemConfig{}, &model.StorageTarget{}, &model.OAuthSession{},
    &model.BackupTask{}, &model.BackupRecord{}, &model.Notification{}, &model.Node{},
    &model.BackupTaskStorageTarget{}, &model.AuditLog{}, &model.AgentCommand{},
    &model.AgentInstallToken{},
); err != nil {
    return nil, fmt.Errorf("migrate schema: %w", err)
}
```

- [ ] **Step 4: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过，无错误

- [ ] **Step 5: Commit**

```bash
git add server/internal/model/agent_install_token.go server/internal/model/node.go server/internal/database/database.go
git commit -m "功能: 新增 AgentInstallToken 模型与 Node token 轮换字段"
```

---

## Task 2: NodeRepository 支持 prev_token 回退认证

**Files:**
- Modify: `server/internal/repository/node_repository.go:50-59`
- Create: `server/internal/repository/node_repository_test.go`（若已存在则追加用例）

- [ ] **Step 1: 写失败测试**

Create or append to `server/internal/repository/node_repository_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openTestNodeDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nodes.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestFindByTokenFallsBackToPrevToken(t *testing.T) {
	db := openTestNodeDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	future := time.Now().UTC().Add(24 * time.Hour)
	node := &model.Node{
		Name: "test", Token: "new-token",
		PrevToken: "old-token", PrevTokenExpires: &future,
	}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 新 token 能查到
	got, err := repo.FindByToken(ctx, "new-token")
	if err != nil || got == nil || got.ID != node.ID {
		t.Fatalf("new token lookup failed: err=%v got=%v", err, got)
	}

	// 旧 token 也能查到（未过期）
	got, err = repo.FindByToken(ctx, "old-token")
	if err != nil || got == nil || got.ID != node.ID {
		t.Fatalf("prev_token lookup failed: err=%v got=%v", err, got)
	}
}

func TestFindByTokenRejectsExpiredPrevToken(t *testing.T) {
	db := openTestNodeDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	past := time.Now().UTC().Add(-1 * time.Hour)
	node := &model.Node{
		Name: "test", Token: "new-token",
		PrevToken: "stale", PrevTokenExpires: &past,
	}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.FindByToken(ctx, "stale")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got != nil {
		t.Fatalf("expected stale prev_token rejected, got %v", got)
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `cd server && go test ./internal/repository/ -run TestFindByToken -v`
Expected: FAIL —— `prev_token lookup failed: got=<nil>` 或 stale 未被拒

- [ ] **Step 3: 实现 FindByToken 回退查询**

Modify `server/internal/repository/node_repository.go`, 替换 `FindByToken`:

```go
func (r *GormNodeRepository) FindByToken(ctx context.Context, token string) (*model.Node, error) {
	var item model.Node
	// 主 token 查询
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&item).Error
	if err == nil {
		return &item, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	// 回退：prev_token 且未过期
	now := time.Now().UTC()
	err = r.db.WithContext(ctx).
		Where("prev_token = ? AND prev_token_expires IS NOT NULL AND prev_token_expires > ?", token, now).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}
```

- [ ] **Step 4: 再跑测试验证通过**

Run: `cd server && go test ./internal/repository/ -run TestFindByToken -v`
Expected: PASS（两个测试）

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/node_repository.go server/internal/repository/node_repository_test.go
git commit -m "功能: NodeRepository.FindByToken 支持 prev_token 24h 过渡"
```

---

## Task 3: AgentInstallTokenRepository 实现与测试

**Files:**
- Create: `server/internal/repository/agent_install_token_repository.go`
- Create: `server/internal/repository/agent_install_token_repository_test.go`

- [ ] **Step 1: 写 Repository 接口与失败测试**

Create `server/internal/repository/agent_install_token_repository_test.go`:

```go
package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openTestInstallTokenDB(t *testing.T) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "install.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentInstallToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInstallTokenConsumeOnce(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	tok := &model.AgentInstallToken{
		Token: "abc", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(15 * time.Minute),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 首次消费成功
	got, err := repo.ConsumeByToken(ctx, "abc")
	if err != nil {
		t.Fatalf("consume err: %v", err)
	}
	if got == nil || got.ConsumedAt == nil {
		t.Fatalf("expected consumed token, got %+v", got)
	}

	// 第二次消费应返回 nil（已作废）
	got, err = repo.ConsumeByToken(ctx, "abc")
	if err != nil {
		t.Fatalf("second consume err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on second consume, got %+v", got)
	}
}

func TestInstallTokenConsumeExpired(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	tok := &model.AgentInstallToken{
		Token: "stale", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-time.Minute),
		CreatedByID: 1,
	}
	if err := repo.Create(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.ConsumeByToken(ctx, "stale")
	if err != nil {
		t.Fatalf("consume err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on expired, got %+v", got)
	}
}

func TestInstallTokenGC(t *testing.T) {
	db := openTestInstallTokenDB(t)
	repo := NewAgentInstallTokenRepository(db)
	ctx := context.Background()

	// 造一条 8 天前过期的
	old := &model.AgentInstallToken{
		Token: "old", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-8 * 24 * time.Hour),
		CreatedByID: 1,
	}
	_ = repo.Create(ctx, old)

	// 造一条今天过期的（不应被 GC）
	fresh := &model.AgentInstallToken{
		Token: "fresh", NodeID: 1, Mode: model.InstallModeSystemd,
		Arch: model.InstallArchAuto, AgentVer: "v1.7.0",
		DownloadSrc: model.InstallSourceGitHub,
		ExpiresAt:   time.Now().UTC().Add(-1 * time.Hour),
		CreatedByID: 1,
	}
	_ = repo.Create(ctx, fresh)

	n, err := repo.DeleteExpiredBefore(ctx, time.Now().UTC().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("gc err: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted, got %d", n)
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `cd server && go test ./internal/repository/ -run TestInstallToken -v`
Expected: FAIL（`undefined: NewAgentInstallTokenRepository`）

- [ ] **Step 3: 实现 Repository**

Create `server/internal/repository/agent_install_token_repository.go`:

```go
package repository

import (
	"context"
	"errors"
	"time"

	"backupx/server/internal/model"
	"gorm.io/gorm"
)

type AgentInstallTokenRepository interface {
	Create(ctx context.Context, t *model.AgentInstallToken) error
	FindByToken(ctx context.Context, token string) (*model.AgentInstallToken, error)
	// ConsumeByToken 原子地把 ConsumedAt 置为 now；若 token 不存在/已过期/已消费则返回 (nil, nil)。
	ConsumeByToken(ctx context.Context, token string) (*model.AgentInstallToken, error)
	// DeleteExpiredBefore 硬删除 ExpiresAt < threshold 的记录，返回删除行数。
	DeleteExpiredBefore(ctx context.Context, threshold time.Time) (int64, error)
	// CountCreatedSince 统计某 node 在 since 之后的生成次数（用于节点级限流）。
	CountCreatedSince(ctx context.Context, nodeID uint, since time.Time) (int64, error)
}

type GormAgentInstallTokenRepository struct {
	db *gorm.DB
}

func NewAgentInstallTokenRepository(db *gorm.DB) *GormAgentInstallTokenRepository {
	return &GormAgentInstallTokenRepository{db: db}
}

func (r *GormAgentInstallTokenRepository) Create(ctx context.Context, t *model.AgentInstallToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *GormAgentInstallTokenRepository) FindByToken(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	var item model.AgentInstallToken
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// ConsumeByToken 使用事务：SELECT → 校验 → UPDATE consumed_at。
// SQLite 不支持 SELECT FOR UPDATE，这里用 UPDATE ... WHERE consumed_at IS NULL AND expires_at > now
// 的条件更新 + RowsAffected 判断实现原子消费。
func (r *GormAgentInstallTokenRepository) ConsumeByToken(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	var consumed *model.AgentInstallToken
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&model.AgentInstallToken{}).
			Where("token = ? AND consumed_at IS NULL AND expires_at > ?", token, now).
			Update("consumed_at", &now)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		var item model.AgentInstallToken
		if err := tx.Where("token = ?", token).First(&item).Error; err != nil {
			return err
		}
		consumed = &item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return consumed, nil
}

func (r *GormAgentInstallTokenRepository) DeleteExpiredBefore(ctx context.Context, threshold time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("expires_at < ?", threshold).Delete(&model.AgentInstallToken{})
	return result.RowsAffected, result.Error
}

func (r *GormAgentInstallTokenRepository) CountCreatedSince(ctx context.Context, nodeID uint, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.AgentInstallToken{}).
		Where("node_id = ? AND created_at >= ?", nodeID, since).
		Count(&n).Error
	return n, err
}
```

- [ ] **Step 4: 再跑测试验证通过**

Run: `cd server && go test ./internal/repository/ -run TestInstallToken -v`
Expected: PASS（3 个测试）

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/agent_install_token_repository.go server/internal/repository/agent_install_token_repository_test.go
git commit -m "功能: 新增 AgentInstallToken 仓储与原子消费语义"
```

---

## Task 4: 安装脚本渲染器 `installscript` 包

**Files:**
- Create: `server/internal/installscript/renderer.go`
- Create: `server/internal/installscript/renderer_test.go`
- Create: `server/internal/installscript/testdata/systemd.golden.sh`
- Create: `server/internal/installscript/testdata/foreground.golden.sh`
- Create: `server/internal/installscript/testdata/docker.golden.sh`
- Create: `server/internal/installscript/testdata/compose.golden.yml`
- Create: `deploy/agent-install.sh.tmpl`
- Create: `deploy/agent-compose.yml.tmpl`

- [ ] **Step 1: 先建脚本模板文件**

Create `deploy/agent-install.sh.tmpl`:

```bash
#!/bin/sh
# BackupX Agent 一键安装脚本（由 Master 动态渲染）
# 模式: {{.Mode}} | 架构: {{.Arch}} | 版本: {{.AgentVersion}}
set -eu

MASTER_URL="{{.MasterURL}}"
AGENT_TOKEN="{{.AgentToken}}"
AGENT_VERSION="{{.AgentVersion}}"
DOWNLOAD_BASE="{{.DownloadBase}}"
INSTALL_PREFIX="{{.InstallPrefix}}"
ARCH="{{.Arch}}"

# 1. 前置检查
[ "$(id -u)" -eq 0 ] || { echo "请使用 root 或 sudo 执行" >&2; exit 1; }
command -v curl >/dev/null || command -v wget >/dev/null \
    || { echo "需要 curl 或 wget" >&2; exit 1; }
{{if eq .Mode "systemd"}}command -v systemctl >/dev/null || { echo "不支持非 systemd 系统" >&2; exit 1; }
{{end}}{{if eq .Mode "docker"}}command -v docker >/dev/null || { echo "需要先安装 docker" >&2; exit 1; }
{{end}}
# 2. 架构检测
if [ "$ARCH" = "auto" ]; then
    case "$(uname -m)" in
        x86_64|amd64)  ARCH=amd64 ;;
        aarch64|arm64) ARCH=arm64 ;;
        *) echo "不支持的架构: $(uname -m)" >&2; exit 1 ;;
    esac
fi

{{if ne .Mode "docker"}}
# 3. 下载二进制（systemd / foreground 模式）
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

# 4. 安装二进制 + 用户
echo "[2/4] 安装到 ${INSTALL_PREFIX}"
id backupx >/dev/null 2>&1 || useradd --system --home-dir "$INSTALL_PREFIX" --shell /usr/sbin/nologin backupx
install -d -o backupx -g backupx "$INSTALL_PREFIX" /var/lib/backupx-agent
install -m 0755 "$TMPDIR/backupx-${AGENT_VERSION}-linux-${ARCH}/backupx" "$INSTALL_PREFIX/backupx"
{{end}}

{{if eq .Mode "systemd"}}
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
{{end}}

{{if eq .Mode "foreground"}}
# 5. 前台运行
echo "[3/3] 前台启动 agent（Ctrl+C 退出）"
export BACKUPX_AGENT_MASTER="${MASTER_URL}"
export BACKUPX_AGENT_TOKEN="${AGENT_TOKEN}"
exec "${INSTALL_PREFIX}/backupx" agent --temp-dir /var/lib/backupx-agent
{{end}}

{{if eq .Mode "docker"}}
# Docker 模式：直接用镜像启动容器
echo "[1/2] 拉取镜像 awuqing/backupx:${AGENT_VERSION}"
docker pull "awuqing/backupx:${AGENT_VERSION}"
echo "[2/2] 启动容器 backupx-agent"
docker rm -f backupx-agent >/dev/null 2>&1 || true
docker run -d --name backupx-agent --restart=unless-stopped \
    -e "BACKUPX_AGENT_MASTER=${MASTER_URL}" \
    -e "BACKUPX_AGENT_TOKEN=${AGENT_TOKEN}" \
    -v /var/lib/backupx-agent:/tmp/backupx-agent \
    "awuqing/backupx:${AGENT_VERSION}" agent
echo "✓ 容器已启动"
{{end}}
```

Create `deploy/agent-compose.yml.tmpl`:

```yaml
# BackupX Agent docker-compose 片段
# 生成于 {{.MasterURL}} · 节点 ID {{.NodeID}}
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

- [ ] **Step 2: 写失败测试（渲染器 + golden file 对比）**

Create `server/internal/installscript/renderer_test.go`:

```go
package installscript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backupx/server/internal/model"
)

var testCtx = Context{
	MasterURL:     "https://master.example.com",
	AgentToken:    "test-token-hex",
	AgentVersion:  "v1.7.0",
	Mode:          model.InstallModeSystemd,
	Arch:          model.InstallArchAuto,
	DownloadBase:  "https://github.com/Awuqing/BackupX/releases/download",
	InstallPrefix: "/opt/backupx-agent",
	NodeID:        42,
}

func TestRenderScriptSystemd(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "systemd.golden.sh"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	want := string(wantBytes)
	if got != want {
		t.Errorf("systemd script mismatch:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestRenderScriptForeground(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeForeground
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "exec \"${INSTALL_PREFIX}/backupx\" agent") {
		t.Errorf("foreground script missing exec line:\n%s", got)
	}
	if strings.Contains(got, "systemctl") {
		t.Errorf("foreground script should not reference systemctl:\n%s", got)
	}
}

func TestRenderScriptDocker(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	got, err := RenderScript(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "docker run") {
		t.Errorf("docker script missing `docker run`:\n%s", got)
	}
	if !strings.Contains(got, "awuqing/backupx:v1.7.0") {
		t.Errorf("docker script missing image tag:\n%s", got)
	}
}

func TestRenderComposeYaml(t *testing.T) {
	ctx := testCtx
	ctx.Mode = model.InstallModeDocker
	got, err := RenderComposeYaml(ctx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	if !strings.Contains(got, "image: awuqing/backupx:v1.7.0") {
		t.Errorf("compose missing image:\n%s", got)
	}
	if !strings.Contains(got, "BACKUPX_AGENT_TOKEN: \"test-token-hex\"") {
		t.Errorf("compose missing token env:\n%s", got)
	}
}

func TestDownloadBaseMapping(t *testing.T) {
	cases := map[string]string{
		model.InstallSourceGitHub:  "https://github.com/Awuqing/BackupX/releases/download",
		model.InstallSourceGhproxy: "https://ghproxy.com/https://github.com/Awuqing/BackupX/releases/download",
	}
	for src, want := range cases {
		got := DownloadBaseFor(src)
		if got != want {
			t.Errorf("src=%s want=%s got=%s", src, want, got)
		}
	}
}
```

- [ ] **Step 3: 跑测试验证失败**

Run: `cd server && go test ./internal/installscript/ -v`
Expected: FAIL（包不存在）

- [ ] **Step 4: 实现 renderer.go**

Create `server/internal/installscript/renderer.go`:

```go
// Package installscript 负责把一次性安装令牌 + 节点配置渲染为可执行 shell 脚本或 docker-compose YAML。
//
// 模板文件通过 go:embed 嵌入二进制，避免运行时依赖外部资源。
package installscript

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"backupx/server/internal/model"
)

//go:embed ../../../deploy/agent-install.sh.tmpl
var installScriptTmpl string

//go:embed ../../../deploy/agent-compose.yml.tmpl
var composeYamlTmpl string

// Context 是模板渲染输入。
type Context struct {
	MasterURL     string
	AgentToken    string
	AgentVersion  string
	Mode          string // systemd|docker|foreground
	Arch          string // amd64|arm64|auto
	DownloadBase  string
	InstallPrefix string
	NodeID        uint
}

// DownloadBaseFor 将下载源枚举转换为具体 URL 前缀。
func DownloadBaseFor(src string) string {
	switch src {
	case model.InstallSourceGhproxy:
		return "https://ghproxy.com/https://github.com/Awuqing/BackupX/releases/download"
	default:
		return "https://github.com/Awuqing/BackupX/releases/download"
	}
}

// RenderScript 渲染目标机安装脚本。
func RenderScript(ctx Context) (string, error) {
	ctx = withDefaults(ctx)
	tmpl, err := template.New("install").Parse(installScriptTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

// RenderComposeYaml 渲染 docker-compose.yml 片段。
func RenderComposeYaml(ctx Context) (string, error) {
	ctx = withDefaults(ctx)
	tmpl, err := template.New("compose").Parse(composeYamlTmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

func withDefaults(ctx Context) Context {
	if ctx.InstallPrefix == "" {
		ctx.InstallPrefix = "/opt/backupx-agent"
	}
	if ctx.DownloadBase == "" {
		ctx.DownloadBase = DownloadBaseFor(model.InstallSourceGitHub)
	}
	return ctx
}
```

- [ ] **Step 5: 生成 golden files**

Run to capture current output:

```bash
cd server && go test ./internal/installscript/ -run TestRenderScriptSystemd -v 2>&1 | head -80
```

If systemd golden file missing, generate it by running renderer once manually. For initial creation, write a small temporary helper or simply create `server/internal/installscript/testdata/systemd.golden.sh` with the exact expected rendered output matching `testCtx`. 

Approach: comment out the test's file comparison temporarily, capture stdout via a one-off `main_test.go` helper, or write the content directly based on your template expansion.

The golden file content should be the full rendered template for `testCtx` (systemd mode, auto arch). Produce it by:

```bash
cd server && cat <<'EOF' > /tmp/gen.go
package main
import (
    "fmt"
    "backupx/server/internal/installscript"
    "backupx/server/internal/model"
)
func main() {
    out, _ := installscript.RenderScript(installscript.Context{
        MasterURL: "https://master.example.com",
        AgentToken: "test-token-hex",
        AgentVersion: "v1.7.0",
        Mode: model.InstallModeSystemd,
        Arch: model.InstallArchAuto,
        DownloadBase: "https://github.com/Awuqing/BackupX/releases/download",
        InstallPrefix: "/opt/backupx-agent",
        NodeID: 42,
    })
    fmt.Print(out)
}
EOF
go run /tmp/gen.go > server/internal/installscript/testdata/systemd.golden.sh
rm /tmp/gen.go
```

Alternatively, skip the exact golden match for systemd (too brittle) and only keep the substring-based tests in `TestRenderScriptForeground` / `TestRenderScriptDocker` / `TestRenderComposeYaml`. Modify `TestRenderScriptSystemd` to use substring assertions instead:

```go
func TestRenderScriptSystemd(t *testing.T) {
	got, err := RenderScript(testCtx)
	if err != nil {
		t.Fatalf("render err: %v", err)
	}
	mustContain := []string{
		"BACKUPX_AGENT_MASTER=${MASTER_URL}",
		"Environment=\"BACKUPX_AGENT_TOKEN=${AGENT_TOKEN}\"",
		"systemctl daemon-reload",
		"systemctl enable --now backupx-agent",
		"X-Agent-Token: ${AGENT_TOKEN}",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("systemd script missing %q", s)
		}
	}
	mustNotContain := []string{"docker run", "exec \"${INSTALL_PREFIX}"}
	for _, s := range mustNotContain {
		if strings.Contains(got, s) {
			t.Errorf("systemd script unexpectedly contains %q", s)
		}
	}
}
```

**Decision**：用 substring 方式（删除 golden file 需求），避免模板微调牵一发动全身。相应删除：testdata 目录中的 `.golden.sh` 文件不再创建。

- [ ] **Step 6: 跑测试验证通过**

Run: `cd server && go test ./internal/installscript/ -v`
Expected: PASS（5 个测试）

- [ ] **Step 7: Commit**

```bash
git add server/internal/installscript/ deploy/agent-install.sh.tmpl deploy/agent-compose.yml.tmpl
git commit -m "功能: 新增 installscript 包，渲染 systemd/docker/foreground 安装脚本"
```

---

## Task 5: InstallTokenService（业务层 + 限流）

**Files:**
- Create: `server/internal/service/install_token_service.go`
- Create: `server/internal/service/install_token_service_test.go`

- [ ] **Step 1: 写失败测试**

Create `server/internal/service/install_token_service_test.go`:

```go
package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openInstallTokenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "it.db")),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.AgentInstallToken{}, &model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestInstallTokenServiceCreateAndConsume(t *testing.T) {
	db := openInstallTokenTestDB(t)
	repo := repository.NewAgentInstallTokenRepository(db)
	nodeRepo := repository.NewNodeRepository(db)

	// 准备 node
	node := &model.Node{Name: "n1", Token: "agent-token"}
	_ = nodeRepo.Create(context.Background(), node)

	svc := NewInstallTokenService(repo, nodeRepo)
	created, err := svc.Create(context.Background(), InstallTokenInput{
		NodeID:       node.ID,
		Mode:         model.InstallModeSystemd,
		Arch:         model.InstallArchAuto,
		AgentVersion: "v1.7.0",
		DownloadSrc:  model.InstallSourceGitHub,
		TTLSeconds:   900,
		CreatedByID:  1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Token == "" || created.ExpiresAt.Before(time.Now().UTC()) {
		t.Fatalf("invalid token: %+v", created)
	}

	consumed, err := svc.Consume(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if consumed == nil || consumed.NodeID != node.ID {
		t.Fatalf("expected consumed token for node, got %+v", consumed)
	}

	// 二次消费应返回 nil
	again, err := svc.Consume(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("second consume err: %v", err)
	}
	if again != nil {
		t.Fatalf("expected nil on second consume")
	}
}

func TestInstallTokenServiceValidatesInput(t *testing.T) {
	db := openInstallTokenTestDB(t)
	svc := NewInstallTokenService(
		repository.NewAgentInstallTokenRepository(db),
		repository.NewNodeRepository(db),
	)
	cases := []struct {
		name string
		in   InstallTokenInput
	}{
		{"bad mode", InstallTokenInput{NodeID: 1, Mode: "xxx", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"bad arch", InstallTokenInput{NodeID: 1, Mode: "systemd", Arch: "risc", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
		{"bad ttl low", InstallTokenInput{NodeID: 1, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 10, CreatedByID: 1}},
		{"bad ttl high", InstallTokenInput{NodeID: 1, Mode: "systemd", Arch: "auto", AgentVersion: "v1", DownloadSrc: "github", TTLSeconds: 999999, CreatedByID: 1}},
		{"missing version", InstallTokenInput{NodeID: 1, Mode: "systemd", Arch: "auto", AgentVersion: "", DownloadSrc: "github", TTLSeconds: 300, CreatedByID: 1}},
	}
	for _, tc := range cases {
		if _, err := svc.Create(context.Background(), tc.in); err == nil {
			t.Errorf("%s: expected validation error", tc.name)
		}
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `cd server && go test ./internal/service/ -run TestInstallTokenService -v`
Expected: FAIL（`undefined: NewInstallTokenService`）

- [ ] **Step 3: 实现 Service**

Create `server/internal/service/install_token_service.go`:

```go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// InstallTokenService 负责一次性安装令牌的创建/消费/校验。
type InstallTokenService struct {
	repo     repository.AgentInstallTokenRepository
	nodeRepo repository.NodeRepository
}

func NewInstallTokenService(repo repository.AgentInstallTokenRepository, nodeRepo repository.NodeRepository) *InstallTokenService {
	return &InstallTokenService{repo: repo, nodeRepo: nodeRepo}
}

// InstallTokenInput 生成安装令牌的输入。
type InstallTokenInput struct {
	NodeID       uint
	Mode         string
	Arch         string
	AgentVersion string
	DownloadSrc  string
	TTLSeconds   int
	CreatedByID  uint
}

// InstallTokenOutput 生成结果。
type InstallTokenOutput struct {
	Token     string
	ExpiresAt time.Time
	Node      *model.Node
	Record    *model.AgentInstallToken
}

// RateLimitWindow 每节点限流窗口：60s 内最多 5 次。
const (
	InstallTokenMinTTL       = 300    // 5 分钟
	InstallTokenMaxTTL       = 86400  // 24 小时
	InstallTokenRateWindow   = 60 * time.Second
	InstallTokenRatePerWin   = 5
)

var (
	validModes   = map[string]bool{model.InstallModeSystemd: true, model.InstallModeDocker: true, model.InstallModeForeground: true}
	validArches  = map[string]bool{model.InstallArchAmd64: true, model.InstallArchArm64: true, model.InstallArchAuto: true}
	validSources = map[string]bool{model.InstallSourceGitHub: true, model.InstallSourceGhproxy: true}
)

// Create 生成一次性安装令牌。
func (s *InstallTokenService) Create(ctx context.Context, in InstallTokenInput) (*InstallTokenOutput, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	node, err := s.nodeRepo.FindByID(ctx, in.NodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(404, "NODE_NOT_FOUND", "节点不存在", nil)
	}

	// 限流
	since := time.Now().UTC().Add(-InstallTokenRateWindow)
	count, err := s.repo.CountCreatedSince(ctx, in.NodeID, since)
	if err != nil {
		return nil, err
	}
	if count >= InstallTokenRatePerWin {
		return nil, apperror.New(429, "INSTALL_TOKEN_RATE_LIMITED",
			fmt.Sprintf("每 %d 秒最多生成 %d 次", int(InstallTokenRateWindow.Seconds()), InstallTokenRatePerWin), nil)
	}

	token, err := generateInstallToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	expiresAt := time.Now().UTC().Add(time.Duration(in.TTLSeconds) * time.Second)
	record := &model.AgentInstallToken{
		Token:       token,
		NodeID:      in.NodeID,
		Mode:        in.Mode,
		Arch:        in.Arch,
		AgentVer:    in.AgentVersion,
		DownloadSrc: in.DownloadSrc,
		ExpiresAt:   expiresAt,
		CreatedByID: in.CreatedByID,
	}
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}
	return &InstallTokenOutput{Token: token, ExpiresAt: expiresAt, Node: node, Record: record}, nil
}

// Consume 原子消费令牌。未命中/已过期/已消费均返回 (nil, nil)。
func (s *InstallTokenService) Consume(ctx context.Context, token string) (*ConsumedInstallToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	record, err := s.repo.ConsumeByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, nil
	}
	node, err := s.nodeRepo.FindByID(ctx, record.NodeID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, apperror.New(404, "NODE_NOT_FOUND", "节点已被删除", nil)
	}
	return &ConsumedInstallToken{
		Record: record,
		Node:   node,
	}, nil
}

// ConsumedInstallToken 是消费成功后返回给 handler 的组合体。
type ConsumedInstallToken struct {
	Record *model.AgentInstallToken
	Node   *model.Node
}

// StartGC 启动后台 GC，按 interval 扫描 `expires_at < now-7d` 的记录并硬删除。
// 返回的 stop 函数会取消定时器。
func (s *InstallTokenService) StartGC(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = s.repo.DeleteExpiredBefore(ctx, time.Now().UTC().Add(-7*24*time.Hour))
			}
		}
	}()
}

func (s *InstallTokenService) validate(in InstallTokenInput) error {
	if in.NodeID == 0 {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "nodeId 必填", nil)
	}
	if !validModes[in.Mode] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "mode 非法", nil)
	}
	if !validArches[in.Arch] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "arch 非法", nil)
	}
	if !validSources[in.DownloadSrc] {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "downloadSrc 非法", nil)
	}
	if strings.TrimSpace(in.AgentVersion) == "" {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID", "agentVersion 必填", nil)
	}
	if in.TTLSeconds < InstallTokenMinTTL || in.TTLSeconds > InstallTokenMaxTTL {
		return apperror.BadRequest("INSTALL_TOKEN_INVALID",
			fmt.Sprintf("ttlSeconds 需在 %d-%d", InstallTokenMinTTL, InstallTokenMaxTTL), nil)
	}
	return nil
}

func generateInstallToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
```

- [ ] **Step 4: 再跑测试验证通过**

Run: `cd server && go test ./internal/service/ -run TestInstallTokenService -v`
Expected: PASS（2 个测试）

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/install_token_service.go server/internal/service/install_token_service_test.go
git commit -m "功能: 新增 InstallTokenService 含输入校验、限流、GC"
```

---

## Task 6: NodeService 扩展 —— BatchCreate + RotateToken + SelfStatus

**Files:**
- Modify: `server/internal/service/node_service.go`
- Create: `server/internal/service/node_service_test.go`

- [ ] **Step 1: 写失败测试**

Create `server/internal/service/node_service_test.go`:

```go
package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openNodeServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ns.db")),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.Node{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestBatchCreateNodes(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	ctx := context.Background()

	items, err := svc.BatchCreate(ctx, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3, got %d", len(items))
	}
	for _, it := range items {
		if it.ID == 0 || it.Name == "" {
			t.Errorf("invalid item %+v", it)
		}
	}
}

func TestBatchCreateRejectsDuplicates(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	ctx := context.Background()

	// 先建一个 "a"
	_, _ = svc.Create(ctx, NodeCreateInput{Name: "a"})

	_, err := svc.BatchCreate(ctx, []string{"a", "b"})
	if err == nil {
		t.Fatalf("expected error on duplicate with existing")
	}

	// 批次内重复
	_, err = svc.BatchCreate(ctx, []string{"x", "x"})
	if err == nil {
		t.Fatalf("expected error on intra-batch duplicate")
	}
}

func TestBatchCreateLimitEnforced(t *testing.T) {
	db := openNodeServiceDB(t)
	svc := NewNodeService(repository.NewNodeRepository(db), "test")
	ctx := context.Background()

	names := make([]string, 51)
	for i := range names {
		names[i] = "n" + string(rune('a'+i%26))
	}
	_, err := svc.BatchCreate(ctx, names)
	if err == nil {
		t.Fatalf("expected error on >50 batch")
	}
}

func TestRotateToken(t *testing.T) {
	db := openNodeServiceDB(t)
	repo := repository.NewNodeRepository(db)
	svc := NewNodeService(repo, "test")
	ctx := context.Background()

	oldTok, err := svc.Create(ctx, NodeCreateInput{Name: "rot"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 查新建的 node
	var node model.Node
	db.First(&node, "name = ?", "rot")

	newTok, err := svc.RotateToken(ctx, node.ID)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newTok == oldTok || len(newTok) != 64 {
		t.Fatalf("invalid new token: %s", newTok)
	}

	// 旧 token 仍可查（24h 内）
	found, _ := repo.FindByToken(ctx, oldTok)
	if found == nil || found.ID != node.ID {
		t.Fatalf("old token should still work via prev_token fallback")
	}
	// 新 token 也可查
	found2, _ := repo.FindByToken(ctx, newTok)
	if found2 == nil || found2.ID != node.ID {
		t.Fatalf("new token should work")
	}

	// prev_token_expires 应设置为约 24h 后
	db.First(&node, node.ID)
	if node.PrevTokenExpires == nil {
		t.Fatalf("prev_token_expires not set")
	}
	diff := node.PrevTokenExpires.Sub(time.Now().UTC())
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Fatalf("prev_token_expires out of range: %v", diff)
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `cd server && go test ./internal/service/ -run "TestBatchCreate|TestRotateToken" -v`
Expected: FAIL（方法未定义）

- [ ] **Step 3: 实现 BatchCreate + RotateToken**

Append to `server/internal/service/node_service.go` (在文件末尾，`generateToken` 之前):

```go
// BatchCreate 批量创建远程节点，事务内执行。
// 校验：1-50 项、每项 1-128 字符、批次内去重、与已有节点名去重。
// 返回 NodeCreateResult 列表（不含 token，前端应再调 install-tokens 接口）。
func (s *NodeService) BatchCreate(ctx context.Context, names []string) ([]NodeCreateResult, error) {
	cleaned, err := validateBatchNames(names)
	if err != nil {
		return nil, err
	}
	// 与数据库已有名称去重
	existing, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	existingSet := make(map[string]bool, len(existing))
	for _, n := range existing {
		existingSet[n.Name] = true
	}
	for _, name := range cleaned {
		if existingSet[name] {
			return nil, apperror.BadRequest("NODE_DUPLICATE_NAME",
				fmt.Sprintf("节点名「%s」已存在", name), nil)
		}
	}

	results := make([]NodeCreateResult, 0, len(cleaned))
	for _, name := range cleaned {
		tok, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate token: %w", err)
		}
		node := &model.Node{
			Name:     name,
			Token:    tok,
			Status:   model.NodeStatusOffline,
			IsLocal:  false,
			LastSeen: time.Now().UTC(),
		}
		if err := s.repo.Create(ctx, node); err != nil {
			return nil, err
		}
		results = append(results, NodeCreateResult{ID: node.ID, Name: node.Name})
	}
	return results, nil
}

// NodeCreateResult 批量创建结果。注意：不暴露 agent token，token 获取走 install-token 流程。
type NodeCreateResult struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

// RotateToken 轮换指定节点的 agent token。
// 旧 token 复制到 prev_token，保留 24h 过渡。
func (s *NodeService) RotateToken(ctx context.Context, id uint) (string, error) {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	if node == nil {
		return "", apperror.New(404, "NODE_NOT_FOUND", "节点不存在", nil)
	}
	if node.IsLocal {
		return "", apperror.BadRequest("NODE_ROTATE_LOCAL", "本机节点无需轮换", nil)
	}
	newTok, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generate: %w", err)
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	node.PrevToken = node.Token
	node.PrevTokenExpires = &expires
	node.Token = newTok
	if err := s.repo.Update(ctx, node); err != nil {
		return "", err
	}
	return newTok, nil
}

// validateBatchNames 校验并去重批次内名称。
func validateBatchNames(names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, apperror.BadRequest("NODE_BATCH_EMPTY", "节点名列表不能为空", nil)
	}
	if len(names) > 50 {
		return nil, apperror.BadRequest("NODE_BATCH_TOO_MANY", "单次最多创建 50 个节点", nil)
	}
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len(name) > 128 {
			return nil, apperror.BadRequest("NODE_NAME_TOO_LONG",
				fmt.Sprintf("节点名「%s」超过 128 字符", name), nil)
		}
		if seen[name] {
			return nil, apperror.BadRequest("NODE_DUPLICATE_NAME",
				fmt.Sprintf("批次内重复节点名「%s」", name), nil)
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, apperror.BadRequest("NODE_BATCH_EMPTY", "去除空白后列表为空", nil)
	}
	return out, nil
}
```

- [ ] **Step 4: 再跑测试验证通过**

Run: `cd server && go test ./internal/service/ -run "TestBatchCreate|TestRotateToken" -v`
Expected: PASS（4 个测试）

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/node_service.go server/internal/service/node_service_test.go
git commit -m "功能: NodeService 新增 BatchCreate 与 RotateToken"
```

---

## Task 7: AgentService 新增 SelfStatus 方法

**Files:**
- Modify: `server/internal/service/agent_service.go`

- [ ] **Step 1: 写失败测试**

Append to `server/internal/service/node_service_test.go`（或新建 `agent_service_test.go`）：

```go
func TestAgentSelfStatus(t *testing.T) {
	db := openNodeServiceDB(t)
	if err := db.AutoMigrate(&model.BackupTask{}, &model.BackupRecord{}, &model.StorageTarget{}, &model.AgentCommand{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	nodeRepo := repository.NewNodeRepository(db)
	node := &model.Node{Name: "x", Token: "abc", Status: model.NodeStatusOnline}
	_ = nodeRepo.Create(context.Background(), node)

	// cipher / 其他 repos 可传 nil 或简单桩（SelfStatus 只用 nodeRepo）
	svc := NewAgentService(nodeRepo, nil, nil, nil, nil, nil)
	got, err := svc.SelfStatus(context.Background(), node)
	if err != nil {
		t.Fatalf("self: %v", err)
	}
	if got.ID != node.ID || got.Name != "x" || got.Status != "online" {
		t.Fatalf("bad self status: %+v", got)
	}
}
```

- [ ] **Step 2: 跑测试验证失败**

Run: `cd server && go test ./internal/service/ -run TestAgentSelfStatus -v`
Expected: FAIL（`undefined: SelfStatus`）

- [ ] **Step 3: 实现 SelfStatus**

Append to `server/internal/service/agent_service.go`:

```go
// AgentSelfStatus 是 /api/v1/agent/self 端点返回给 Agent 的轻量状态摘要。
type AgentSelfStatus struct {
	ID       uint      `json:"id"`
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"lastSeen"`
}

// SelfStatus 返回 Agent token 所属节点的当前状态，供安装脚本末尾探活。
func (s *AgentService) SelfStatus(ctx context.Context, node *model.Node) (*AgentSelfStatus, error) {
	if node == nil {
		return nil, apperror.Unauthorized("NODE_INVALID_TOKEN", "节点不存在", nil)
	}
	return &AgentSelfStatus{
		ID:       node.ID,
		Name:     node.Name,
		Status:   node.Status,
		LastSeen: node.LastSeen,
	}, nil
}
```

- [ ] **Step 4: 再跑测试验证通过**

Run: `cd server && go test ./internal/service/ -run TestAgentSelfStatus -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/agent_service.go server/internal/service/node_service_test.go
git commit -m "功能: AgentService 新增 SelfStatus 用于安装脚本探活"
```

---

## Task 8: HTTP 处理器 —— install_handler.go（公开端点）

**Files:**
- Create: `server/internal/http/install_handler.go`

- [ ] **Step 1: 实现 install_handler.go**

Create `server/internal/http/install_handler.go`:

```go
package http

import (
	stdhttp "net/http"
	"strings"
	"sync"
	"time"

	"backupx/server/internal/installscript"
	"backupx/server/internal/model"
	"backupx/server/internal/service"
	"github.com/gin-gonic/gin"
)

// InstallHandler 公开路由（不走 JWT），实现 /install/:token 与 /install/:token/compose.yml。
type InstallHandler struct {
	tokenService *service.InstallTokenService
	auditService *service.AuditService
	externalURL  string // 可选：MasterURL 硬编码覆盖
	limiter      *ipLimiter
}

func NewInstallHandler(tokenService *service.InstallTokenService, auditService *service.AuditService, externalURL string) *InstallHandler {
	return &InstallHandler{
		tokenService: tokenService,
		auditService: auditService,
		externalURL:  externalURL,
		limiter:      newIPLimiter(20, time.Minute),
	}
}

// Script 消费 token 并返回 shell 脚本；Mode 按 token 存储决定。
func (h *InstallHandler) Script(c *gin.Context) {
	if !h.limiter.allow(c.ClientIP()) {
		c.String(stdhttp.StatusTooManyRequests, "请求过于频繁，请稍后再试\n")
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	consumed, err := h.tokenService.Consume(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if consumed == nil {
		c.String(stdhttp.StatusGone, "install token 不存在、已过期或已消费\n")
		return
	}
	// 审计
	if h.auditService != nil {
		h.auditService.Record(service.AuditEntry{
			Username:   "",
			Category:   "install_token",
			Action:     "consume",
			TargetType: "node",
			TargetID:   uintToStr(consumed.Node.ID),
			TargetName: consumed.Node.Name,
			Detail:     "install token 消费（script）",
			ClientIP:   c.ClientIP(),
		})
	}
	masterURL := h.resolveMasterURL(c)
	script, err := installscript.RenderScript(installscript.Context{
		MasterURL:     masterURL,
		AgentToken:    consumed.Node.Token,
		AgentVersion:  consumed.Record.AgentVer,
		Mode:          consumed.Record.Mode,
		Arch:          consumed.Record.Arch,
		DownloadBase:  installscript.DownloadBaseFor(consumed.Record.DownloadSrc),
		InstallPrefix: "/opt/backupx-agent",
		NodeID:        consumed.Node.ID,
	})
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "render error\n")
		return
	}
	c.Data(stdhttp.StatusOK, "text/x-shellscript; charset=utf-8", []byte(script))
}

// Compose 消费 token 并返回 docker-compose YAML，仅 Mode=docker 有效。
func (h *InstallHandler) Compose(c *gin.Context) {
	if !h.limiter.allow(c.ClientIP()) {
		c.String(stdhttp.StatusTooManyRequests, "请求过于频繁\n")
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	// 先不消费，窥视 Mode
	record, err := h.tokenService.Peek(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if record == nil {
		c.String(stdhttp.StatusGone, "install token 不存在或已作废\n")
		return
	}
	if record.Mode != model.InstallModeDocker {
		c.String(stdhttp.StatusBadRequest, "该 install token 的模式不是 docker\n")
		return
	}
	consumed, err := h.tokenService.Consume(c.Request.Context(), token)
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "server error\n")
		return
	}
	if consumed == nil {
		c.String(stdhttp.StatusGone, "install token 已过期或已消费\n")
		return
	}
	if h.auditService != nil {
		h.auditService.Record(service.AuditEntry{
			Category:   "install_token",
			Action:     "consume",
			TargetType: "node",
			TargetID:   uintToStr(consumed.Node.ID),
			TargetName: consumed.Node.Name,
			Detail:     "install token 消费（compose）",
			ClientIP:   c.ClientIP(),
		})
	}
	masterURL := h.resolveMasterURL(c)
	yaml, err := installscript.RenderComposeYaml(installscript.Context{
		MasterURL:    masterURL,
		AgentToken:   consumed.Node.Token,
		AgentVersion: consumed.Record.AgentVer,
		Mode:         model.InstallModeDocker,
		NodeID:       consumed.Node.ID,
	})
	if err != nil {
		c.String(stdhttp.StatusInternalServerError, "render error\n")
		return
	}
	c.Data(stdhttp.StatusOK, "text/yaml; charset=utf-8", []byte(yaml))
}

// resolveMasterURL 按优先级：系统配置 > X-Forwarded-* > Request.Host。
func (h *InstallHandler) resolveMasterURL(c *gin.Context) string {
	if strings.TrimSpace(h.externalURL) != "" {
		return strings.TrimRight(h.externalURL, "/")
	}
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}

// ipLimiter 简单内存滑动窗口限流。
type ipLimiter struct {
	mu      sync.Mutex
	events  map[string][]time.Time
	limit   int
	window  time.Duration
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{events: make(map[string][]time.Time), limit: limit, window: window}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	keep := l.events[ip][:0]
	for _, t := range l.events[ip] {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.limit {
		l.events[ip] = keep
		return false
	}
	l.events[ip] = append(keep, now)
	return true
}

func uintToStr(u uint) string {
	return strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(
		"0000000000"+itoa(u), "0000000000"), "_", ""), "0")
}

// itoa 避免依赖 strconv（包已经导入 gin，无所谓，但保留小助手）
func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}
```

**注意**：`tokenService.Peek` 方法需要在 `install_token_service.go` 补充。

- [ ] **Step 2: 给 InstallTokenService 补 Peek 方法**

Append to `server/internal/service/install_token_service.go`:

```go
// Peek 只读查询（不消费），用于 compose 端点先检查 Mode。
func (s *InstallTokenService) Peek(ctx context.Context, token string) (*model.AgentInstallToken, error) {
	if strings.TrimSpace(token) == "" {
		return nil, nil
	}
	return s.repo.FindByToken(ctx, token)
}
```

- [ ] **Step 3: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过（此任务暂无独立测试，集成测试在 Task 11）

- [ ] **Step 4: Commit**

```bash
git add server/internal/http/install_handler.go server/internal/service/install_token_service.go
git commit -m "功能: 新增公开的 install_handler 渲染安装脚本与 compose.yml"
```

---

## Task 9: HTTP 处理器 —— NodeHandler 扩展

**Files:**
- Modify: `server/internal/http/node_handler.go`

- [ ] **Step 1: 补全 handler 方法**

Append to `server/internal/http/node_handler.go`:

```go
// 批量创建节点。
func (h *NodeHandler) BatchCreate(c *gin.Context) {
	var input struct {
		Names []string `json:"names" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	results, err := h.service.BatchCreate(c.Request.Context(), input.Names)
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "node", "batch_create", "node", "",
		fmt.Sprintf("%d", len(results)), fmt.Sprintf("批量创建 %d 个节点", len(results)))
	response.Success(c, results)
}

// 轮换 agent token。
func (h *NodeHandler) RotateToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	tok, err := h.service.RotateToken(c.Request.Context(), uint(id))
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "node", "rotate_token", "node", fmt.Sprintf("%d", id), "",
		fmt.Sprintf("轮换节点 Token (ID: %d)", id))
	response.Success(c, gin.H{"newToken": tok})
}
```

以及 install-tokens 与 preview 端点。由于它们依赖 `InstallTokenService` 与 `installscript` 包，在 NodeHandler struct 上增加这些依赖：

在 `NodeHandler` struct 与 `NewNodeHandler` 中插入字段与参数：

```go
type NodeHandler struct {
	service           *service.NodeService
	auditService      *service.AuditService
	installTokenSvc   *service.InstallTokenService
	externalURL       string
}

func NewNodeHandler(
	nodeService *service.NodeService,
	auditService *service.AuditService,
	installTokenSvc *service.InstallTokenService,
	externalURL string,
) *NodeHandler {
	return &NodeHandler{
		service:         nodeService,
		auditService:    auditService,
		installTokenSvc: installTokenSvc,
		externalURL:     externalURL,
	}
}
```

追加 handler：

```go
// 生成一次性安装令牌。
func (h *NodeHandler) CreateInstallToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Error(c, err)
		return
	}
	var input struct {
		Mode         string `json:"mode"`
		Arch         string `json:"arch"`
		AgentVersion string `json:"agentVersion"`
		DownloadSrc  string `json:"downloadSrc"`
		TTLSeconds   int    `json:"ttlSeconds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": err.Error()})
		return
	}
	// 默认值
	if input.Mode == "" {
		input.Mode = "systemd"
	}
	if input.Arch == "" {
		input.Arch = "auto"
	}
	if input.DownloadSrc == "" {
		input.DownloadSrc = "github"
	}
	if input.TTLSeconds == 0 {
		input.TTLSeconds = 900
	}

	createdBy := uint(0)
	// 审计来源
	if subj, ok := c.Get(contextUserSubjectKey); ok {
		_ = subj // username 仅用于 audit；CreatedByID 可暂设 0 或留 TODO 如无 userID 映射
	}

	out, err := h.installTokenSvc.Create(c.Request.Context(), service.InstallTokenInput{
		NodeID:       uint(id),
		Mode:         input.Mode,
		Arch:         input.Arch,
		AgentVersion: input.AgentVersion,
		DownloadSrc:  input.DownloadSrc,
		TTLSeconds:   input.TTLSeconds,
		CreatedByID:  createdBy,
	})
	if err != nil {
		response.Error(c, err)
		return
	}
	recordAudit(c, h.auditService, "install_token", "create", "node",
		fmt.Sprintf("%d", id), out.Node.Name,
		fmt.Sprintf("生成 %s/%s install token TTL=%ds", input.Mode, input.Arch, input.TTLSeconds))

	masterURL := resolveMasterURL(c, h.externalURL)
	body := gin.H{
		"installToken": out.Token,
		"expiresAt":    out.ExpiresAt,
		"url":          masterURL + "/install/" + out.Token,
	}
	if input.Mode == "docker" {
		body["composeUrl"] = masterURL + "/install/" + out.Token + "/compose.yml"
	} else {
		body["composeUrl"] = ""
	}
	response.Success(c, body)
}

// 预览脚本（占位 token，不消费）。
func (h *NodeHandler) PreviewScript(c *gin.Context) {
	mode := c.DefaultQuery("mode", "systemd")
	arch := c.DefaultQuery("arch", "auto")
	ver := c.Query("agentVersion")
	if ver == "" {
		c.JSON(stdhttp.StatusBadRequest, gin.H{"code": "INVALID_INPUT", "message": "agentVersion required"})
		return
	}
	src := c.DefaultQuery("downloadSrc", "github")
	ctx := installscript.Context{
		MasterURL:     resolveMasterURL(c, h.externalURL),
		AgentToken:    "<AGENT_TOKEN>",
		AgentVersion:  ver,
		Mode:          mode,
		Arch:          arch,
		DownloadBase:  installscript.DownloadBaseFor(src),
		InstallPrefix: "/opt/backupx-agent",
	}
	script, err := installscript.RenderScript(ctx)
	if err != nil {
		response.Error(c, err)
		return
	}
	c.Data(stdhttp.StatusOK, "text/x-shellscript; charset=utf-8", []byte(script))
}

// resolveMasterURL 在 install_handler.go 的内部实现与此重复；提炼成 package-level helper。
func resolveMasterURL(c *gin.Context, externalURL string) string {
	if strings.TrimSpace(externalURL) != "" {
		return strings.TrimRight(externalURL, "/")
	}
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}
```

**DRY 调整**：同时删除 `install_handler.go` 里的 `resolveMasterURL` 方法，改用包级 `resolveMasterURL` 函数：

Modify `server/internal/http/install_handler.go`:

```go
// 将原来的 h.resolveMasterURL(c) 调用替换为 resolveMasterURL(c, h.externalURL)
// 并删除 InstallHandler 上的 resolveMasterURL 方法
```

然后补 import：在 `node_handler.go` 顶部确保导入：

```go
import (
	"fmt"
	stdhttp "net/http"
	"strconv"
	"strings"

	"backupx/server/internal/http/installscript"  // 若包路径不对按实际修正
	"backupx/server/internal/installscript"
	"backupx/server/internal/service"
	"backupx/server/pkg/response"

	"github.com/gin-gonic/gin"
)
```

（删除重复的 installscript import，保留第二个实际路径）

- [ ] **Step 2: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add server/internal/http/node_handler.go server/internal/http/install_handler.go
git commit -m "功能: NodeHandler 新增批量创建/轮换/install-token/预览端点"
```

---

## Task 10: AgentHandler 新增 Self 端点

**Files:**
- Modify: `server/internal/http/agent_handler.go`

- [ ] **Step 1: 实现 Self handler**

Append to `server/internal/http/agent_handler.go`:

```go
// Self 返回当前 Agent token 所属节点的状态，供安装脚本探活。
func (h *AgentHandler) Self(c *gin.Context) {
	node, err := h.agentService.AuthenticatedNode(c.Request.Context(), extractToken(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	status, err := h.agentService.SelfStatus(c.Request.Context(), node)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, status)
}
```

- [ ] **Step 2: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add server/internal/http/agent_handler.go
git commit -m "功能: AgentHandler 新增 Self 端点"
```

---

## Task 11: Router 注册新路由与依赖 wire

**Files:**
- Modify: `server/internal/http/router.go`
- Modify: `server/internal/app/app.go`

- [ ] **Step 1: RouterDependencies 新增字段**

Modify `server/internal/http/router.go`, 在 `RouterDependencies` struct 中追加：

```go
InstallTokenService *service.InstallTokenService
MasterExternalURL   string
```

- [ ] **Step 2: 修改 handler 实例化，注入依赖**

Modify `server/internal/http/router.go` `NewRouter` 函数：

```go
// 原：nodeHandler := NewNodeHandler(deps.NodeService, deps.AuditService)
nodeHandler := NewNodeHandler(deps.NodeService, deps.AuditService, deps.InstallTokenService, deps.MasterExternalURL)
```

注册新路由（在 `nodes` 路由组内）：

```go
nodes.POST("/batch", nodeHandler.BatchCreate)
nodes.POST("/:id/install-tokens", nodeHandler.CreateInstallToken)
nodes.POST("/:id/rotate-token", nodeHandler.RotateToken)
nodes.GET("/:id/install-script-preview", nodeHandler.PreviewScript)
```

在 Agent 路由组内添加：

```go
agent.GET("/self", agentHandler.Self)
```

在 `api := engine.Group("/api")` **之外**、在 `engine.NoRoute` 之前，注册公开的 /install 路由：

```go
if deps.InstallTokenService != nil {
	installHandler := NewInstallHandler(deps.InstallTokenService, deps.AuditService, deps.MasterExternalURL)
	engine.GET("/install/:token", installHandler.Script)
	engine.GET("/install/:token/compose.yml", installHandler.Compose)
}
```

- [ ] **Step 3: app.go 新增 wire**

Modify `server/internal/app/app.go`，在 Agent service 初始化之后追加：

```go
// 一键部署：install token service + 后台 GC
installTokenRepo := repository.NewAgentInstallTokenRepository(db)
installTokenService := service.NewInstallTokenService(installTokenRepo, nodeRepo)
installTokenService.StartGC(ctx, time.Hour)
```

并把它加入 `RouterDependencies`：

```go
router := aphttp.NewRouter(aphttp.RouterDependencies{
	// ... 已有字段
	InstallTokenService: installTokenService,
	MasterExternalURL:   cfg.Server.ExternalURL, // 如配置无此字段，传 ""
})
```

**注意**：如 `cfg.Server` 无 `ExternalURL`，直接传 `""`；后续可在 `config.ServerConfig` 增该字段（非本任务范围）。

- [ ] **Step 4: 编译验证**

Run: `cd server && go build ./...`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add server/internal/http/router.go server/internal/app/app.go
git commit -m "功能: 注册一键部署新路由并 wire InstallTokenService"
```

---

## Task 12: 集成测试 —— 端到端流程

**Files:**
- Create: `server/internal/http/install_flow_test.go`

- [ ] **Step 1: 写集成测试**

Create `server/internal/http/install_flow_test.go`:

```go
package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
	"backupx/server/internal/service"
	"context"
)

func setupInstallFlowRouter(t *testing.T) (*http.Handler, string) {
	t.Helper()
	tempDir := t.TempDir()
	cfg := config.Config{
		Server:   config.ServerConfig{Host: "127.0.0.1", Port: 8340, Mode: "test"},
		Database: config.DatabaseConfig{Path: filepath.Join(tempDir, "backupx.db")},
		Security: config.SecurityConfig{JWTExpire: "24h"},
		Log:      config.LogConfig{Level: "error"},
	}
	log, _ := logger.New(cfg.Log)
	db, _ := database.Open(cfg.Database, log)

	userRepo := repository.NewUserRepository(db)
	systemConfigRepo := repository.NewSystemConfigRepository(db)
	resolved, _ := service.ResolveSecurity(context.Background(), cfg.Security, systemConfigRepo)
	jwtMgr := security.NewJWTManager(resolved.JWTSecret, time.Hour)
	authSvc := service.NewAuthService(userRepo, systemConfigRepo, jwtMgr, security.NewLoginRateLimiter(5, time.Minute))

	nodeRepo := repository.NewNodeRepository(db)
	nodeSvc := service.NewNodeService(nodeRepo, "test")
	_ = nodeSvc.EnsureLocalNode(context.Background())

	installTokenRepo := repository.NewAgentInstallTokenRepository(db)
	installTokenSvc := service.NewInstallTokenService(installTokenRepo, nodeRepo)

	auditLogRepo := repository.NewAuditLogRepository(db)
	auditSvc := service.NewAuditService(auditLogRepo)

	router := NewRouter(RouterDependencies{
		Config:              cfg,
		Version:             "test",
		Logger:              log,
		AuthService:         authSvc,
		NodeService:         nodeSvc,
		InstallTokenService: installTokenSvc,
		AuditService:        auditSvc,
		JWTManager:          jwtMgr,
		UserRepository:      userRepo,
		SystemConfigRepo:    systemConfigRepo,
	})

	// setup 管理员并登录取 JWT
	setupBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "password-123", "displayName": "admin"})
	setupReq := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewBuffer(setupBody))
	setupReq.Header.Set("Content-Type", "application/json")
	setupRec := httptest.NewRecorder()
	router.ServeHTTP(setupRec, setupReq)
	var setupResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(setupRec.Body.Bytes(), &setupResp)

	var h http.Handler = router
	return &h, setupResp.Data.Token
}

func TestOneClickInstallFlow(t *testing.T) {
	handlerPtr, jwt := setupInstallFlowRouter(t)
	router := *handlerPtr

	// 1. 批量创建
	batchBody, _ := json.Marshal(map[string][]string{"names": {"prod-a", "prod-b"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	if batchRec.Code != 200 {
		t.Fatalf("batch create failed: %d %s", batchRec.Code, batchRec.Body.String())
	}
	var batchResp struct {
		Data []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	if len(batchResp.Data) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(batchResp.Data))
	}
	nodeID := batchResp.Data[0].ID

	// 2. 生成 install token
	genBody, _ := json.Marshal(map[string]any{
		"mode":         "systemd",
		"arch":         "auto",
		"agentVersion": "v1.7.0",
		"downloadSrc":  "github",
		"ttlSeconds":   900,
	})
	genReq := httptest.NewRequest(http.MethodPost,
		"/api/nodes/"+itoa(nodeID)+"/install-tokens", bytes.NewBuffer(genBody))
	genReq.Header.Set("Content-Type", "application/json")
	genReq.Header.Set("Authorization", "Bearer "+jwt)
	genRec := httptest.NewRecorder()
	router.ServeHTTP(genRec, genReq)
	if genRec.Code != 200 {
		t.Fatalf("install-tokens failed: %d %s", genRec.Code, genRec.Body.String())
	}
	var genResp struct {
		Data struct {
			InstallToken string `json:"installToken"`
			URL          string `json:"url"`
		} `json:"data"`
	}
	_ = json.Unmarshal(genRec.Body.Bytes(), &genResp)
	if genResp.Data.InstallToken == "" {
		t.Fatalf("missing installToken")
	}

	// 3. 公开端点消费
	scriptReq := httptest.NewRequest(http.MethodGet, "/install/"+genResp.Data.InstallToken, nil)
	scriptRec := httptest.NewRecorder()
	router.ServeHTTP(scriptRec, scriptReq)
	if scriptRec.Code != 200 {
		t.Fatalf("script fetch failed: %d %s", scriptRec.Code, scriptRec.Body.String())
	}
	if !strings.Contains(scriptRec.Body.String(), "systemctl enable --now backupx-agent") {
		t.Fatalf("script missing systemctl enable:\n%s", scriptRec.Body.String())
	}

	// 4. 再次消费应 410
	scriptRec2 := httptest.NewRecorder()
	router.ServeHTTP(scriptRec2, httptest.NewRequest(http.MethodGet, "/install/"+genResp.Data.InstallToken, nil))
	if scriptRec2.Code != http.StatusGone {
		t.Fatalf("second consume should be 410, got %d", scriptRec2.Code)
	}
}

func TestInstallTokenRateLimit(t *testing.T) {
	handlerPtr, jwt := setupInstallFlowRouter(t)
	router := *handlerPtr
	// 创建节点
	batchBody, _ := json.Marshal(map[string][]string{"names": {"rl-test"}})
	batchReq := httptest.NewRequest(http.MethodPost, "/api/nodes/batch", bytes.NewBuffer(batchBody))
	batchReq.Header.Set("Content-Type", "application/json")
	batchReq.Header.Set("Authorization", "Bearer "+jwt)
	batchRec := httptest.NewRecorder()
	router.ServeHTTP(batchRec, batchReq)
	var batchResp struct {
		Data []struct{ ID uint `json:"id"` } `json:"data"`
	}
	_ = json.Unmarshal(batchRec.Body.Bytes(), &batchResp)
	nodeID := batchResp.Data[0].ID

	body, _ := json.Marshal(map[string]any{
		"mode": "systemd", "arch": "auto", "agentVersion": "v1", "downloadSrc": "github", "ttlSeconds": 300,
	})
	// 前 5 次成功
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/nodes/"+itoa(nodeID)+"/install-tokens", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+jwt)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != 200 {
			t.Fatalf("iter %d expected 200, got %d: %s", i, rec.Code, rec.Body.String())
		}
	}
	// 第 6 次限流
	req := httptest.NewRequest(http.MethodPost, "/api/nodes/"+itoa(nodeID)+"/install-tokens", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
}

func itoa(u uint) string {
	if u == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	return string(buf[i:])
}
```

- [ ] **Step 2: 跑集成测试**

Run: `cd server && go test ./internal/http/ -run "TestOneClickInstallFlow|TestInstallTokenRateLimit" -v`
Expected: PASS（2 个测试）

若 429 测试失败（install_token_service 限流是 HTTP 层返回非 200，但 `response.Error` 映射的 status code 取决于 apperror 构造），检查 `InstallTokenService.Create` 里的 `apperror.New(429, ...)`，确保 `response.Error` 会把 apperror.Status 正确写为 429。

- [ ] **Step 3: Commit**

```bash
git add server/internal/http/install_flow_test.go
git commit -m "测试: 一键部署端到端流程集成测试"
```

---

## Task 13: 回归现有测试 + 合入 Phase 1

**Files:** 无新增

- [ ] **Step 1: 全量测试**

Run: `cd server && go test ./...`
Expected: 所有包 PASS

- [ ] **Step 2: 静态检查**

Run: `cd server && go vet ./...`
Expected: 无 warning

- [ ] **Step 3: （可选）本地启动验证**

```bash
cd server && go run ./cmd/backupx
# 另开一个终端：登录、新建节点、生成 install token、本机 curl 测试（不实际执行脚本，仅验证端点返回）
curl http://localhost:8340/install/<token>
```

- [ ] **Step 4: 本阶段整合后确认**

Phase 1 后端完成。与用户确认是否合入 main 分支，或继续 Phase 2。

---

# Phase 2 — 前端

## Task 14: TypeScript 类型与 API 函数扩展

**Files:**
- Modify: `web/src/types/nodes.ts`
- Modify: `web/src/services/nodes.ts`

- [ ] **Step 1: 扩展类型**

Append to `web/src/types/nodes.ts`:

```typescript
export type InstallMode = 'systemd' | 'docker' | 'foreground'
export type InstallArch = 'amd64' | 'arm64' | 'auto'
export type InstallSource = 'github' | 'ghproxy'

export interface BatchCreateResult {
  id: number
  name: string
}

export interface InstallTokenInput {
  mode: InstallMode
  arch: InstallArch
  agentVersion: string
  downloadSrc: InstallSource
  ttlSeconds: number
}

export interface InstallTokenResult {
  installToken: string
  expiresAt: string
  url: string
  composeUrl: string
}
```

- [ ] **Step 2: 新增 API 函数**

Append to `web/src/services/nodes.ts`:

```typescript
import type {
  NodeSummary, DirEntry,
  BatchCreateResult, InstallTokenInput, InstallTokenResult,
} from '../types/nodes'

export async function batchCreateNodes(names: string[]) {
  const response = await http.post<ApiEnvelope<BatchCreateResult[]>>('/nodes/batch', { names })
  return unwrapApiEnvelope(response.data)
}

export async function createInstallToken(nodeId: number, input: InstallTokenInput) {
  const response = await http.post<ApiEnvelope<InstallTokenResult>>(
    `/nodes/${nodeId}/install-tokens`, input,
  )
  return unwrapApiEnvelope(response.data)
}

export async function rotateNodeToken(nodeId: number) {
  const response = await http.post<ApiEnvelope<{ newToken: string }>>(
    `/nodes/${nodeId}/rotate-token`,
  )
  return unwrapApiEnvelope(response.data)
}

export async function fetchScriptPreview(
  nodeId: number,
  params: { mode: string; arch: string; agentVersion: string; downloadSrc: string },
) {
  const response = await http.get<string>(`/nodes/${nodeId}/install-script-preview`, {
    params,
    responseType: 'text',
  })
  return response.data
}
```

- [ ] **Step 3: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功（若 tsc 严格模式未通过，据 error 补 import）

- [ ] **Step 4: Commit**

```bash
git add web/src/types/nodes.ts web/src/services/nodes.ts
git commit -m "功能: 前端新增一键部署 API 类型与函数"
```

---

## Task 15: Wizard Step 1 —— 节点信息输入

**Files:**
- Create: `web/src/pages/nodes/wizard/Step1NodeName.tsx`

- [ ] **Step 1: 实现 Step1 组件**

Create `web/src/pages/nodes/wizard/Step1NodeName.tsx`:

```tsx
import React from 'react'
import { Radio, Input, Typography } from '@arco-design/web-react'

const { Text } = Typography
const TextArea = Input.TextArea

export type Mode = 'single' | 'batch'

interface Props {
  mode: Mode
  onModeChange: (m: Mode) => void
  singleName: string
  onSingleNameChange: (v: string) => void
  batchText: string
  onBatchTextChange: (v: string) => void
}

export function Step1NodeName({
  mode, onModeChange, singleName, onSingleNameChange, batchText, onBatchTextChange,
}: Props) {
  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Radio.Group
          type="button"
          value={mode}
          onChange={(v) => onModeChange(v as Mode)}
          options={[
            { label: '单节点', value: 'single' },
            { label: '批量创建', value: 'batch' },
          ]}
        />
      </div>
      {mode === 'single' ? (
        <div>
          <Text bold style={{ marginBottom: 6, display: 'block' }}>节点名称</Text>
          <Input
            placeholder="如：prod-db-01"
            value={singleName}
            onChange={onSingleNameChange}
            maxLength={128}
          />
        </div>
      ) : (
        <div>
          <Text bold style={{ marginBottom: 6, display: 'block' }}>节点名称（每行一个，最多 50 个）</Text>
          <TextArea
            rows={8}
            placeholder={'prod-db-01\nprod-db-02\nprod-web-01'}
            value={batchText}
            onChange={onBatchTextChange}
            style={{ fontFamily: 'monospace', fontSize: 13 }}
          />
          <Text type="secondary" style={{ fontSize: 12, marginTop: 4, display: 'block' }}>
            空行自动忽略；重名会在提交时报错
          </Text>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/nodes/wizard/Step1NodeName.tsx
git commit -m "功能: Wizard Step1 节点信息输入组件"
```

---

## Task 16: Wizard Step 2 —— 部署参数

**Files:**
- Create: `web/src/pages/nodes/wizard/Step2DeployOptions.tsx`

- [ ] **Step 1: 实现 Step2 组件**

Create `web/src/pages/nodes/wizard/Step2DeployOptions.tsx`:

```tsx
import React from 'react'
import { Form, Radio, Select, Typography } from '@arco-design/web-react'
import type { InstallMode, InstallArch, InstallSource } from '../../../types/nodes'

const { Text } = Typography

export interface DeployOptions {
  mode: InstallMode
  arch: InstallArch
  agentVersion: string
  downloadSrc: InstallSource
  ttlSeconds: number
}

interface Props {
  masterVersion: string
  value: DeployOptions
  onChange: (v: DeployOptions) => void
}

export function Step2DeployOptions({ masterVersion, value, onChange }: Props) {
  const update = (patch: Partial<DeployOptions>) => onChange({ ...value, ...patch })
  return (
    <Form layout="vertical" size="default">
      <Form.Item label="安装模式">
        <Radio.Group
          type="button"
          value={value.mode}
          onChange={(v) => update({ mode: v })}
          options={[
            { label: 'systemd（推荐）', value: 'systemd' },
            { label: 'Docker', value: 'docker' },
            { label: '前台运行（调试）', value: 'foreground' },
          ]}
        />
      </Form.Item>

      <Form.Item label="架构">
        <Select
          value={value.arch}
          onChange={(v) => update({ arch: v })}
          options={[
            { label: '自动检测（uname -m）', value: 'auto' },
            { label: 'amd64 (x86_64)', value: 'amd64' },
            { label: 'arm64 (aarch64)', value: 'arm64' },
          ]}
        />
      </Form.Item>

      <Form.Item label="Agent 版本">
        <Select
          value={value.agentVersion}
          onChange={(v) => update({ agentVersion: v })}
          options={[
            { label: `${masterVersion}（跟随 Master，推荐）`, value: masterVersion },
            // 更多历史版本可后续接入 /api/system/releases
          ]}
        />
      </Form.Item>

      <Form.Item label="安装命令有效期">
        <Select
          value={value.ttlSeconds}
          onChange={(v) => update({ ttlSeconds: v })}
          options={[
            { label: '5 分钟', value: 300 },
            { label: '15 分钟（推荐）', value: 900 },
            { label: '1 小时', value: 3600 },
            { label: '24 小时', value: 86400 },
          ]}
        />
      </Form.Item>

      <Form.Item label="二进制下载源" extra={<Text type="secondary">国内服务器选 ghproxy 镜像加速</Text>}>
        <Radio.Group
          type="button"
          value={value.downloadSrc}
          onChange={(v) => update({ downloadSrc: v })}
          options={[
            { label: 'GitHub 直连', value: 'github' },
            { label: 'ghproxy 镜像', value: 'ghproxy' },
          ]}
        />
      </Form.Item>
    </Form>
  )
}
```

- [ ] **Step 2: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/nodes/wizard/Step2DeployOptions.tsx
git commit -m "功能: Wizard Step2 部署参数表单"
```

---

## Task 17: Wizard Step 3 —— 命令预览与倒计时

**Files:**
- Create: `web/src/pages/nodes/wizard/Step3CommandPreview.tsx`
- Create: `web/src/pages/nodes/BatchCommandTable.tsx`

- [ ] **Step 1: 实现 Step3 单节点视图**

Create `web/src/pages/nodes/wizard/Step3CommandPreview.tsx`:

```tsx
import React, { useEffect, useState } from 'react'
import { Typography, Button, Space, Collapse, Spin, Message, Tag } from '@arco-design/web-react'
import { IconCopy, IconRefresh } from '@arco-design/web-react/icon'
import { fetchScriptPreview } from '../../../services/nodes'
import type { InstallTokenResult, InstallMode } from '../../../types/nodes'

const { Text } = Typography

interface Props {
  nodeId: number
  nodeName: string
  token: InstallTokenResult
  mode: InstallMode
  previewParams: { mode: string; arch: string; agentVersion: string; downloadSrc: string }
  onRegenerate: () => void
}

export function Step3CommandPreview({ nodeId, nodeName, token, mode, previewParams, onRegenerate }: Props) {
  const [remaining, setRemaining] = useState(0)
  const [preview, setPreview] = useState<string>('')
  const [loadingPreview, setLoadingPreview] = useState(false)

  useEffect(() => {
    const expires = new Date(token.expiresAt).getTime()
    const tick = () => setRemaining(Math.max(0, Math.floor((expires - Date.now()) / 1000)))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [token.expiresAt])

  const expired = remaining === 0
  const command = `curl -fsSL ${token.url} | sudo sh`
  const dockerComposeCmd = mode === 'docker'
    ? `curl -fsSL ${token.composeUrl} -o docker-compose.yml && docker-compose up -d`
    : null

  const copy = async (s: string) => {
    await navigator.clipboard.writeText(s)
    Message.success('已复制')
  }

  const loadPreview = async () => {
    setLoadingPreview(true)
    try {
      const text = await fetchScriptPreview(nodeId, previewParams)
      setPreview(text)
    } catch {
      Message.error('预览加载失败')
    } finally {
      setLoadingPreview(false)
    }
  }

  return (
    <div>
      <Space style={{ marginBottom: 12 }}>
        <Text bold>节点：</Text>
        <Tag>{nodeName}</Tag>
        <Tag color={expired ? 'gray' : 'green'}>
          {expired ? '已过期' : `有效期 ${Math.floor(remaining / 60)}:${String(remaining % 60).padStart(2, '0')}`}
        </Tag>
      </Space>

      <div style={{ background: 'var(--color-fill-2)', padding: '12px 14px', borderRadius: 6, marginBottom: 12 }}>
        <Text style={{
          fontFamily: 'monospace', fontSize: 13, wordBreak: 'break-all',
          opacity: expired ? 0.4 : 1, userSelect: 'all',
        }}>
          {command}
        </Text>
        <div style={{ marginTop: 8 }}>
          <Space>
            <Button size="small" icon={<IconCopy />} disabled={expired} onClick={() => copy(command)}>复制</Button>
            {expired && <Button size="small" type="primary" icon={<IconRefresh />} onClick={onRegenerate}>重新生成</Button>}
          </Space>
        </div>
      </div>

      {dockerComposeCmd && (
        <div style={{ background: 'var(--color-fill-2)', padding: '12px 14px', borderRadius: 6, marginBottom: 12 }}>
          <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 4 }}>
            或使用 docker-compose：
          </Text>
          <Text style={{ fontFamily: 'monospace', fontSize: 13, wordBreak: 'break-all', opacity: expired ? 0.4 : 1 }}>
            {dockerComposeCmd}
          </Text>
          <div style={{ marginTop: 8 }}>
            <Button size="small" icon={<IconCopy />} disabled={expired} onClick={() => copy(dockerComposeCmd)}>复制</Button>
          </div>
        </div>
      )}

      <Text type="secondary" style={{ fontSize: 12, display: 'block', marginBottom: 8 }}>
        命令仅显示一次，复制后请尽快在目标机执行。token 一经消费立即作废。
      </Text>

      <Collapse bordered={false}>
        <Collapse.Item name="preview" header="展开脚本预览"
          onActive={() => { if (!preview) loadPreview() }}>
          {loadingPreview ? <Spin /> : (
            <pre style={{
              background: 'var(--color-fill-2)', padding: 12, borderRadius: 4,
              fontSize: 12, maxHeight: 400, overflow: 'auto', whiteSpace: 'pre-wrap',
            }}>{preview}</pre>
          )}
        </Collapse.Item>
      </Collapse>
    </div>
  )
}
```

- [ ] **Step 2: 实现 BatchCommandTable**

Create `web/src/pages/nodes/BatchCommandTable.tsx`:

```tsx
import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Message, Typography } from '@arco-design/web-react'
import { IconCopy, IconDownload } from '@arco-design/web-react/icon'

const { Text } = Typography

export interface BatchCommandRow {
  nodeId: number
  nodeName: string
  command: string
  expiresAt: string
}

interface Props {
  rows: BatchCommandRow[]
}

export function BatchCommandTable({ rows }: Props) {
  const [remaining, setRemaining] = useState<Record<number, number>>({})

  useEffect(() => {
    const tick = () => {
      const next: Record<number, number> = {}
      rows.forEach((r) => {
        const exp = new Date(r.expiresAt).getTime()
        next[r.nodeId] = Math.max(0, Math.floor((exp - Date.now()) / 1000))
      })
      setRemaining(next)
    }
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [rows])

  const copy = async (s: string) => {
    await navigator.clipboard.writeText(s)
    Message.success('已复制')
  }

  const exportAll = () => {
    const content = [
      '#!/bin/sh',
      '# BackupX Agent 批量部署脚本',
      '# 使用方法：在目标机逐个执行下面对应节点命令',
      '',
      ...rows.map((r) => `# --- ${r.nodeName} ---\n${r.command}`),
    ].join('\n\n')
    const blob = new Blob([content], { type: 'text/x-shellscript' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `backupx-batch-install-${new Date().toISOString().slice(0, 10)}.sh`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div>
      <Table
        size="small"
        pagination={false}
        columns={[
          { title: '节点', dataIndex: 'nodeName', width: 140 },
          {
            title: '安装命令',
            dataIndex: 'command',
            render: (cmd: string, row: BatchCommandRow) => {
              const left = remaining[row.nodeId] ?? 0
              return (
                <Text style={{
                  fontFamily: 'monospace', fontSize: 12, wordBreak: 'break-all',
                  opacity: left === 0 ? 0.4 : 1,
                }}>
                  {cmd}
                </Text>
              )
            },
          },
          {
            title: '剩余', dataIndex: 'expiresAt', width: 90,
            render: (_v, row: BatchCommandRow) => {
              const left = remaining[row.nodeId] ?? 0
              return <Text type={left === 0 ? 'secondary' : 'primary'} style={{ fontSize: 12 }}>
                {left === 0 ? '已过期' : `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`}
              </Text>
            },
          },
          {
            title: '操作', width: 80,
            render: (_v, row: BatchCommandRow) => (
              <Button size="small" icon={<IconCopy />} onClick={() => copy(row.command)}
                disabled={(remaining[row.nodeId] ?? 0) === 0}>复制</Button>
            ),
          },
        ]}
        data={rows}
        rowKey="nodeId"
      />
      <div style={{ marginTop: 12, textAlign: 'right' }}>
        <Space>
          <Button icon={<IconDownload />} onClick={exportAll}>导出 .sh</Button>
        </Space>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/nodes/wizard/Step3CommandPreview.tsx web/src/pages/nodes/BatchCommandTable.tsx
git commit -m "功能: Wizard Step3 命令预览 + BatchCommandTable 批量表"
```

---

## Task 18: AgentInstallWizard 主容器

**Files:**
- Create: `web/src/pages/nodes/AgentInstallWizard.tsx`

- [ ] **Step 1: 实现 Wizard 主体**

Create `web/src/pages/nodes/AgentInstallWizard.tsx`:

```tsx
import React, { useState } from 'react'
import { Modal, Steps, Button, Space, Message, Spin } from '@arco-design/web-react'
import { Step1NodeName, type Mode } from './wizard/Step1NodeName'
import { Step2DeployOptions, type DeployOptions } from './wizard/Step2DeployOptions'
import { Step3CommandPreview } from './wizard/Step3CommandPreview'
import { BatchCommandTable, type BatchCommandRow } from './BatchCommandTable'
import { batchCreateNodes, createInstallToken, createNode } from '../../services/nodes'
import type { InstallTokenResult } from '../../types/nodes'

const Step = Steps.Step

interface Props {
  visible: boolean
  onClose: () => void
  onSuccess: () => void
  masterVersion: string
  // 当从节点列表直接点"生成安装命令"时传入，跳过 Step1
  fixedNode?: { id: number; name: string }
}

export function AgentInstallWizard({ visible, onClose, onSuccess, masterVersion, fixedNode }: Props) {
  const [step, setStep] = useState(fixedNode ? 1 : 0)
  const [mode, setMode] = useState<Mode>('single')
  const [singleName, setSingleName] = useState('')
  const [batchText, setBatchText] = useState('')

  const [deploy, setDeploy] = useState<DeployOptions>({
    mode: 'systemd',
    arch: 'auto',
    agentVersion: masterVersion,
    downloadSrc: 'github',
    ttlSeconds: 900,
  })

  const [singleToken, setSingleToken] = useState<InstallTokenResult | null>(null)
  const [singleNodeInfo, setSingleNodeInfo] = useState<{ id: number; name: string } | null>(null)
  const [batchRows, setBatchRows] = useState<BatchCommandRow[]>([])
  const [submitting, setSubmitting] = useState(false)

  const reset = () => {
    setStep(fixedNode ? 1 : 0)
    setMode('single')
    setSingleName('')
    setBatchText('')
    setSingleToken(null)
    setSingleNodeInfo(null)
    setBatchRows([])
  }

  const handleClose = () => {
    reset()
    onClose()
  }

  const parseBatchNames = (): string[] =>
    batchText.split('\n').map((s) => s.trim()).filter(Boolean)

  const handleNextFromStep1 = () => {
    if (mode === 'single') {
      if (!singleName.trim()) {
        Message.warning('请输入节点名称')
        return
      }
    } else {
      const names = parseBatchNames()
      if (names.length === 0) {
        Message.warning('请至少输入一个节点名称')
        return
      }
      if (names.length > 50) {
        Message.warning('单次最多创建 50 个节点')
        return
      }
    }
    setStep(1)
  }

  const handleGenerate = async () => {
    setSubmitting(true)
    try {
      if (fixedNode) {
        // 仅生成 token
        const tok = await createInstallToken(fixedNode.id, {
          mode: deploy.mode, arch: deploy.arch,
          agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
          ttlSeconds: deploy.ttlSeconds,
        })
        setSingleNodeInfo(fixedNode)
        setSingleToken(tok)
      } else if (mode === 'single') {
        const n = await createNode(singleName.trim())
        // createNode 原本返回 token；此处我们不再用它；我们需要 node id
        // 为拿 node id，可由后端 createNode 响应里补 id，或改调 batchCreate 单条
        // 使用 batchCreateNodes 走同一路径更一致：
        const created = await batchCreateNodes([singleName.trim()])
        const one = created[0]
        const tok = await createInstallToken(one.id, {
          mode: deploy.mode, arch: deploy.arch,
          agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
          ttlSeconds: deploy.ttlSeconds,
        })
        setSingleNodeInfo({ id: one.id, name: one.name })
        setSingleToken(tok)
        // 抛弃 createNode 的重复创建：改为只调 batchCreateNodes
        void n
      } else {
        const names = parseBatchNames()
        const created = await batchCreateNodes(names)
        const rows: BatchCommandRow[] = []
        for (const c of created) {
          const tok = await createInstallToken(c.id, {
            mode: deploy.mode, arch: deploy.arch,
            agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
            ttlSeconds: deploy.ttlSeconds,
          })
          rows.push({
            nodeId: c.id, nodeName: c.name,
            command: `curl -fsSL ${tok.url} | sudo sh`,
            expiresAt: tok.expiresAt,
          })
        }
        setBatchRows(rows)
      }
      setStep(2)
      onSuccess()
    } catch (e: any) {
      Message.error(e?.message || '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  const regenerateSingle = async () => {
    if (!singleNodeInfo) return
    setSubmitting(true)
    try {
      const tok = await createInstallToken(singleNodeInfo.id, {
        mode: deploy.mode, arch: deploy.arch,
        agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
        ttlSeconds: deploy.ttlSeconds,
      })
      setSingleToken(tok)
    } catch (e: any) {
      Message.error(e?.message || '重新生成失败')
    } finally {
      setSubmitting(false)
    }
  }

  const previewParams = {
    mode: deploy.mode, arch: deploy.arch,
    agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
  }

  return (
    <Modal
      title={fixedNode ? `为「${fixedNode.name}」生成安装命令` : '添加节点'}
      visible={visible}
      onCancel={handleClose}
      footer={null}
      style={{ width: 760 }}
      unmountOnExit
    >
      <Steps current={step} size="small" style={{ marginBottom: 24 }}>
        {!fixedNode && <Step title="节点信息" />}
        <Step title="部署参数" />
        <Step title="安装命令" />
      </Steps>

      {submitting && <div style={{ textAlign: 'center', padding: 32 }}><Spin /></div>}

      {!submitting && step === 0 && (
        <>
          <Step1NodeName
            mode={mode} onModeChange={setMode}
            singleName={singleName} onSingleNameChange={setSingleName}
            batchText={batchText} onBatchTextChange={setBatchText}
          />
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              <Button onClick={handleClose}>取消</Button>
              <Button type="primary" onClick={handleNextFromStep1}>下一步</Button>
            </Space>
          </div>
        </>
      )}

      {!submitting && step === 1 && (
        <>
          <Step2DeployOptions masterVersion={masterVersion} value={deploy} onChange={setDeploy} />
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Space>
              {!fixedNode && <Button onClick={() => setStep(0)}>上一步</Button>}
              <Button onClick={handleClose}>取消</Button>
              <Button type="primary" onClick={handleGenerate} loading={submitting}>
                生成安装命令
              </Button>
            </Space>
          </div>
        </>
      )}

      {!submitting && step === 2 && (
        <>
          {singleToken && singleNodeInfo && (
            <Step3CommandPreview
              nodeId={singleNodeInfo.id}
              nodeName={singleNodeInfo.name}
              token={singleToken}
              mode={deploy.mode}
              previewParams={previewParams}
              onRegenerate={regenerateSingle}
            />
          )}
          {batchRows.length > 0 && <BatchCommandTable rows={batchRows} />}
          <div style={{ marginTop: 24, textAlign: 'right' }}>
            <Button type="primary" onClick={handleClose}>完成</Button>
          </div>
        </>
      )}
    </Modal>
  )
}
```

**DRY 修正**：`handleGenerate` 里 single 模式同时调用了 `createNode` 和 `batchCreateNodes` 造成重复创建。修正为只调 `batchCreateNodes`:

```tsx
} else if (mode === 'single') {
  const created = await batchCreateNodes([singleName.trim()])
  const one = created[0]
  const tok = await createInstallToken(one.id, {
    mode: deploy.mode, arch: deploy.arch,
    agentVersion: deploy.agentVersion, downloadSrc: deploy.downloadSrc,
    ttlSeconds: deploy.ttlSeconds,
  })
  setSingleNodeInfo({ id: one.id, name: one.name })
  setSingleToken(tok)
}
```

- [ ] **Step 2: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/nodes/AgentInstallWizard.tsx
git commit -m "功能: AgentInstallWizard 主容器整合三步向导"
```

---

## Task 19: NodesPage 集成 Wizard + 操作列改造

**Files:**
- Modify: `web/src/pages/nodes/NodesPage.tsx`

- [ ] **Step 1: 改造 NodesPage**

Replace `web/src/pages/nodes/NodesPage.tsx` 的两个地方：

a) 顶部"添加节点"按钮改为打开 Wizard；移除原 Modal 与 newToken state。
b) 操作列加"生成安装命令" / "重新生成 Token"两个入口。

完整替换：

```tsx
import React, { useEffect, useState, useCallback } from 'react'
import {
  Table, Button, Space, Tag, Typography, PageHeader, Modal, Input, Message, Badge, Popconfirm, Card,
  Empty, Dropdown, Menu,
} from '@arco-design/web-react'
import {
  IconPlus, IconDelete, IconDesktop, IconCloudDownload, IconEdit, IconMore,
} from '@arco-design/web-react/icon'
import type { NodeSummary } from '../../types/nodes'
import { listNodes, deleteNode, updateNode, rotateNodeToken } from '../../services/nodes'
import { AgentInstallWizard } from './AgentInstallWizard'
import { fetchSystemInfo } from '../../services/system'

const { Text } = Typography

export default function NodesPage() {
  const [nodes, setNodes] = useState<NodeSummary[]>([])
  const [loading, setLoading] = useState(false)

  const [wizardVisible, setWizardVisible] = useState(false)
  const [wizardFixedNode, setWizardFixedNode] = useState<{ id: number; name: string } | undefined>()
  const [masterVersion, setMasterVersion] = useState('latest')

  const [editVisible, setEditVisible] = useState(false)
  const [editNode, setEditNode] = useState<NodeSummary | null>(null)
  const [editName, setEditName] = useState('')

  const fetchNodes = useCallback(async () => {
    setLoading(true)
    try {
      const data = await listNodes()
      setNodes(data)
    } catch {
      Message.error('获取节点列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchNodes()
    fetchSystemInfo().then((info) => setMasterVersion(info.version || 'latest')).catch(() => {})
  }, [fetchNodes])

  const handleDelete = async (id: number) => {
    try {
      await deleteNode(id)
      Message.success('节点已删除')
      fetchNodes()
    } catch {
      Message.error('删除节点失败')
    }
  }

  const handleEdit = async () => {
    if (!editNode || !editName.trim()) {
      Message.warning('请输入节点名称')
      return
    }
    try {
      await updateNode(editNode.id, { name: editName.trim() })
      Message.success('节点更新成功')
      setEditVisible(false)
      fetchNodes()
    } catch {
      Message.error('更新节点失败')
    }
  }

  const handleRotate = async (record: NodeSummary) => {
    try {
      const { newToken } = await rotateNodeToken(record.id)
      Modal.success({
        title: 'Token 已轮换',
        content: (
          <div>
            <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
              新 Token（24 小时内新旧 Token 均可认证，便于滚动替换）：
            </Text>
            <Text copyable style={{ fontFamily: 'monospace', fontSize: 12, wordBreak: 'break-all' }}>
              {newToken}
            </Text>
          </div>
        ),
      })
    } catch {
      Message.error('轮换 Token 失败')
    }
  }

  const columns = [
    {
      title: '节点名称', dataIndex: 'name',
      render: (name: string, record: NodeSummary) => (
        <Space>
          {record.isLocal ? <IconDesktop style={{ color: 'var(--color-primary-6)' }} /> : <IconCloudDownload />}
          <Text bold>{name}</Text>
          {record.isLocal && <Tag color="arcoblue" size="small" bordered>本机</Tag>}
        </Space>
      ),
    },
    {
      title: '状态', dataIndex: 'status', width: 100,
      render: (status: string) => status === 'online'
        ? <Badge status="success" text="在线" />
        : <Badge status="default" text="离线" />,
    },
    { title: '主机名', dataIndex: 'hostname', render: (v: string) => v || '-' },
    { title: 'IP 地址', dataIndex: 'ipAddress', render: (v: string) => v || '-' },
    {
      title: '系统', dataIndex: 'os', width: 120,
      render: (_: string, record: NodeSummary) => record.os
        ? <Tag bordered>{record.os}/{record.arch}</Tag> : '-',
    },
    { title: 'Agent 版本', dataIndex: 'agentVersion', width: 100, render: (v: string) => v || '-' },
    {
      title: '最后活跃', dataIndex: 'lastSeen', width: 170,
      render: (v: string) => v ? new Date(v).toLocaleString('zh-CN') : '-',
    },
    {
      title: '操作', width: 180,
      render: (_: unknown, record: NodeSummary) => (
        <Space>
          <Button type="text" icon={<IconEdit />} size="small"
            onClick={() => { setEditNode(record); setEditName(record.name); setEditVisible(true) }} />
          {!record.isLocal && (
            <>
              <Dropdown trigger="click" droplist={(
                <Menu>
                  <Menu.Item key="install"
                    onClick={() => { setWizardFixedNode({ id: record.id, name: record.name }); setWizardVisible(true) }}>
                    生成安装命令
                  </Menu.Item>
                  <Menu.Item key="rotate" onClick={() => handleRotate(record)}>
                    重新生成 Token
                  </Menu.Item>
                </Menu>
              )}>
                <Button type="text" icon={<IconMore />} size="small" />
              </Dropdown>
              <Popconfirm title="确定删除该节点？" onOk={() => handleDelete(record.id)}>
                <Button type="text" status="danger" icon={<IconDelete />} size="small" />
              </Popconfirm>
            </>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '0 4px' }}>
      <PageHeader
        title="节点管理"
        subTitle="管理集群中的服务器节点"
        extra={
          <Button type="primary" icon={<IconPlus />}
            onClick={() => { setWizardFixedNode(undefined); setWizardVisible(true) }}>
            添加节点
          </Button>
        }
      />

      <Card style={{ marginTop: 16 }}>
        <Table columns={columns} data={nodes} rowKey="id" loading={loading} pagination={false}
          noDataElement={<Empty description="暂无节点数据，系统将自动创建本机节点" />} />
      </Card>

      <AgentInstallWizard
        visible={wizardVisible}
        onClose={() => setWizardVisible(false)}
        onSuccess={fetchNodes}
        masterVersion={masterVersion}
        fixedNode={wizardFixedNode}
      />

      <Modal title="编辑节点" visible={editVisible}
        onCancel={() => setEditVisible(false)} onOk={handleEdit}
        okText="保存" cancelText="取消">
        <div style={{ marginBottom: 8 }}>
          <Text type="secondary">节点名称</Text>
        </div>
        <Input placeholder="输入节点名称" value={editName} onChange={setEditName} />
      </Modal>
    </div>
  )
}
```

**依赖**：确保 `web/src/services/system.ts` 有 `fetchSystemInfo` 函数返回 `{ version: string }`。若没有，检查现有代码：

```bash
grep -r "fetchSystemInfo\|version" web/src/services/system.ts
```

如果只需 version，可用现有端点 `GET /api/system/info` 的响应字段。若函数名不同，按需调整 import。

- [ ] **Step 2: 编译验证**

Run: `cd web && npm run build`
Expected: 构建成功

- [ ] **Step 3: 手动验证**

```bash
cd web && npm run serve
# 浏览器打开，登录后进入节点管理
# - 点"添加节点"走三步向导
# - 操作列下拉「生成安装命令」/「重新生成 Token」验证
```

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/nodes/NodesPage.tsx
git commit -m "功能: NodesPage 集成一键部署 Wizard 与操作列改造"
```

---

## Task 20: 前端回归与 Phase 2 合入

**Files:** 无新增

- [ ] **Step 1: 构建检查**

```bash
cd web && npm run build && npm run lint 2>/dev/null || true
```
Expected: 构建成功，lint 无新增警告

- [ ] **Step 2: 端到端手动测试清单**

- [ ] 单节点：输入名 → 选 systemd/amd64 → 生成命令 → 复制
- [ ] 单节点：选 docker → 看到 compose 额外命令
- [ ] 单节点：选 foreground → 仅一行 `curl | sudo sh`
- [ ] 批量：粘贴 3 行 → 生成 3 条命令的表 → 导出 .sh
- [ ] 操作列：「生成安装命令」跳过 Step1 → 直接 Step2
- [ ] 操作列：「重新生成 Token」 → 弹窗显示新 token
- [ ] 倒计时：5min TTL → 倒数到 0 后命令变灰、出现"重新生成"按钮
- [ ] 重复消费同一 token：浏览器第二次 GET /install/:token → 410

- [ ] **Step 3: 确认 Phase 2 完成**

---

# Phase 3 — 文档

## Task 21: 更新英文文档

**Files:**
- Modify: `docs-site/docs/features/multi-node.md`

- [ ] **Step 1: 重写 Walkthrough 章节**

Replace the "## Walkthrough" section in `docs-site/docs/features/multi-node.md` with:

```markdown
## Walkthrough

### 1. Open the install wizard

In the Web Console → **Node Management** → **Add Node**. You'll see a three-step wizard.

- **Step 1 — Node info.** Give the node a name, or switch to batch mode and paste multiple names (one per line, max 50).
- **Step 2 — Deploy options.** Pick install mode (`systemd` recommended, `docker`, or `foreground` for debugging), architecture (auto-detect by default), agent version (defaults to the master's version), TTL for the install link, and download source (`github` or the `ghproxy` mirror for mainland China).
- **Step 3 — Copy the command.** A single `curl ... | sudo sh` line is shown with a countdown timer (default 15 min). Click copy, paste into the target machine, and run with root privileges.

### 2. One-line install on the target host

Example (systemd):

```bash
curl -fsSL https://master.example.com/install/Xk3p9...vM | sudo sh
```

The script:

1. Detects OS and architecture (`uname -m`)
2. Downloads the matching `backupx` binary from GitHub Release (or ghproxy mirror)
3. Installs to `/opt/backupx-agent` and creates a `backupx` system user
4. Writes `/etc/systemd/system/backupx-agent.service` with the token baked in
5. Runs `systemctl enable --now backupx-agent`
6. Polls `/api/v1/agent/self` until the master confirms `status: online`

Reruns are idempotent — to upgrade or re-provision, simply generate a new install command and run it again.

### 3. Rotate tokens at any time

When a token needs to be rotated (compliance audit, leaked credentials, ...), go to the node's action menu → **Rotate Token**. The old token remains valid for 24 h to allow rolling restarts; after that it becomes invalid.

### 4. Batch deployment

In Step 1 choose "Batch" and paste node names. Step 3 shows a table with one command per node plus a "Download .sh" button that bundles all commands into a single shell file for SSH loops.
```

- [ ] **Step 2: Commit**

```bash
git add docs-site/docs/features/multi-node.md
git commit -m "文档: 更新英文多节点文档，描述一键部署向导"
```

---

## Task 22: 更新中文文档

**Files:**
- Modify: `docs-site/i18n/zh-CN/docusaurus-plugin-content-docs/current/features/multi-node.md`

- [ ] **Step 1: 更新中文 Walkthrough**

Replace the corresponding section with:

```markdown
## 一键部署步骤

### 1. 打开安装向导

Web 控制台 → **节点管理** → **添加节点**。打开三步向导：

- **第一步 · 节点信息**：填写节点名称，或切换"批量创建"粘贴多行名称（每行一个，最多 50 个）。
- **第二步 · 部署参数**：选择安装模式（`systemd` 推荐、`docker`、`foreground` 仅调试）、架构（默认自动检测）、Agent 版本（默认跟随 Master 版本）、有效期、下载源（`github` 直连或 `ghproxy` 镜像，国内服务器建议后者）。
- **第三步 · 安装命令**：一行 `curl ... | sudo sh` 命令，带倒计时（默认 15 分钟）。点击复制粘贴到目标机以 root 运行。

### 2. 目标机一条命令完成

例子（systemd 模式）：

```bash
curl -fsSL https://master.example.com/install/Xk3p9...vM | sudo sh
```

脚本会自动：

1. 检测操作系统与架构
2. 从 GitHub Release（或 ghproxy 镜像）下载对应的 `backupx` 二进制
3. 安装到 `/opt/backupx-agent`，创建系统用户 `backupx`
4. 写入 `/etc/systemd/system/backupx-agent.service`（token 已烧入环境变量）
5. 执行 `systemctl enable --now backupx-agent`
6. 轮询 `/api/v1/agent/self`，直到 Master 确认 `status: online`

脚本是幂等的：需要升级或重装时，重新生成一条安装命令再跑一次即可。

### 3. 随时轮换 Token

操作列 → **重新生成 Token**。旧 Token 24 小时内仍有效，便于滚动重启；超时后失效。

### 4. 批量部署

第一步选"批量创建"，粘贴节点名。第三步显示每个节点对应的命令表格，底部「导出 .sh」可打包为单个 shell 文件，便于 SSH 循环执行。
```

- [ ] **Step 2: Commit**

```bash
git add docs-site/i18n/zh-CN/docusaurus-plugin-content-docs/current/features/multi-node.md
git commit -m "文档: 同步中文多节点文档"
```

---

# 最终步骤

## Task 23: 全量回归 + PR 准备

**Files:** 无新增

- [ ] **Step 1: 全量测试**

```bash
cd server && go test ./... && go vet ./...
cd web && npm run build
```
Expected: 全部通过

- [ ] **Step 2: 检查 diff 覆盖**

与用户确认是否：
- 按三个阶段分三个 PR，分别请求 review（推荐）
- 或打包成一个 PR

- [ ] **Step 3: 视用户要求创建 PR**

示例（按用户要求触发时）：

```bash
# 切分支（用户确认后）
git checkout -b feat/one-click-agent-deploy
git push -u origin feat/one-click-agent-deploy

# 创建 PR（中文标题与正文，按 CLAUDE.md 规范）
gh pr create --title "功能: 一键部署 Agent 向导（Issue #43）" --body "$(cat <<'EOF'
## 概述

实现 [#43](https://github.com/Awuqing/BackupX/issues/43) 一键部署 Agent 向导，参考 Komari 体验：
- Web 向导：勾选参数 → 一次性 install token → `curl | sudo sh`
- 支持批量创建 50 节点 + 命令表导出
- Agent token 轮换（24h 过渡）
- 安全：install token 独立短寿命 + 限流 + 审计

## 改动范围

- 后端：新增 `agent_install_tokens` 表、`installtoken` 服务、`installscript` 渲染器、两组路由
- 前端：替换原 Modal 为三步向导 + 操作列改造
- 文档：更新 multi-node 中英双份

详见 [设计文档](docs/superpowers/specs/2026-04-19-one-click-agent-deploy-design.md)。

## 测试

- [x] 单元测试：repository + service 覆盖
- [x] 集成测试：`install_flow_test.go` 端到端流程
- [ ] 手动验收：systemd / docker / foreground 三模式在 Linux 容器里执行

## 兼容性

- 老 Agent 走 `POST /api/nodes/heartbeat` 不受影响
- `/install/:token` 是新增公开路由，对现有单机部署零影响
- DB 迁移可逆（DROP TABLE + ALTER TABLE DROP COLUMN）
EOF
)"
```

---

# Plan Self-Review

（编写完成后的自检笔记，不在执行时展开）

- ✓ Spec §3 数据模型 → Task 1 覆盖
- ✓ Spec §4.1 BatchCreate → Task 6 + Task 9
- ✓ Spec §4.2 InstallToken → Task 5 + Task 9
- ✓ Spec §4.3 RotateToken → Task 6 + Task 9
- ✓ Spec §4.4/§4.5 公开端点 → Task 8 + Task 11
- ✓ Spec §4.6 Agent Self → Task 7 + Task 10
- ✓ Spec §4.7 Preview → Task 9 (PreviewScript handler)
- ✓ Spec §5 安装脚本模板 → Task 4
- ✓ Spec §6 前端 UI → Task 14-19
- ✓ Spec §7 安全（限流 / 审计） → Task 5 + Task 8 + Task 9
- ✓ Spec §8 测试 → Task 3, 4, 5, 6, 7, 12
- ✓ Spec §9 分阶段发布 → Phase 1/2/3 结构
- ✓ Spec §10 回滚 → 设计文档已列，实施无需额外任务
- ⚠ Spec §11 `/api/system/releases` 复用：Task 16 注释里说"后续可接入"，V1 仅单项默认 Master 版本。符合 YAGNI

**类型一致性检查**：
- `InstallTokenInput`（service 层）与 `InstallTokenInput`（前端 TS）字段一致 ✓
- `NodeCreateResult` 后端返回 `{id, name}`，前端 `BatchCreateResult` 同构 ✓
- `InstallTokenResult` 前端期望 `{installToken, expiresAt, url, composeUrl}`，后端 handler 返回同构 ✓
- `rotateNodeToken` 前端期望 `{newToken}`，后端 `RotateToken` 返回 `gin.H{"newToken": tok}` ✓
