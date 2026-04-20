# v2.1.0 可观测性与流控设计 (2026-04-19)

## 背景

v2.0.0 交付了 11 项企业能力（RBAC / API Key / 多节点集群 / 3-2-1 复制 / 验证演练等），产品具备"企业级备份管理平台"的完整能力。v2.1.0 聚焦 "**投入生产后运维团队**" 的两类刚需：

1. **可观测性**：SRE 要能把 BackupX 接入 Prometheus/Grafana 做容量规划与告警。
2. **流控精细化**：不同节点的带宽/并发应该能各自配置，而不是一刀切。
3. **审计外输**：合规团队需要把审计事件送到 SIEM / WORM 存储，实现集中留档。

## 范围

**In scope：**

- `/metrics` Prometheus 端点（10+ 核心指标）
- 节点级带宽限速生效（`model.Node.BandwidthLimit` 已存在但未落地）
- 审计日志 Webhook 外输（HMAC-SHA256 签名）

**Out of scope（放入后续迭代）：**

- Prometheus 鉴权（企业生产可用反向代理做）
- Grafana Dashboard JSON 预置
- 节点级并发已在 v2.0 完成，不再重复
- 审计事件的 Syslog/Kafka 渠道（Webhook 已能衔接 Fluent Bit）
- 前端 Settings 页 UI（可 API 配置，UI 后续补）

## 架构

### 1. Prometheus /metrics

```
业务服务                    metrics.Metrics                 /metrics HTTP
─────────                  ────────────────                ──────────────
BackupExec  ─ObserveRun──► Counter+Histogram ◄─Scrape── Prometheus
Restore     ─ObserveRun──►
Verify      ─ObserveRun──►
Replication ─ObserveRun──►
                           Gauge (storage/node/SLA)
Collector(30s) ─update───► ▲
                           │
                           repo.StorageUsage / Node.List / Task.List
```

- **独立 Registry**：避免与 default registry 中的默认 metrics 混淆，只暴露 backupx_ + go_ + process_
- **零值安全**：`*Metrics` nil 时所有方法静默退化，不影响未注入 metrics 的单测
- **Gauge 异步刷新**：30s 后台 goroutine 采集慢查询数据，避免阻塞 /metrics 请求
- **Counter/Histogram 同步**：任务完成时直接 Observe，延迟 < 1μs

指标清单：

| 指标 | 类型 | 标签 | 含义 |
|------|------|------|------|
| `backupx_task_run_total` | Counter | status, task_type | 备份任务运行计数 |
| `backupx_task_run_duration_seconds` | Histogram | task_type | 任务耗时分布 |
| `backupx_task_bytes_total` | Counter | task_type | 累计产出字节数 |
| `backupx_task_running` | Gauge | - | 正在运行任务数 |
| `backupx_storage_used_bytes` | Gauge | target_name, target_type | 存储目标用量 |
| `backupx_node_online` | Gauge | node_name, role | 节点在线状态 |
| `backupx_verify_run_total` | Counter | status | 验证演练计数 |
| `backupx_restore_run_total` | Counter | status | 恢复操作计数 |
| `backupx_replication_run_total` | Counter | status | 副本复制计数 |
| `backupx_sla_breach_tasks` | Gauge | - | 违反 SLA 任务数 |
| `backupx_app_info` | Gauge | version | 应用版本（恒为 1） |

### 2. 节点级带宽限速

现状：`BackupExecutionService` 在 `resolveProvider()` 中用全局 `s.bandwidthLimit`（来自 `cfg.Backup.BandwidthLimit`）注入 rclone TransferConfig。

改进：新增 `resolveProviderForNode(ctx, targetID, nodeID)`：

```go
func (s *BackupExecutionService) effectiveBandwidth(ctx context.Context, nodeID uint) string {
    if nodeID == 0 || s.nodeRepo == nil {
        return s.bandwidthLimit
    }
    node, err := s.nodeRepo.FindByID(ctx, nodeID)
    if err != nil || node == nil {
        return s.bandwidthLimit
    }
    if strings.TrimSpace(node.BandwidthLimit) != "" {
        return node.BandwidthLimit
    }
    return s.bandwidthLimit
}
```

优先级：`Node.BandwidthLimit` > 全局默认。仅 Master 本地执行生效；Agent 使用自身 Node 配置（在 Agent runtime 中独立应用）。

### 3. 审计 Webhook

```
AuditService.Record(entry)
   │
   ├─> repo.Create (写 DB)                     [fire-and-forget]
   └─> fireWebhook(record)                     [fire-and-forget]
         │
         ├─ HTTP POST JSON to webhookURL
         ├─ Header: X-BackupX-Signature: sha256=<hmac>
         └─ 失败: log.Printf，不阻塞主流程
```

Payload schema：

```json
{
  "eventType": "audit.log",
  "occurredAt": "2026-04-19T10:30:00Z",
  "actor": { "userId": 1, "username": "alice" },
  "category": "auth",
  "action": "login_success",
  "targetType": "user",
  "targetId": "1",
  "targetName": "alice",
  "detail": "admin login",
  "clientIp": "10.0.0.1"
}
```

签名：`HMAC-SHA256(secret, raw_json_body)`，接收方需要验证以防伪造。

配置路径：前端通过 `PUT /api/settings` 写入 `audit_webhook_url` / `audit_webhook_secret`，SettingsService 保存后立即通过 `AuditWebhookConfigurer` 接口同步到 AuditService，无需重启。

## 测试

- `metrics/registry_test.go` — 注册、采集、nil safety、HTTP handler 端到端
- `service/audit_service_webhook_test.go` — 签名正确性、异步发送、禁用路径
- 所有现有测试保持通过（backup_execution_service_test / restore_service_test / verification_service_test）

## 风险与应对

| 风险 | 应对 |
|------|------|
| Prometheus 采集阻塞 | Gauge 走后台 Collector + Counter/Histogram 是内存操作，无 IO |
| Webhook 打爆业务 | 3s 超时 + fire-and-forget goroutine，单次 panic 也不影响主流程 |
| 指标基数爆炸 | task_name 不作为 label（仅 task_type），避免 Prometheus series 失控 |
| 节点带宽配置错误 | 走 rclone.BwTimetable.Set 校验，解析失败静默沿用全局默认 |

## 部署建议

- Prometheus 抓取配置：`scrape_interval: 30s`，匹配 Collector 间隔
- Grafana alert 示例：`sum(backupx_sla_breach_tasks) > 0` 触发
- Webhook 接收侧建议：Fluent Bit HTTP input → Elasticsearch / Loki
