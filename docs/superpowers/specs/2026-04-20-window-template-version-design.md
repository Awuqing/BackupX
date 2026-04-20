# 设计文档：维护窗口 + 任务模板 + Agent 版本感知 + 集群概览

- 日期：2026-04-20
- 范围：第六轮企业级增强，聚焦集群规模化运维
- 状态：已落地

## 1. 目标

前五轮已完成：集群路由、验证、SLA 监控、RBAC、API Key、事件总线、节点配额、备份复制、存储健康。

本轮补齐集群规模化运维最后一公里：
1. **维护窗口**：业务高峰期禁止备份调度
2. **任务模板**：一次保存，N 次批量创建（100+ 主机刚需）
3. **Agent 版本感知**：节点 Agent 落后 Master 主动告警
4. **集群概览**：Dashboard 一眼看齐所有节点健康度

## 2. 能力一：维护窗口

### 2.1 模型
- 新字段 `BackupTask.MaintenanceWindows` CSV
- 语法：`time=HH:MM-HH:MM` 或 `days=mon|tue,time=22:00-06:00`
- 支持多段（`;` 分隔）、跨午夜（start > end）、指定星期

### 2.2 核心实现
`backup/window.go` 新增：
- `ParseMaintenanceWindows(string) → []MaintenanceWindow`
- `IsWithinWindow(t, windows) bool` — 判断 t 是否在任一窗口
- `ValidateMaintenanceWindows(string) error` — 输入合法性校验

### 2.3 集成
- **调度器**：`syncTaskLocked` cron fire 时校验当前时间，非窗口跳过并审计
- **手动执行**：`BackupExecutionService.startTask` 同样校验（防止业务高峰误触发）
- **前端**：任务表单新增"维护窗口"输入 + 帮助文案

### 2.4 测试
`backup/window_test.go` 覆盖：同日/跨夜/星期过滤/多段组合/无效输入

## 3. 能力二：任务模板

### 3.1 模型
```go
TaskTemplate {
    ID, Name, Description, TaskType
    Payload     string  // 序列化的 BackupTaskUpsertInput
    CreatedBy
    CreatedAt, UpdatedAt
}
```

### 3.2 服务
`TaskTemplateService`：
- CRUD：`List / Get / Create / Update / Delete`
- 批量应用：`Apply(id, input) → []Result`
  - 每个 Variables 条目 name 必填，覆盖模板 Name
  - sourcePath / sourcePaths / dbHost / dbName / tags / nodeId 若提供则覆盖
  - best-effort：单个失败不影响其他，返回详细结果

### 3.3 HTTP
```
GET    /task-templates           列表
GET    /task-templates/:id       详情
POST   /task-templates           创建（operator+）
PUT    /task-templates/:id       更新（operator+）
DELETE /task-templates/:id       删除（operator+）
POST   /task-templates/:id/apply 批量应用（operator+）
```

### 3.4 前端
- 新菜单 `/task-templates`
- 列表 + 每行"应用"按钮 → Modal 动态添加行 → 批量创建 → 展示结果表
- 对 viewer 隐藏写入操作

## 4. 能力三：Agent 版本感知

### 4.1 实现
`ClusterVersionMonitor`：
- 每 30 分钟扫描所有远程节点
- 比较 `node.AgentVer` vs `master.Version`（major.minor 级别）
- 落后节点派发 `agent_outdated` 事件
- 同节点 24 小时内只告警一次
- 版本升级后自动清除记忆，允许下次落后再告警

### 4.2 版本比较策略
- 宽松策略：只比 `major.minor`，放过 patch 差异避免小版本发布噪音
- `dev` 版本 / 空版本不告警
- 解析失败保守不告警

### 4.3 事件
新增 `agent_outdated`，接入现有 Notification 总线

## 5. 能力四：Dashboard 集群概览

### 5.1 API
`GET /dashboard/cluster` 返回：
- Master 版本
- 总节点数、在线数、离线数、过期 Agent 数
- 每节点详情：名称/主机名/状态/版本/版本状态/任务数/最近心跳

### 5.2 前端
Dashboard 新增"集群概览"卡片：
- 4 个统计指标
- 节点列表表格（状态徽章、版本状态着色）
- 仅在 totalNodes > 0 时展示（单节点场景不打扰）

## 6. 事件总线扩展

新事件：`agent_outdated`  
订阅方式与其他企业事件一致（Notification.EventTypes CSV）

## 7. 数据迁移

- 新表：`task_templates`
- 新字段：`backup_tasks.maintenance_windows`
- 全 AutoMigrate，向后兼容

## 8. 双 review 通过

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- 新增测试：`backup/window_test.go` 6 条（同日/跨夜/星期/多段/无效/空）
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 9. 未做（下一轮）

- Agent 自更新（远程分发二进制 + 信任链）
- 备份加密密钥轮换
- WebSocket 实时 Dashboard
- 报表 PDF/CSV 导出
- PITR 增量备份
- SSO / OIDC
