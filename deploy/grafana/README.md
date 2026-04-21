# BackupX Grafana Dashboard

对接 BackupX v2.1+ 暴露的 Prometheus `/metrics` 端点。

## 导入步骤

1. 在 Grafana 配置 Prometheus 数据源指向你的 Prometheus（例如 `http://prometheus:9090`）
2. 在 Prometheus 配置抓取 BackupX：

```yaml
scrape_configs:
  - job_name: 'backupx'
    scrape_interval: 30s
    static_configs:
      - targets: ['backupx-master:8340']
```

3. Grafana → Dashboards → Import → 上传 `backupx-dashboard.json` → 选 Prometheus 数据源 → Import

## 面板内容

- 当前运行任务数 / SLA 违约数 / 在线节点 / 24h 成功率 / 应用版本
- 任务执行速率（按 success/failed 堆叠）
- 任务耗时 P50/P95/P99（按任务类型）
- 任务产出字节速率
- 存储目标用量 TopN 柱状图
- 节点在线状态表（红/绿标色）
- 验证 / 恢复 / 复制的成功率时间线

## 自定义建议

- 将 `backupx_sla_breach_tasks > 0` 配为 AlertManager 告警
- `sum(backupx_node_online) < N` 触发集群容量告警（N 为你集群的最少节点数）
- P99 任务耗时突变可用于发现慢任务和资源压力
