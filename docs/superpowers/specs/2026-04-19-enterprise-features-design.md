# 设计文档：BackupX 企业级产品化 — 验证演练 + SLA 监控 + 标签分组

- 日期：2026-04-19
- 范围：本轮交付三项核心企业级能力，闭环"可验证、可度量、可管理"
- 状态：已通过（用户授权自主执行）

## 1. 目标与非目标

### 目标
让 BackupX 从"能备份"升级为"**能保证恢复**、能**量化 SLA**、能**大规模管理**"的企业级备份管理平台。

### 非目标（本轮不做）
- RBAC 多用户角色（涉及所有接口重构，下轮单独做）
- Webhook 事件总线 / 对外 API Key 管理
- 异地镜像复制
- SSO / OIDC
- 合规报表导出

## 2. 能力一：备份验证 / 自动恢复演练

### 2.1 问题
绝大多数备份工具只保证"备份执行成功"，不保证"备份真的能恢复"。企业合规（SOC2、ISO27001、HIPAA）要求定期验证备份有效性。手动演练成本高，被普遍跳过。

### 2.2 设计

**模型**：`VerificationRecord`（独立表，参考 RestoreRecord 架构）
```
BackupRecordID  源备份记录
TaskID          关联任务
NodeID          在哪里执行（复用集群路由）
Status          running | success | failed
Mode            quick | deep     # quick=格式校验；deep=真恢复到沙箱
ErrorMessage
LogContent
DurationSeconds
StartedAt / CompletedAt
TriggeredBy     system(调度) / username(手动)
```

**验证策略（按任务类型）**：

| 类型 | quick 模式 | deep 模式（v2） |
|------|----------|---------------|
| file | 下载到沙箱 → tar header 遍历 + 记录中 SHA-256 比对 | + 解压到临时目录校验文件完整性 |
| sqlite | 下载 + `PRAGMA integrity_check` | + 打开查表 |
| mysql | dump 头部格式校验（`-- MySQL dump`） | + 导入到临时库 |
| postgresql | dump 头部格式校验（`PostgreSQL database dump`） | + 导入到临时库 |
| saphana | tar archive 解析 + 数据文件存在 | v2 |

**v1 实施 quick 模式**，deep 模式作为扩展点预留。

**BackupTask 扩展字段**：
```
VerifyEnabled   bool
VerifyCronExpr  string   # 独立 cron，如 "0 0 4 * * *"
VerifyMode      string   # quick（默认）
```

**调度**：复用现有 `scheduler.Service`，增加 `VerificationRunner` 接口（类似 TaskRunner），scheduler 内部再加一组 cron entries for verify。

**HTTP API**：
```
POST /backup/tasks/:id/verify            → 手动触发验证
GET  /verify/records                     → 列表
GET  /verify/records/:id                 → 详情
GET  /verify/records/:id/logs/stream     → SSE
```

**前端**：
- 任务表单增加 "验证与演练" 步骤（Cron + 启用开关）
- 新增 "验证记录" 页面（路由 /verify/records + 菜单）
- 任务详情页显示最近一次验证状态
- 失败则通知（复用通知服务）

**集群适配**：验证执行路由与备份恢复对称，任务绑定远程节点时通过 Agent 执行（复用 restore_record 路径的下载+解压能力，加入验证判定）。本轮 v1 先只在 Master 执行（下载远端备份文件本地验证）；远程 Agent 路由作为扩展点。

### 2.3 与备份恢复的区别
- **Verify 是只读的**：不覆盖任务源数据，只在隔离沙箱校验
- 失败不触发回滚机制，只记录并告警

## 3. 能力二：SLA 监控与告警规则

### 3.1 问题
当前 Dashboard 只显示历史指标，缺：
- **RPO 监控**：任务最长允许未备份间隔，超出则视为 SLA 违约
- **连续失败告警**：一次失败就告警会导致告警疲劳
- **静默时段**：维护窗口不触发告警

### 3.2 设计

**BackupTask 扩展字段**：
```
SLAHoursRPO            int    # 期望 RPO 小时数，0=不限
AlertOnConsecutiveFails int   # 连续失败 N 次才告警（默认 1）
```

**Dashboard 新增**：
- SLA 合规卡片：总任务数、合规/违约分布、违约任务清单
- 任务列表按"SLA 状态"着色（绿/黄/红）

**告警规则引擎**（扩展现有 notification）：
- 备份完成时检查：如果失败，查 task 的 `AlertOnConsecutiveFails` 和最近 N 条记录，判断是否达到阈值再发通知
- 后台监控：周期扫描所有任务，计算 `now - LastSuccessAt > SLAHoursRPO` → 触发 SLA 违约事件

**Dashboard API**：
```
GET /dashboard/sla  → {totalTasks, compliant, violated, violations: [{taskId, name, lastSuccessAt, hoursSinceLastSuccess, slaHours}]}
```

### 3.3 前端
- Dashboard 新增"SLA 合规"区块
- 任务列表新列"SLA 状态"
- 任务表单"存储与策略"步骤新增 SLA 配置

## 4. 能力三：任务分组 / 标签

### 4.1 问题
`BackupTask.Tags` 字段已存在但未激活；大规模（>50 任务）场景下难以管理。

### 4.2 设计

**Tags 语义**：逗号分隔字符串（沿用现有字段结构），前端用 InputTag 组件展示。

**新增能力**：
- 任务列表：按标签筛选 / 分组视图切换
- 批量操作：批量启停、批量立即执行、批量删除（已有部分批量端点，扩展）
- 标签建议：`GET /backup/tasks/tags`（去重返回全系统使用过的标签）

**前端**：
- 任务表单"基础信息"步骤新增标签输入（InputTag）
- 任务列表工具条新增"按标签筛选"多选
- 列表新增"标签"列（显示 Tag 芯片）
- 选中任务后悬浮"批量操作"工具条

## 5. 数据迁移

新增三字段（`VerifyEnabled` / `VerifyCronExpr` / `VerifyMode` / `SLAHoursRPO` / `AlertOnConsecutiveFails`）走 AutoMigrate。新增表 `verification_records` 走 AutoMigrate。

## 6. 双 review 目标

- `go build ./...` / `go vet ./...` / `go test ./... -count=1` 全绿
- `npx tsc --noEmit` / `npm run build` 通过
- 新增 3+ 单元测试：verification runner 策略、SLA 违约计算、标签筛选
- 所有新字段对非集群用户零影响（向后兼容）

## 7. 实施顺序

1. 备份验证模型 + 仓储 + VerificationService（本地执行策略）
2. 任务字段迁移 + 调度器 verify 入口 + HTTP handler
3. 前端 verify 配置步骤 + 记录页 + 路由/菜单
4. SLA 字段迁移 + Dashboard SLA API + 告警阈值逻辑
5. 前端 Dashboard SLA 卡片 + 任务表单 SLA 配置
6. 标签：前端 InputTag + 筛选 + 分组视图 + 批量操作
7. 单元测试 + 全链路 review
