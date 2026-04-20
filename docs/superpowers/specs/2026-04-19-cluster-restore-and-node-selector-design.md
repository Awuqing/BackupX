# 设计文档：集群感知恢复功能 & 任务节点选择器

- 日期：2026-04-19
- 状态：已通过（用户授权自主执行）
- 影响范围：server、web、agent
- 关联讨论：社区反馈"PVE 服务器能备份吗？有没有一键恢复"及作者回复"好像写成 bug 了"、"一键恢复后续优化"

## 1. 问题定义

### 1.1 B1 — 任务表单缺少执行节点选择器（Bug）

`web/src/components/backup-tasks/BackupTaskFormDrawer.tsx` 的草稿对象里有 `nodeId: 0` 字段，编辑时也能从 `initialValue.nodeId` 回填，但三步表单（基础/源/存储策略）**完全没有任何 Select 让用户选择节点**。结果：

- 所有任务被迫以 `nodeId = 0` 创建（Master 本地执行）
- 已安装的远程 Agent 根本拉不到 `run_task` 命令
- 多节点集群的核心价值失效

后端 `BackupExecutionService.startTask` 通过 `isRemoteNode(task.NodeID)` 判断路由，能力本就支持远程执行，缺口只在 UI。

### 1.2 恢复功能底层错误（架构级）

`server/internal/service/backup_execution_service.go:175 RestoreRecord`：

1. **同步阻塞**：HTTP POST 同步执行完整恢复流程，大文件/大库必超时
2. **忽视节点路由**：总是在 Master 本地 `runner.Restore`，无论任务绑定哪个节点
3. **无日志/无记录**：传 `backup.NopLogWriter{}`，用户看不到任何进度或失败原因；未建独立恢复记录
4. **前端误用状态**：`BackupRecordLogDrawer.handleRestore` 把"恢复已提交"塞进 `setStreamError`，UI 渲染为黄色警告

**架构后果**：任务绑定到 Agent 节点 A（源文件/数据库只在 A 可达）时，点击恢复 → Master 下载备份 → Master 本地恢复 → **文件写到 Master 的 `/var/www`、连 Master 本地不存在的数据库**。完全错的机器。

Agent 端 `server/internal/agent/executor.go` 只实现了 `handleRunTask` 与 `handleListDir`，从设计上就没有恢复能力。

## 2. 设计目标

- 恢复与备份**对称**：支持本地/远程节点路由，同一套设施（AgentCommand 队列、日志流）
- 恢复一等公民：独立 `RestoreRecord` 模型 + 异步执行 + LogHub SSE + 列表页
- 破坏性操作必须**可见且可确认**：前端恢复前弹窗展示目标位置、覆盖警告
- 复用现有基建，不引入新依赖/新抽象层

## 3. 架构设计

### 3.1 数据层

```go
// model/restore_record.go
type RestoreRecord struct {
    ID              uint
    BackupRecordID  uint   // 源备份记录
    TaskID          uint   // 冗余：便于筛选
    NodeID          uint   // 在哪个节点执行
    Status          string // running|success|failed
    ErrorMessage    string
    LogContent      string
    DurationSeconds int
    StartedAt       time.Time
    CompletedAt     *time.Time
    TriggeredBy     string // 用户名（审计冗余）
    CreatedAt, UpdatedAt
}
```

迁移：`database.go` 的 `AutoMigrate` 增加 `&model.RestoreRecord{}`。

### 3.2 服务层

新增 `service.RestoreService`：

```go
type RestoreService struct {
    restores   repository.RestoreRecordRepository
    records    repository.BackupRecordRepository
    tasks      repository.BackupTaskRepository
    targets    repository.StorageTargetRepository
    nodeRepo   repository.NodeRepository
    storage    *storage.Registry
    runners    *backup.Registry
    logHub     *backup.LogHub
    cipher     *codec.ConfigCipher
    dispatcher AgentDispatcher
    // ...依赖同 BackupExecutionService
}

// 启动恢复：同步创建 RestoreRecord → 判断路由 → 返回记录
func (s *RestoreService) Start(ctx, backupRecordID, triggeredBy) (*RestoreRecordDetail, error)

// Master 本地执行：下载 → 解密/解压 → runner.Restore(LogSink → LogHub)
func (s *RestoreService) executeLocally(ctx, restoreID)

// Agent 路由：EnqueueCommand("restore_record", {restoreRecordId})
func (s *RestoreService) dispatchToAgent(ctx, restore *model.RestoreRecord)
```

路由决策：

```
restore := 创建 RestoreRecord(status=running, nodeId=task.NodeID)
if isRemoteNode(task.NodeID):
    EnqueueCommand(nodeID, "restore_record", {restoreRecordId: restore.ID})
else:
    go executeLocally(restore.ID)  // 复用 BackupExecutionService.semaphore? 不，独立通道避免阻塞备份
return restore
```

### 3.3 Agent 端

#### 3.3.1 新增命令类型

`model/agent_command.go`：

```go
const AgentCommandTypeRestoreRecord = "restore_record"  // Payload: {"restoreRecordId": N}
```

#### 3.3.2 Master ↔ Agent API（复用 Agent API 组）

- `GET  /api/agent/restores/:id/spec`  → 返回 `AgentRestoreSpec`（已解密存储配置、任务 spec、备份记录 storagePath/fileName）
- `POST /api/agent/restores/:id`       → `AgentRestoreUpdate`（status / errorMessage / logAppend）

`AgentRestoreSpec`：

```go
type AgentRestoreSpec struct {
    RestoreRecordID uint
    BackupRecordID  uint
    TaskID          uint
    TaskName, Type  string
    SourcePath      string
    SourcePaths     string
    DBHost, DBName  string
    // ... 同 AgentTaskSpec 的任务字段
    Storage         AgentStorageTargetConfig // 只需下载源目标
    StoragePath     string                   // 远端对象 key
    FileName        string
    Compression     string
    Encrypt         bool  // 当前 Agent 不支持加密恢复，直接返回失败
}
```

#### 3.3.3 Agent Executor

`agent/executor.go` 新增 `ExecuteRestore(restoreRecordID)`：

1. `client.GetRestoreSpec(restoreRecordID)`
2. 若 `Encrypt == true` → `UpdateRestoreRecord(status=failed, errorMessage="Agent 不支持加密恢复")`
3. 临时目录下载备份文件（通过 storage provider `Download`）
4. `.enc` 或 `.gz` 的逆向处理（当前不支持加密；`.gz` 调 `compress.GunzipFile`）
5. `runner.Restore(backupSpec, preparedPath, restoreLogger)` — logger 把每行通过 `UpdateRestoreRecord{LogAppend}` 回传
6. 成功 → `UpdateRestoreRecord(status=success)`

`agent/agent.go` 的 `switch cmd.Type` 增加 `"restore_record": handleRestoreRecord`。

### 3.4 HTTP 层

新增 handler `restore_record_handler.go`：

```
POST /api/backup/records/:id/restore      → 202，body: {restoreRecordId}
GET  /api/restore/records                 → 列表（支持 ?taskId, ?status 筛选）
GET  /api/restore/records/:id             → 详情（含 logContent）
GET  /api/restore/records/:id/logs/stream → SSE（复用 LogHub，sequence 事件协议）
```

Agent 端点 `agent_handler.go`：

```
GET  /api/agent/restores/:id/spec
POST /api/agent/restores/:id
```

`router.go` 对应注册。注意：`LogHub` 的 recordID 命名空间是 `uint`，恢复记录 ID 可能与备份记录 ID 冲突 → 决策：

- **方案**：LogHub 加 `topic` 维度 —— 工作量较大
- **简化方案**：恢复记录用 `restoreID + 常量偏移` 或使用独立 `LogHub` 实例

本次选择**独立 LogHub 实例**（`RestoreLogHub`），彻底隔离，代码量最小。

### 3.5 前端

#### 3.5.1 修 B1 — 节点选择器

`BackupTaskFormDrawer.tsx`：

- 已有 `localNodeId` prop
- 新增 `nodes: NodeSummary[]` prop（由父组件传入）
- `renderBasicStep()` 增加：

```tsx
<div>
  <Typography.Text>执行节点</Typography.Text>
  <Select
    value={draft.nodeId || undefined}
    placeholder="留空或选择本机 = 在 Master 执行"
    allowClear
    options={nodeOptions}  // [{label: `${name} (${status})`, value: id}]
    onChange={(value) => updateDraft({ nodeId: Number(value ?? 0) })}
  />
  <Typography.Paragraph type="secondary" style={{ marginBottom: 0, marginTop: 4 }}>
    任务将在该节点上执行备份与恢复；源路径/数据库以该节点视角解析。
  </Typography.Paragraph>
</div>
```

`BackupTasksPage`：把已加载的 `nodeList` 传给 FormDrawer。

#### 3.5.2 恢复 UX

- `BackupRecordLogDrawer.handleRestore`：
  - 打开 `RestoreConfirmDialog`（列出将覆盖的目标路径/数据库 + 执行节点 + 风险说明）
  - 确认后 POST restore，拿 `restoreRecordId`
  - `Message.success('恢复已启动，正在打开日志')`
  - 关闭抽屉 → `navigate('/restore/records?restoreId=X')`
- 新增 `components/restore-records/RestoreRecordLogDrawer.tsx`（结构复刻 BackupRecordLogDrawer，去掉下载/删除按钮）
- 新增 `pages/restore-records/RestoreRecordsPage.tsx`（列表 + 状态 tag + 点击打开 Drawer）
- `router/index.tsx` 加 `restore/records` 路由
- `layouts/AppLayout.tsx` 菜单加"恢复记录"

#### 3.5.3 Types & Services

- `types/restore-records.ts`
- `services/restore-records.ts`：`listRestoreRecords`、`getRestoreRecord`、`startRestoreFromBackup`、`streamRestoreRecordLogs`

### 3.6 依赖注入（app.go）

```go
restoreRecordRepo := repository.NewRestoreRecordRepository(db)
restoreLogHub := backup.NewLogHub()
restoreService := service.NewRestoreService(
    restoreRecordRepo, backupRecordRepo, backupTaskRepo, storageTargetRepo,
    nodeRepo, storageRegistry, backupRunnerRegistry, restoreLogHub, configCipher,
    agentService, cfg.Backup.TempDir, cfg.Backup.MaxConcurrent)
// 注入到 router
```

`BackupRecordHandler.Restore` 改为委托给 `RestoreService.Start`。旧的 `BackupExecutionService.RestoreRecord` 保留（本地执行逻辑抽取到 RestoreService 复用），对外 HTTP 契约变更：

- **新契约**：`POST /backup/records/:id/restore` 返回 `{restoreRecordId: N}`（前端改为跳转到恢复详情页，而不是等同步完成）
- **Agent**：新增 `handleRestoreRecord`

### 3.7 安全性

- 恢复是破坏性操作：后端审计日志已记录
- 前端二次确认
- 路径穿越：`FileRunner.Restore` 已有 `strings.HasPrefix` 校验 targetParent，沿用

### 3.8 迁移与兼容性

- 旧 `BackupRecordService.Restore` 方法保留，改为内部调用新 `RestoreService.Start`（避免外部使用方报错）—— 但 HTTP 输出变化是已知 breaking
- 因为"恢复"目前是废的（见底层错误），前端无历史记录显示，破坏性 HTTP 变更可接受
- 数据库无删表操作，只 AutoMigrate 新表

## 4. 非目标（YAGNI）

本次**不做**：
- 恢复到自定义路径/自定义数据库连接（路径穿越、鉴权面大，留作 v2）
- 恢复干运行（dry-run）
- Agent 加密恢复（与 Agent 加密备份同策略：加密密钥不下发到 Agent）
- 跨节点恢复（把 Agent A 的备份恢复到 Agent B）—— 任务绑定哪个节点就在哪个节点恢复

## 5. 测试策略

### 后端
- `RestoreService.Start`：本机任务 → 走本地分支；远程任务 → 入队 `AgentCommand`
- `RestoreRecordRepository`：CRUD + 列表筛选
- `Agent Executor.ExecuteRestore`：mock HTTP client + stub runner

### 前端
- 通过 `tsc --noEmit` 保证类型安全
- 新增的 Dialog/Drawer/Page 至少跑通渲染（现有测试框架 vitest）

### 双 review 清单
- `go build ./...` / `go vet ./...` / `go test ./... -count=1 -race` 全绿
- `npm run build`（前端） 通过
- CLAUDE.md 规范：所有错误必须处理、中文 commit、不引入新 UI 库
- 修改范围对照讨论：B1 节点选择器 ✅、恢复底层重构 ✅

## 6. 实施顺序

1. RestoreRecord model + migration + repository
2. AgentCommand 新命令类型常量
3. RestoreService（本地执行 + 节点路由）
4. AgentService + HTTP：GetRestoreSpec / UpdateRestoreRecord
5. Agent client + executor：ExecuteRestore
6. Master HTTP：RestoreHandler + router
7. app.go 依赖注入
8. 前端：types/services → 节点选择器 → 确认对话框 → 日志抽屉 → 列表页 → 路由 + 菜单
9. 修 B2（`handleRestore` 改为 Message.success + 跳转）
10. 单元测试
11. 双 review（build/vet/test + tsc）
