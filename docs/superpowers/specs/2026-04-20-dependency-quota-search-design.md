# 设计文档：任务依赖链 + 存储容量配额 + 全局搜索

- 日期：2026-04-20
- 状态：已落地
- 范围：第九轮企业工作流 + 容量治理 + 全局可达性

## 1. 目标

本轮补齐三类企业场景能力：
1. **工作流**：任务间依赖（A 备份成功后自动触发 B 归档）
2. **容量硬限制**：除了 85% 告警，需要严格拒绝超配额备份
3. **大规模可达性**：100+ 任务/记录场景下快速定位

## 2. 能力一：任务依赖链

### 2.1 数据
`BackupTask.DependsOnTaskIDs` CSV — 当前任务依赖的上游任务 ID 列表。

### 2.2 触发路径
```
BackupExecutionService.executeTask 上传成功 →
  DependentsResolver.TriggerDependents(upstreamID) →
    列出所有 depends_on 包含 upstreamID 的已启用任务 →
      逐个 RunTaskByID（best-effort，失败仅 warn）
```

`DependentsResolver` 接口由 `BackupTaskService` 实现，避免 execution 直接查仓储。

### 2.3 校验
- 上游任务存在性校验
- 不能自环（依赖自己）
- DFS 循环检测（depth > 32 视为潜在循环）

### 2.4 典型场景
- DB 备份成功 → 触发"归档打包"任务
- 多个源任务都成功 → 触发"合规报表生成"（多上游支持）

## 3. 能力二：存储容量软配额

### 3.1 模型
`StorageTarget.QuotaBytes` int64。0 = 不限制。

### 3.2 强制策略
`BackupExecutionService.executeTask` 上传前：
```
target.QuotaBytes > 0 AND
  currentUsed (来自 records.StorageUsage) + fileSize > QuotaBytes
→ 上传直接失败（不重试），记录 failed 原因
```

与 `storage_capacity_warning`（85% 通知）的区别：
- 容量预警：提醒运维人员清理/扩容
- 软配额：硬性拒绝超配额，避免失控

### 3.3 典型配置
- 生产数据库备份目标：QuotaBytes = 500 GB
- 冷备归档目标：QuotaBytes = 2 TB

## 4. 能力三：全局搜索

### 4.1 服务
`SearchService.Search(query)` 四类资源搜索：
- **任务**：name/type/tags/sourcePath/dbHost/dbName
- **存储目标**：name/description/type
- **节点**：name/hostname/ipAddress
- **最近 100 条备份记录**：fileName/storagePath/taskName

### 4.2 API
`GET /search?q=关键字` 返回 `{tasks, records, storage, nodes, totalCount}`，每类最多 10 条。

### 4.3 前端
顶部 Header 全局搜索入口：
- 假 Input 样式 + "Ctrl+K" 提示
- 点击/快捷键唤起 Modal
- Input 300ms debounce 触发后端搜索
- 分栏展示（任务 / 备份记录 / 存储目标 / 节点）
- 点击结果项导航到对应页面

### 4.4 设计权衡
- 不索引：依赖 SQL LIKE 足够应付 < 10000 任务规模
- 备份记录只搜最近 100 条：避免全表扫描，企业场景足够
- 无高亮：保持简单，后续可用 `<mark>` 加

## 5. 数据迁移

- 新字段 `backup_tasks.depends_on_task_ids` CSV
- 新字段 `storage_targets.quota_bytes` int64
- 无新表
- AutoMigrate 向后兼容（默认 0 / 空）

## 6. 双 review 通过

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 7. 未做（下一轮）

- Agent 自更新
- 加密密钥轮换（涉及数据迁移）
- WebSocket 实时推送
- PITR 增量备份
- SSO / OIDC
- 前端任务表单"上游依赖"多选器（后端 API 已就绪，UI 待补）
- 前端存储表单"配额"InputNumber（后端已就绪）
- 任务依赖图可视化
