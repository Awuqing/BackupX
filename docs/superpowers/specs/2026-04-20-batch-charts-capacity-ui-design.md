# 设计文档：批量操作 + Dashboard 图表 + 存储容量 UI

- 日期：2026-04-20
- 状态：已落地
- 范围：第八轮前端图表化闭环 + 规模化运维 UI

## 1. 目标

前七轮把企业级后端能力做齐：集群、验证、SLA、RBAC、API Key、3-2-1 复制、存储健康、维护窗口、任务模板、Agent 版本感知、集群概览、存储容量监控、审计 CSV、K8s 健康检查、多维统计 API。

本轮关注"可用的 UI"：
1. **任务批量操作**：100+ 任务场景下逐个操作低效
2. **Dashboard 图表化**：多维统计 API 已有（第七轮），UI 缺失
3. **存储容量可视化**：预警事件已派发（第七轮），列表需看到使用率

## 2. 能力一：任务批量操作

### 2.1 后端
`BackupTaskService` 新增：
- `BatchToggle(ctx, ids, enabled)`：批量启停
- `BatchDeleteTasks(ctx, ids)`：批量删除
- `BatchResult` 单条结果：`{id, name, success, error}`

`BackupRunHandler` 新增 `BatchRun`：循环调用 `RunTaskByID`，best-effort。

HTTP：
```
POST /backup/tasks/batch/toggle   # {ids, enabled}
POST /backup/tasks/batch/delete   # {ids}
POST /backup/tasks/batch/run      # {ids}
```

全部需要 `RequireNotViewer()`。审计日志记录"批量 X N/M 个任务"。

### 2.2 前端
- 任务列表开启 `rowSelection`（仅 writable 用户可见）
- 选中 > 0 时顶部浮现工具条：批量执行 / 启用 / 停用 / 删除 / 取消
- 批量后 Message 展示"成功 X / 失败 Y"

## 3. 能力二：Dashboard 多维统计图表

### 3.1 实现
`fetchDashboardBreakdown(30)` 调用第七轮的 `/dashboard/breakdown?days=30`。

两个图表：
- **任务类型分布**：饼图（file/mysql/postgresql/sqlite/saphana）
- **任务按节点分布**：柱状图（含本机 Master）

### 3.2 设计决策
- 只在有数据时展示（避免空图浪费屏幕）
- 使用 ECharts BarChart + PieChart，共享已注册组件
- 颜色方案与存储使用量饼图一致

### 3.3 未做
存储分组的"字节数饼图"已在 Dashboard 现有"存储使用量分布"中（来自 `stats.storageUsage`），不重复。

## 4. 能力三：存储容量 UI

### 4.1 前端
存储目标列表卡片内：
- 加载时异步获取每个启用目标的 `GetUsage`（含 About 的 diskUsage）
- 若后端返回 `diskUsage.total + used` → 进度条 + 使用率文字 + 容量预警标签（≥85% 红）
- 若仅有累计备份字节数 → 降级展示"已用备份 X（N 个记录）"

### 4.2 进度条颜色
- < 70%：绿色（#00B42A）
- 70-85%：橙色（#FF7D00）
- ≥ 85%：红色（#F53F3F）+ "容量预警"标签

### 4.3 后端
无改动。第七轮已有的 `StorageDiskUsage` 字段 + HealthMonitor 已支持。

## 5. 双 review 通过

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 6. 未做（下一轮）

- 备份加密密钥轮换（涉及数据迁移）
- WebSocket 实时 Dashboard
- Agent 自更新
- PITR 增量备份
- SSO / OIDC
- 报表 PDF 导出
- 任务依赖（A 完成后 B 执行）
- 备份元数据全局搜索
