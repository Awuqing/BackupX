# v2.2.0 节点池与可视化运维闭环 (2026-04-21)

## 背景

v2.1 暴露了 Prometheus `/metrics` + 节点级带宽限速，SRE 已经拿到"看"的能力。本轮补齐"调度"和"可视化"的闭环：

1. **节点池**：任务不再只能绑定固定节点，还可以按标签动态调度
2. **Grafana Dashboard**：v2.1 指标从"裸数据"升级为开箱即用的运维视图
3. **Agent 版本漂移 UI**：节点列表一眼看出哪台 Agent 落后于 Master

## 范围

- `model.Node.Labels` (CSV) + `model.BackupTask.NodePoolTag`
- `BackupExecutionService.selectPoolNode()` — 标签匹配 + 当前运行任务数最少原则
- `deploy/grafana/backupx-dashboard.json` — 11 面板对接 v2.1 指标
- 前端节点列表显示版本漂移 + 标签/池；任务表单支持节点池输入

## 架构

### 1. 节点池调度

```
task.NodeID == 0 && task.NodePoolTag != ""
        ↓
selectPoolNode(ctx, tag):
    1. nodeRepo.List() 过滤 status=online AND HasLabel(tag)
    2. 无候选 → 返回 NODE_POOL_EMPTY 错误（任务失败，用户立即感知）
    3. 按 countRunningOnNode(id) 升序选最小负载者
    4. 并列按 ID 稳定（可预期）
        ↓
record.NodeID = chosen.ID  （仅本次运行，不回写 task）
task.NodeID = chosen.ID    （供后续 route/agent 路由逻辑使用）
```

**互斥规则**：`NodeID > 0` 与 `NodePoolTag != ""` 在 Create/Update 校验中被拒绝（`BACKUP_TASK_INVALID`）。固定节点 = 显式路由，节点池 = 动态路由，两者语义互斥。

**调度不回写 task**：池选出的节点 ID 仅写入 BackupRecord（审计追溯），task.NodeID 仍为 0。这样下次执行会**重新选**负载最低者，实现真正的轮转均衡。

### 2. Grafana Dashboard

11 个面板，按语义分组：

| 区域 | 面板 |
|------|------|
| 概览（4 stat） | 运行中任务数 / SLA 违约数 / 在线节点数 / 24h 成功率 / 应用版本 |
| 时序（4） | 任务执行速率（按状态堆叠）、P50/P95/P99 耗时、产出字节速率、验证/恢复/复制成功率 |
| 容量（1） | 存储目标用量 TopN 柱状图 |
| 集群（1） | 节点在线状态表（值 0/1 → 红/绿色文本映射） |

设计要点：
- `DS_PROMETHEUS` 为 template variable，导入时让用户选数据源
- 默认 refresh `30s`，与服务端 collector 采样周期一致
- SLA 违约 stat 阈值 ≥1 即红色，直接可接 AlertManager

### 3. Agent 版本漂移 UI

```
renderAgentVersion(agentVer, masterVer):
    空 agentVer → "-"（未上报）
    agentVer == masterVer → 原样显示
    不同 → 橙红 Tag "<agentVer> ≠ <masterVer>" + Tooltip 建议升级
```

`masterVer` 通过 `/api/system/info` 已有接口获得，前端无需新增 API。

## 测试

- `model/node_label_test.go` — HasLabel / LabelSet / nil 安全
- `service/node_pool_scheduler_test.go` — 负载最低 / 空池报错 / nil repo 降级
- 前端 `npm run build` 通过

## 风险与应对

| 风险 | 应对 |
|------|------|
| 节点池在所有节点离线时任务失败 | 明确返回 `NODE_POOL_EMPTY`，用户立即感知并切换固定节点 |
| 运行任务数统计成瓶颈 | countRunningOnNode 走 BackupRecord.List({status:running})，规模大时可引入节点级 semaphore 计数器 |
| Labels 格式笔误（重复、空格） | `normalizeLabels` 规整 CSV 再入库；前端 Tag 渲染自动 trim |
| 版本漂移 UI 误报 dev 分支 | Master 版本取自 main.version ldflags；dev 构建显示 "dev"，不会匹配任何 Agent 版本，纯显示意义 |

## 下轮候选

- Agent 二进制自更新（远程下发 + 签名验证）
- 任务运行前的"节点可达性预检"（TCP/HTTP 探针）
- Grafana Loki 集成：把 backup 日志流接入 Loki，配合 Tempo 做端到端追踪
