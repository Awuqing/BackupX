---
sidebar_position: 3
title: Monitoring and Alerts
description: Health probes, Prometheus metrics, initial alert rules, and operational validation.
---

# Monitoring and Alerts

BackupX exposes low-cost health endpoints and a dedicated Prometheus registry. Monitor both the control plane and the outcome of backup, restore, verification, and replication work.

## Probes

| Endpoint | Meaning | Expected response |
| --- | --- | --- |
| `/health` | Liveness: the HTTP process can respond | HTTP 200 with `status: live` |
| `/ready` | Readiness: the process can reach SQLite | HTTP 200 with `status: ready`; HTTP 503 on database failure |
| `/api/health` | API-prefixed alias for liveness | Same as `/health` |
| `/api/ready` | API-prefixed alias for readiness | Same as `/ready` |
| `/metrics` | Prometheus exposition | HTTP 200 when metrics are enabled |

Use `/health` for a liveness probe and `/ready` for readiness or load-balancer traffic decisions. Do not restart a process only because an external storage provider is unavailable; storage health belongs in task and target alerts.

~~~bash
curl -fsS http://127.0.0.1:8340/health
curl -fsS http://127.0.0.1:8340/ready
curl -fsS http://127.0.0.1:8340/metrics | head
~~~

These endpoints are unauthenticated. Restrict them to orchestrator and monitoring networks.

## Prometheus scrape

~~~yaml
scrape_configs:
  - job_name: backupx
    scheme: https
    metrics_path: /metrics
    static_configs:
      - targets: [backup.example.com]
~~~

When Nginx terminates TLS, allow the Prometheus source address to reach `/metrics` and deny other public clients. The internal collector refreshes storage, node, command-queue, and SLA gauges every 30 seconds.

## BackupX metrics

| Metric | Type | Labels | Purpose |
| --- | --- | --- | --- |
| `backupx_app_info` | gauge | `version` | Running release metadata |
| `backupx_task_run_total` | counter | `status`, `task_type` | Backup outcomes |
| `backupx_task_run_duration_seconds` | histogram | `task_type` | Backup duration distribution |
| `backupx_task_bytes_total` | counter | `task_type` | Produced backup bytes |
| `backupx_task_running` | gauge | none | Current backup concurrency |
| `backupx_storage_used_bytes` | gauge | `target_name`, `target_type` | Recorded usage per target |
| `backupx_node_online` | gauge | `node_name`, `role` | Node online state, 1 or 0 |
| `backupx_agent_command_queue_depth` | gauge | `node_name`, `role` | Pending and dispatched commands |
| `backupx_agent_command_running` | gauge | `node_name`, `role` | Long-running Agent commands |
| `backupx_agent_command_timeout_total` | gauge | `node_name`, `role` | Snapshot of timed-out commands |
| `backupx_verify_run_total` | counter | `status` | Verification outcomes |
| `backupx_restore_run_total` | counter | `status` | Restore outcomes |
| `backupx_replication_run_total` | counter | `status` | Replication outcomes |
| `backupx_sla_breach_tasks` | gauge | none | Enabled tasks outside their configured RPO |

Standard Go runtime and process collectors are registered in the same endpoint.

## Initial alert rules

Tune windows and thresholds to the schedules and RPOs of each environment:

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

The `BackupXNotReady` example assumes a blackbox probe job named `backupx-ready`. If no blackbox exporter is used, alert from the load balancer or orchestrator readiness signal instead.

## Operational dashboard

Track these views together:

- Success and failure rate by task type.
- P50, P95, and maximum run duration relative to the backup window.
- Bytes produced compared with the expected data-change rate.
- Current running tasks versus `backup.max_concurrent`.
- Offline Agents, queue depth, running commands, and timeout-count changes.
- Storage growth, free capacity from the storage provider, and retention cleanup.
- SLA breach count and age of the most recent successful backup for critical tasks.
- Verification, restore, and replication success rates.

Prometheus storage usage is based on BackupX record metadata, not necessarily the provider's billable capacity. Monitor provider quota and filesystem free space separately.

## Post-deployment validation

After installation, upgrade, proxy changes, or recovery:

1. Check liveness and readiness locally and through the public proxy.
2. Confirm Prometheus sees one active Master and the expected version label.
3. Verify every expected Agent reports `backupx_node_online == 1`.
4. Run a small backup and confirm the success counter increases.
5. Run a verification or isolated restore and confirm its counter increases.
6. Trigger a test notification and verify the alert delivery path.

Continue with [Troubleshooting](./troubleshooting) when a probe or metric is abnormal.
