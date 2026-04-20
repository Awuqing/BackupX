# 设计文档：BackupX 企业级深化 — RBAC + API Key + 事件总线 + 节点配额

- 日期：2026-04-19
- 范围：本轮聚焦企业级权限、DevOps 集成与集群资源隔离
- 状态：已落地（用户授权自主执行）

## 1. 问题与目标

前三轮已完成"集群路由、可验证恢复、SLA 监控、任务分组"。企业化缺口：
1. **多用户 / 权限隔离**：系统只有一个 admin，团队无法协作
2. **DevOps 集成**：CI/CD、监控脚本只能用户名密码登录（反模式）
3. **事件订阅**：仅备份成功/失败，verify/restore/SLA 等扩展事件不触达
4. **集群资源管理**：所有节点共享全局 MaxConcurrent，小内存节点被挤爆

本轮交付：
- **RBAC**：admin / operator / viewer 三级 + 中间件 + 前端控权
- **API Key**：`bax_` 前缀，SHA-256 哈希存储，角色继承
- **事件总线**：Notification 支持多事件订阅（`backup_success|backup_failed|verify_failed|restore_*|sla_violation`）
- **节点级并发配额**：Node.MaxConcurrent / BandwidthLimit，独立 semaphore

## 2. RBAC 设计

### 2.1 角色定义

```
admin     全权（用户管理、API Key、系统设置、节点管理、删除数据）
operator  日常运维（任务/存储/通知 CRUD、触发执行/恢复/验证）
viewer    只读（仪表盘、任务列表、记录、日志，不能触发或改变状态）
```

### 2.2 实现

**模型层**：`User.Role` 已存在，补充 `User.Disabled`、常量 + `IsValidRole()`。

**中间件**（server/internal/http/middleware.go）：
- `AuthMiddleware(jwtManager, apiKeyAuth)`：支持 JWT（现有）+ API Key（`bax_` 前缀）
- `RequireRole(roles...)`：白名单角色
- `RequireNotViewer()`：快捷方式 — 禁止 viewer 触发写入/变更

**路由映射**（server/internal/http/router.go）：
- 全部 GET 列表/详情：仅需 AuthMiddleware（viewer 可见）
- POST/PUT/DELETE 任务、存储、通知、记录操作：+`RequireNotViewer()`
- 用户管理、API Key、节点管理、系统设置写入：+`RequireRole("admin")`

**前端**：
- `utils/permissions.ts`：`isAdmin/canWrite/isViewer/roleLabel`
- `AppLayout` 菜单按角色过滤（用户/API Key 菜单仅 admin 可见）
- 任务列表按钮、记录抽屉操作按 `canWrite()` 隐藏
- 顶部用户名后缀角色标签

### 2.3 兼容性

- 首位用户仍由 Setup 创建为 admin（无破坏）
- 现有 User.Role 默认值 admin 保持

## 3. API Key

**明文格式**：`bax_` + 24 字节随机 hex（24 位熵，192 bit）

**存储**：KeyHash = SHA-256(明文)，Prefix 取前 12 字符供 UI 区分

**识别**：中间件看到 `Authorization: Bearer bax_xxx` 或 `X-Api-Key: bax_xxx` 走 API Key 路径

**管理**（仅 admin）：
- `GET /api-keys` 列表
- `POST /api-keys` 创建（返回一次明文 + summary）
- `PUT /api-keys/:id/toggle` 启停
- `DELETE /api-keys/:id` 撤销

**审计**：每次使用更新 `LastUsedAt`，创建/撤销记审计日志

**安全考虑**：
- 24 字节随机熵，无需加盐
- 无明文日志 / 无明文存储
- 过期支持（TTL 小时数，0=永久）
- 一次性展示：UI Modal 创建后显示明文 + 复制按钮，确认关闭后不可再查看

## 4. 事件总线

### 4.1 事件类型

```
backup_success     备份成功
backup_failed      备份失败
restore_success    恢复成功
restore_failed     恢复失败
verify_failed      验证未通过
sla_violation      SLA 违约（后台监控事件）
```

### 4.2 订阅模型

`Notification.EventTypes` 新字段（CSV）。匹配规则：
- EventTypes 非空：严格匹配订阅事件
- EventTypes 为空：沿用 OnSuccess/OnFailure 旧语义（仅 backup_*）

### 4.3 统一分发

```go
type EventDispatcher interface {
    DispatchEvent(ctx, eventType, title, body, fields) error
}

// NotificationService 实现该接口
// VerificationEventNotifier / RestoreService.dispatchRestoreEvent 分别调用
```

触发点集成：
- `BackupExecutionService.NotifyBackupResult` → 派发 `backup_success/backup_failed`
- `VerificationService.executeLocally`（失败时）→ 派发 `verify_failed`
- `RestoreService.executeLocally`（终态）→ 派发 `restore_success/restore_failed`
- **SLA 违约**（后续可由后台 monitor 调用 DispatchEvent(sla_violation)）

## 5. 节点配额（集群优化）

### 5.1 字段

`Node.MaxConcurrent` (int, 0=不限) + `Node.BandwidthLimit` (string, rclone 格式)

### 5.2 执行模型

`BackupExecutionService` 新增 `nodeSemaphores sync.Map`（懒加载 per-node channel）：

```go
func (s) acquireNodeSemaphore(ctx, nodeID) chan struct{} {
    if nodeID == 0 || nodeRepo == nil { return nil }
    if v, ok := nodeSemaphores.Load(nodeID); ok { return v.(chan struct{}) }
    node, _ := nodeRepo.FindByID(ctx, nodeID)
    if node == nil || node.MaxConcurrent <= 0 { return nil }
    created := make(chan struct{}, node.MaxConcurrent)
    actual, _ := nodeSemaphores.LoadOrStore(nodeID, created)
    return actual.(chan struct{})
}

func (s) executeTask(...) {
    if nodeSem := acquireNodeSemaphore(ctx, task.NodeID); nodeSem != nil {
        nodeSem <- struct{}{}
        defer func() { <-nodeSem }()
    }
    s.semaphore <- struct{}{} // 全局保底
    defer func() { <-s.semaphore }()
    ...
}
```

**约束**：节点容量在首次创建通道时采用，运行时修改 MaxConcurrent 需重启服务生效（避免 resize channel 的 race）。

### 5.3 UI

节点管理页新增字段（编辑节点时）：最大并发、带宽限制。`NodeUpdateInput` 已扩展。

## 6. 数据迁移

新增表：`api_keys`  
新增字段：`users.disabled`、`notifications.event_types`、`nodes.max_concurrent`、`nodes.bandwidth_limit`  
全走 AutoMigrate，向后兼容（默认值不破坏现有功能）。

## 7. 验证

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` 通过
- `npx tsc --noEmit` ✅
- 集群与企业级测试补丁：
  - API Key 哈希不可逆（单测可验 SHA-256 确定性 + rawKey mismatch 拒绝）
  - 节点 semaphore 懒加载（channel LoadOrStore 幂等）
  - 事件分发按订阅匹配（EventTypes 非空时严格）

## 8. 未做（留给下一轮）

- SSO / OIDC（企业 SSO 接入）
- 节点 Agent 自更新
- 备份复制 / 异地镜像
- SLA 违约后台主动扫描 + DispatchEvent 自动触发
- API Key IP 白名单
- 合规报表导出（PDF/CSV）
