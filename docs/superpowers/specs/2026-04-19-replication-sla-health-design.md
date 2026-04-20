# 设计文档：BackupX 企业级闭环 — 备份复制（3-2-1）+ SLA 监控 + 存储健康

- 日期：2026-04-19
- 范围：闭环第三轮 SLA + 实现 3-2-1 备份规则 + 存储目标主动监控
- 状态：已落地（loop 调度自主执行）

## 1. 目标

前四轮已完成：集群路由、验证演练、SLA 视图、RBAC、API Key、事件总线、节点配额。

企业级仍有缺口：
1. **SLA 监控只在 UI 显示**：违约不会主动告警，需要运维人工翻看
2. **缺 3-2-1 规则**：所有备份只有一份副本，不符合企业合规（SOC2/ISO27001 推荐 3 份副本、2 种介质、1 份异地）
3. **存储目标故障被动发现**：要等任务失败才知道云存储挂了

本轮闭环以上三个缺口。

## 2. 能力一：SLA 违约后台扫描

### 2.1 实现

`DashboardService.StartSLAMonitor(ctx, dispatcher, scanInterval, resetInterval)`：
- 每 `scanInterval`（15m）跑一次 `SLACompliance()`
- 违约任务派发 `sla_violation` 事件（复用 Notification 总线）
- 同任务在 `resetInterval`（6h）内不重复派发，避免骚扰
- 任务恢复合规后清除记忆，下次违约重新告警

### 2.2 状态机

```
normal → (超 RPO) → notified(首次派发) → (仍违约) → 沉默(resetInterval 内)
                                         → (resetInterval 过) → 再次派发
                 → (恢复成功) → normal(清除记忆)
```

## 3. 能力二：备份复制（3-2-1 规则）

### 3.1 模型

- `BackupTask.ReplicationTargetIDs` CSV：副本目标存储 ID 列表
- `ReplicationRecord` 独立表：记录每次复制执行（source → dest、状态、耗时、错误）

### 3.2 触发路径

**自动**（3-2-1 刚需）：
```
BackupExecutionService.executeTask 成功 →
  if len(task.ReplicationTargetIDs) > 0 →
    ReplicationService.TriggerAutoReplication(task, record) →
      foreach destID: s.Start(recordID, destID) → async 下载 + 上传
```

**手动**：前端备份记录详情点"复制"，`POST /backup/records/:id/replicate` 带 destTargetId。

### 3.3 核心实现

```go
func (s *ReplicationService) executeReplication(ctx, repID) {
    s.semaphore <- struct{}{}
    sourceProvider, _ := s.resolveProvider(ctx, rep.SourceTargetID)
    destProvider, _ := s.resolveProvider(ctx, rep.DestTargetID)
    
    reader, _ := sourceProvider.Download(ctx, rep.StoragePath)
    localPath := tmpDir + filepath.Base(rep.StoragePath)
    writeReaderToFile(localPath, reader)
    
    file, _ := os.Open(localPath)
    destProvider.Upload(ctx, rep.StoragePath, file, fileSize, meta)
    // 完成 → status = success；失败 → 派发 replication_failed 事件
}
```

### 3.4 集群保护

跨节点 local_disk 场景：源备份在 Agent 的本地磁盘，Master 取不到。与 BackupExecutionService.DownloadRecord 的保护一致，拒绝并返回明确错误。

### 3.5 数据库连接优化

Repository 使用 `SourceTarget`/`DestTarget` 两个不同 foreignKey → 一次查询返回完整信息，前端展示"源 → 目标"名称。

## 4. 能力三：存储目标健康监控

### 4.1 实现

`StorageTargetService.StartHealthMonitor(ctx, dispatcher, interval)`：
- 每 `interval`（5m）列出所有启用的 StorageTarget
- 逐个跑 `TestConnection()` → 更新 LastTestedAt/LastTestStatus
- 健康→故障边沿派发 `storage_unhealthy` 事件
- 故障→健康边沿清除 notified 记忆

### 4.2 设计权衡

- **同步串行扫描**：存储目标数量通常 < 20 个，串行简单可控
- **单次连接超时依赖 provider**：`TestConnection` 各 provider 自己控制（rclone 已有超时）
- **不阻塞存储配置操作**：后台独立 goroutine

## 5. 事件总线扩展

新增两个事件类型：
- `storage_unhealthy`：存储目标掉线
- `replication_failed`：复制失败
- `sla_violation`：SLA 违约（上轮已定义，本轮才有触发点）

## 6. 数据迁移

新增表：`replication_records`  
新增字段：`backup_tasks.replication_target_ids` (CSV)  
全 AutoMigrate，向后兼容（默认空 = 不启用复制）。

## 7. 前端

- **任务表单**新增"备份复制"步骤：副本目标多选（自动过滤掉已是主存储的目标）
- **新菜单**：`/replication/records` 展示复制历史（源/目标/状态/大小/耗时）
- **已有** LastTestStatus 展示在存储目标页，本轮后台扫描会自动更新此字段

## 8. 双 review 通过

- `go build ./...` ✅  `go vet ./...` ✅  `go test ./... -count=1` ✅
- `npx tsc --noEmit` ✅  `npm run build` ✅

## 9. 未做（下一轮）

- 备份窗口（maintenance window）：时段禁止调度
- Agent 自更新
- SSO / OIDC
- 报表 PDF/CSV 导出
- 复制选项：加密再上传、checksum 验证
- 任务模板（批量创建相似任务）
