# 设计文档：实时事件流 + 依赖图可视化 + UI 闭环

- 日期：2026-04-20
- 状态：已落地
- 范围：第十轮实时体验 + 上轮 UI 收口

## 1. 目标

前九轮完成所有企业级后端能力。本轮聚焦"可感知"：
1. **实时体验**：事件发生时 Dashboard 即刻刷新，无需手动 F5
2. **工作流可视化**：依赖关系以图形方式展示，直观理解拓扑
3. **UI 闭环**：上轮后端就绪的依赖配置 + 存储配额需要表单接入

## 2. 能力一：实时事件流（SSE）

### 2.1 设计选型

用 SSE 而非 WebSocket：
- 原生浏览器支持、自动重连
- 单向推送足够（前端订阅、后端推送）
- 不引入新依赖（go-net 标准库）
- 企业场景穿越反向代理无障碍

### 2.2 后端架构

```
notification.DispatchEvent(eventType, ...) →
  1. broadcaster.Publish（非阻塞 SSE 推送）
  2. collectSubscribers + deliver（邮件/Webhook 等持久渠道）
```

双通道设计：
- **EventBroadcaster**（内存）：前端实时 UI
- **NotificationService**（持久+多渠道）：合规审计、离线告警

订阅者 channel buffer = 32，满时丢弃单条，不阻塞生产者。

### 2.3 HTTP 端点

```
GET /api/events/stream
```

- JWT/API Key 认证
- Content-Type: text/event-stream
- 心跳：每 25s 发 `: heartbeat` 注释行保活
- 禁用 nginx 缓冲（X-Accel-Buffering: no）

### 2.4 前端 Hook

`useEventStream(handler, types?)`：
- 用 fetch + ReadableStream 解析 SSE（支持 Bearer token）
- 指数退避重连（1s → 2s → 4s → ... → 30s）
- 可选事件类型过滤，避免无关事件触发重渲染

### 2.5 Dashboard 订阅

监听 8 类事件，任一到达 → 刷新 Dashboard 全量数据：
```
backup_success/failed, restore_success/failed,
verify_failed, sla_violation,
storage_unhealthy, storage_capacity_warning
```

## 3. 能力二：任务依赖图可视化

### 3.1 实现

`TaskDependencyGraph` 组件用 ECharts GraphChart：
- **节点**：任务，按 `lastStatus` 着色（绿成功/红失败/蓝执行/灰空闲）
- **边**：`dependsOnTaskIds` → 当前任务（上游 → 下游）
- **布局**：force 物理仿真，支持拖拽/缩放
- **过滤**：只显示有依赖关系的任务（孤立节点忽略减噪）

### 3.2 集成

任务页 `BackupTasksPage` 表格上方嵌入。无依赖时显示 Empty 引导。

## 4. 能力三：UI 闭环

### 4.1 任务表单 - 上游依赖选择器

`BackupTaskFormDrawer` 新增 "任务依赖" 区块：
- 多选 Select：系统内所有任务（排除自己）
- 帮助文案说明循环依赖自动检测

`BackupTasksPage` 传入 `allTasks`。

### 4.2 存储表单 - 配额输入

`StorageTargetFormDrawer` 新增 "容量配额（GB）"：
- InputNumber（GB 单位，0 = 不限制）
- 内部存 bytes，显示 GB
- 帮助文案区分软配额与 85% 预警

## 5. 数据结构

- 前端 Types：`backup-tasks.dependsOnTaskIds` + `storage-targets.quotaBytes`
- 无数据库变更（后端字段已落地）

## 6. 双 review

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 7. 未做

- Agent 自更新
- 加密密钥轮换
- PITR 增量备份
- SSO / OIDC
- Dashboard 事件流 Toast 展示（当前仅静默刷新）
- 事件历史面板（内存事件可查询）
