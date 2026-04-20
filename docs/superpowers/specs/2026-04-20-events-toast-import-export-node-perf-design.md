# 设计文档：事件 Toast + 任务导入导出 + 节点性能统计

- 日期：2026-04-20
- 状态：已落地
- 范围：第十一轮体验增强 + 集群迁移 + 可观测性

## 1. 能力一：实时事件 Toast + 历史抽屉

### 1.1 前端架构
- `useEventStore`（zustand）：会话内保留最近 50 条事件 + 未读计数
- `EventCenter` 组件：Bell 图标 + 未读徽章 + 抽屉列表
- 订阅 SSE 全事件流（而非仅 Dashboard 子集）
- 按事件类型映射：
  - `success` toast：backup_success / restore_success
  - `error` toast：backup_failed / restore_failed / verify_failed / replication_failed / storage_unhealthy
  - `warning` toast：sla_violation / storage_capacity_warning / agent_outdated

### 1.2 设计决策
- 无持久化：避免 localStorage 膨胀；事件重要性由后端 Notification 保证
- 抽屉打开自动标记已读，简化交互

## 2. 能力二：任务配置导入/导出 JSON

### 2.1 后端
`TaskExportService`：
- `Export(taskIDs)` 返回 `ExportPayload{version, exportedAt, tasks}`
- `Import(payload)` 两阶段：
  1. 创建所有任务（忽略 DependsOn）
  2. 补齐依赖关系（上游名 → 新 ID）
- 敏感字段排除：DBPasswordCiphertext、存储凭证

### 2.2 命名引用
- 存储目标 / 节点 / 依赖任务均按 **name** 引用
- 导入时按名称 lookup 现有系统 ID
- 找不到则静默降级（如节点缺失 → NodeID=0 本机）

### 2.3 冲突策略
任务名已存在时 **跳过**（不覆盖），避免误操作。用户需先删除再导入。

### 2.4 HTTP
```
GET  /api/backup/tasks/export?ids=1,2,3   # 不传 ids 导全部
POST /api/backup/tasks/import             # JSON body，1MB 限制
```

### 2.5 前端
任务页 Header 新增 "导出 JSON" / "导入 JSON"（Upload 组件 `beforeUpload` 阻止实际上传），导入结果 Modal 展示每行创建/跳过/失败状态。

## 3. 能力三：节点性能统计

### 3.1 API
`GET /dashboard/node-performance?days=30` 返回：
```
[{
  nodeId, nodeName, isLocal,
  totalRuns, successRuns, failedRuns, successRate,
  totalBytes, avgDurationSecs,
}]
```

### 3.2 实现
- 复用 `BackupRecord.NodeID`（第二轮加入的字段）
- 单次 List 近 N 天记录 → 按 NodeID 内存聚合
- 按成功率降序，其次按执行次数降序

### 3.3 前端
Dashboard 新增"节点执行表现（近 30 天）"表格：
- 节点名（带 Master 标签）
- 执行次数 / 成功 / 失败
- 成功率（≥95% 绿，≥80% 黄，<80% 红）
- 备份总量（字节）
- 平均耗时

## 4. 双 review

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 5. 未做

- Agent 自更新（远程下发二进制 + 信任链）
- 加密密钥轮换（数据迁移）
- PITR 增量备份
- SSO / OIDC
- 导入时覆盖模式（当前只支持跳过）
- 导入时自动补全缺失存储目标（需要凭证，慎重）
