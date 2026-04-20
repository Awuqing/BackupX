# 设计文档：存储容量监控 + 审计日志导出 + K8s 健康检查 + Dashboard 多维统计

- 日期：2026-04-20
- 状态：已落地
- 范围：第七轮企业运维 + 合规能力增强

## 1. 目标

前六轮完成的能力：集群路由、验证演练、SLA 监控、RBAC、API Key、事件总线、节点配额、备份复制、存储健康、维护窗口、任务模板、Agent 版本感知、集群概览。

本轮补齐三类常见企业运维痛点 + 合规刚需：
1. **存储快满才发现**：TestConnection 通过不代表还有空间
2. **审计合规导出**：月度合规报表需要 CSV 导出到外部归档
3. **容器化部署**：K8s/Swarm 需要 liveness/readiness 探针
4. **Dashboard 信息密度**：单维度统计看不清"哪类任务最多/哪个节点负载重"

## 2. 能力一：存储容量监控

### 2.1 实现
`StorageTargetService.runCapacityCheckOnce` 与健康扫描同频运行（每 5 分钟）：
- 列出所有启用的存储目标
- 类型断言 `StorageAbout` 接口，支持的后端（local_disk / WebDAV 等）执行 About
- 使用率 `Used/Total >= 85%` 派发 `storage_capacity_warning` 事件
- 降到阈值以下清除告警记忆

### 2.2 常量决策
阈值做成 `const StorageCapacityWarningThreshold = 0.85`，不提供配置：
- 业界运维标准线（监控告警通用 85%）
- 留简单配置点反而增加运维复杂度
- 如需其他阈值，用户可订阅 provider 原生监控

### 2.3 新事件
`storage_capacity_warning`：Notification 订阅后可用 Webhook/邮件/Telegram 接收

## 3. 能力二：审计日志高级筛选 + CSV 导出

### 3.1 筛选字段
扩展 `AuditLogListOptions`：
- Category（已有）
- Action、Username、TargetID：精确匹配
- Keyword：模糊匹配 `detail` / `target_name`
- DateFrom / DateTo：时间范围

### 3.2 CSV 导出
`GET /audit-logs/export?<filters>`：
- UTF-8 BOM + 逗号分隔，Excel 正确识别中文
- 文件名 `backupx-audit-YYYYMMDD-HHMMSS.csv`
- 最多 10000 行（防爆）
- 9 列：时间 / 用户 / 类别 / 动作 / 目标类型 / 目标 ID / 目标名 / 详情 / 客户端 IP

### 3.3 权限
审计日志本身就是所有角色可见（合规刚需知情权），导出沿用同权限。

### 3.4 前端
审计页新增：用户名输入 / 关键词输入 / 日期范围选择 / 查询 / 重置 / 导出 CSV

## 4. 能力三：K8s/Swarm 健康检查端点

### 4.1 端点
- `GET /health` 和 `/api/health`：liveness，只要进程响应就 200
- `GET /ready` 和 `/api/ready`：readiness，检查数据库 Ping；失败 503

### 4.2 无认证
两个端点公开：
- liveness 不做依赖检查，只保证"进程存活且可响应"
- readiness 检查 DB 连通性
- 输出字段：`status / version / uptime / checks / timestamp`

### 4.3 路径兼容
同时注册 `/health` 和 `/api/health`，方便反向代理按路径前缀统一转发。

## 5. 能力四：Dashboard 多维度统计

### 5.1 API
`GET /dashboard/breakdown?days=30` 返回：
- ByType：任务按类型分组（file / mysql / postgresql / sqlite / saphana）
- ByStatus：最近 N 天记录按状态（running / success / failed）
- ByNode：任务按执行节点分组
- ByStorage：按存储目标分组 + 累计字节数

### 5.2 实现要点
- 复用现有 `BackupTaskRepository.List` + `BackupRecordRepository.StorageUsage`
- `makeBreakdown` / `makeBreakdownByUint` 通用排序辅助函数
- 类型标签 Localize：`typeLabel("mysql") → "MySQL"` 直接给前端用

## 6. 数据迁移

无新表 / 无新字段。全部是后端新服务方法 + 前端新端点调用。

## 7. 双 review 通过

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 8. 未做（下一轮）

- Agent 自更新（远程分发二进制）
- WebSocket 实时 Dashboard 推送
- 备份加密密钥轮换
- PITR 增量备份
- SSO / OIDC
- 前端 Dashboard breakdown 可视化（饼图/柱状图）接入
- 存储容量 UI 展示（预警条形指示）
