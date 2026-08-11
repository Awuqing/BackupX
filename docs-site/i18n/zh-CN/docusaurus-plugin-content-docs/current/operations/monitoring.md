---
sidebar_position: 3
title: 监控与告警
description: 健康探针、Prometheus 指标、初始告警规则和运维验证。
---

# 监控与告警

BackupX 提供低开销健康端点和独立 Prometheus Registry。监控既要覆盖控制面，也要覆盖备份、恢复、验证和复制的实际结果。

## 探针

| 端点 | 含义 | 预期响应 |
| --- | --- | --- |
| `/health` | 存活：HTTP 进程可响应 | HTTP 200，`status: live` |
| `/ready` | 就绪：进程可访问 SQLite | 正常为 HTTP 200 与 `status: ready`；数据库故障为 HTTP 503 |
| `/api/health` | 带 API 前缀的存活别名 | 与 `/health` 相同 |
| `/api/ready` | 带 API 前缀的就绪别名 | 与 `/ready` 相同 |
| `/metrics` | Prometheus 指标 | 指标启用时为 HTTP 200 |

`/health` 用作 liveness，`/ready` 用作 readiness 或负载均衡流量判断。外部存储暂时不可用不应直接触发进程重启，应通过任务和存储目标告警处理。

~~~bash
curl -fsS http://127.0.0.1:8340/health
curl -fsS http://127.0.0.1:8340/ready
curl -fsS http://127.0.0.1:8340/metrics | head
~~~

这些端点不需要认证，只允许编排器和监控网段访问。

## Prometheus 抓取

~~~yaml
scrape_configs:
  - job_name: backupx
    scheme: https
    metrics_path: /metrics
    static_configs:
      - targets: [backup.example.com]
~~~

Nginx 终止 TLS 时，应只放行 Prometheus 源地址访问 `/metrics`。内部采集器每 30 秒刷新存储、节点、命令队列和 SLA Gauge。

## BackupX 指标

| 指标 | 类型 | 标签 | 用途 |
| --- | --- | --- | --- |
| `backupx_app_info` | gauge | `version` | 当前版本元数据 |
| `backupx_task_run_total` | counter | `status`、`task_type` | 备份结果 |
| `backupx_task_run_duration_seconds` | histogram | `task_type` | 备份耗时分布 |
| `backupx_task_bytes_total` | counter | `task_type` | 备份产出字节数 |
| `backupx_task_running` | gauge | 无 | 当前备份并发 |
| `backupx_storage_used_bytes` | gauge | `target_name`、`target_type` | 按目标记录的使用量 |
| `backupx_node_online` | gauge | `node_name`、`role` | 节点在线状态，1 或 0 |
| `backupx_agent_command_queue_depth` | gauge | `node_name`、`role` | 待处理与已派发命令 |
| `backupx_agent_command_running` | gauge | `node_name`、`role` | Agent 长任务数 |
| `backupx_agent_command_timeout_total` | gauge | `node_name`、`role` | 超时命令数快照 |
| `backupx_verify_run_total` | counter | `status` | 验证结果 |
| `backupx_restore_run_total` | counter | `status` | 恢复结果 |
| `backupx_replication_run_total` | counter | `status` | 复制结果 |
| `backupx_sla_breach_tasks` | gauge | 无 | 超出已配置 RPO 的启用任务数 |

同一端点还注册了标准 Go Runtime 与进程指标。

## 初始告警规则

应根据各环境计划与 RPO 调整窗口和阈值：

~~~yaml
groups:
  - name: backupx
    rules:
      - alert: BackupXTargetDown
        expr: up{job="backupx"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: BackupX metrics endpoint is unreachable

      - alert: BackupXNotReady
        expr: probe_success{job="backupx-ready"} == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: BackupX readiness check is failing

      - alert: BackupXBackupFailure
        expr: sum(increase(backupx_task_run_total{status="failed"}[15m])) > 0
        labels:
          severity: warning
        annotations:
          summary: A BackupX backup failed

      - alert: BackupXSLABreach
        expr: backupx_sla_breach_tasks > 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: One or more backup tasks are outside RPO

      - alert: BackupXAgentOffline
        expr: backupx_node_online{role="agent"} == 0
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: BackupX Agent is offline

      - alert: BackupXAgentQueueBacklog
        expr: backupx_agent_command_queue_depth > 20
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: BackupX Agent command queue is growing
~~~

`BackupXNotReady` 示例假定存在名为 `backupx-ready` 的 Blackbox 探针任务。未部署 Blackbox Exporter 时，应改用负载均衡或编排器的 readiness 信号。

## 运维仪表盘

建议同时展示：

- 按任务类型统计成功率与失败率。
- P50、P95、最大执行时长及其与备份窗口的关系。
- 产出字节数与预期数据变化率。
- 当前任务数与 `backup.max_concurrent`。
- 离线 Agent、队列深度、运行命令和超时数变化。
- 存储增长、提供商剩余容量和保留策略清理。
- SLA 违约数及关键任务最近成功备份时间。
- 验证、恢复和复制成功率。

Prometheus 存储使用量来自 BackupX 记录元数据，不一定等同于提供商计费容量，应另行监控提供商配额和文件系统剩余空间。

## 部署后验证

安装、升级、代理变更或恢复后：

1. 分别从本机和公开代理检查存活与就绪。
2. 确认 Prometheus 只看到一个活动 Master，并带有预期版本标签。
3. 确认所有预期 Agent 的 `backupx_node_online == 1`。
4. 执行小型备份并确认成功 Counter 增长。
5. 执行验证或隔离恢复并确认对应 Counter 增长。
6. 触发测试通知并验证告警投递链路。

探针或指标异常时继续参考[故障排查](./troubleshooting)。
