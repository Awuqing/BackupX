package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"backupx/server/internal/backup"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	storageRclone "backupx/server/internal/storage/rclone"
)

// fakeDispatcher 捕获入队调用，用于验证远程路由。
type fakeDispatcher struct {
	mu    sync.Mutex
	calls []dispatcherCall
}

type dispatcherCall struct {
	NodeID  uint
	CmdType string
	Payload map[string]any
}

func (f *fakeDispatcher) EnqueueCommand(_ context.Context, nodeID uint, cmdType string, payload any) (uint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, _ := json.Marshal(payload)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	f.calls = append(f.calls, dispatcherCall{NodeID: nodeID, CmdType: cmdType, Payload: m})
	return uint(len(f.calls)), nil
}

func (f *fakeDispatcher) snapshot() []dispatcherCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]dispatcherCall, len(f.calls))
	copy(out, f.calls)
	return out
}

type restoreTestHarness struct {
	service    *RestoreService
	execution  *BackupExecutionService
	records    repository.BackupRecordRepository
	restores   repository.RestoreRecordRepository
	tasks      repository.BackupTaskRepository
	nodes      repository.NodeRepository
	dispatcher *fakeDispatcher
	sourceDir  string
	storageDir string
}

func newRestoreTestHarness(t *testing.T, remoteNode bool) *restoreTestHarness {
	t.Helper()
	baseDir := t.TempDir()
	sourceDir := filepath.Join(baseDir, "source")
	storageDir := filepath.Join(baseDir, "storage")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello-restore"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	cipher := codec.NewConfigCipher("restore-secret")
	targets := repository.NewStorageTargetRepository(db)
	tasks := repository.NewBackupTaskRepository(db)
	records := repository.NewBackupRecordRepository(db)
	restores := repository.NewRestoreRecordRepository(db)
	nodes := repository.NewNodeRepository(db)
	targetCipher, err := cipher.EncryptJSON(map[string]any{"basePath": storageDir})
	if err != nil {
		t.Fatalf("EncryptJSON: %v", err)
	}
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "local", Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: targetCipher, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("create target: %v", err)
	}

	// 构造本机节点（始终存在）+ 可选远程节点
	localNode := &model.Node{Name: "local", Token: "local-token", Status: model.NodeStatusOnline, IsLocal: true, LastSeen: time.Now().UTC()}
	if err := db.Create(localNode).Error; err != nil {
		t.Fatalf("seed local node: %v", err)
	}
	taskNodeID := uint(0)
	if remoteNode {
		remote := &model.Node{Name: "edge-1", Token: "remote-token", Status: model.NodeStatusOnline, IsLocal: false, LastSeen: time.Now().UTC()}
		if err := db.Create(remote).Error; err != nil {
			t.Fatalf("seed remote node: %v", err)
		}
		taskNodeID = remote.ID
	}

	task := &model.BackupTask{Name: "restore-test", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, NodeID: taskNodeID, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}
	if err := tasks.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewMySQLRunner(nil), backup.NewSQLiteRunner(), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())

	execution := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, nil, cipher, nil, baseDir, 2, 10, "")
	dispatcher := &fakeDispatcher{}
	restoreLogHub := backup.NewLogHub()
	restoreService := NewRestoreService(restores, records, tasks, targets, nodes, storageRegistry, runnerRegistry, restoreLogHub, cipher, dispatcher, baseDir, 2)

	return &restoreTestHarness{
		service:    restoreService,
		execution:  execution,
		records:    records,
		restores:   restores,
		tasks:      tasks,
		nodes:      nodes,
		dispatcher: dispatcher,
		sourceDir:  sourceDir,
		storageDir: storageDir,
	}
}

func TestRestoreServiceStart_LocalNodeExecutesInline(t *testing.T) {
	h := newRestoreTestHarness(t, false)
	ctx := context.Background()

	// 先跑一次备份产出源备份记录
	backupDetail, err := h.execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}
	if backupDetail.Status != "success" {
		t.Fatalf("expected backup success, got %s", backupDetail.Status)
	}

	// 清空源目录，期望 restore 把它还原
	if err := os.RemoveAll(h.sourceDir); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	// 用同步 async 让测试可等待
	done := make(chan struct{})
	h.service.async = func(job func()) {
		go func() {
			job()
			close(done)
		}()
	}
	detail, err := h.service.Start(ctx, backupDetail.ID, "tester")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if detail.Status != model.RestoreRecordStatusRunning {
		t.Fatalf("expected initial status running, got %s", detail.Status)
	}
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("restore did not complete in time")
	}

	final, err := h.service.Get(ctx, detail.ID)
	if err != nil {
		t.Fatalf("Get final: %v", err)
	}
	if final.Status != model.RestoreRecordStatusSuccess {
		t.Fatalf("expected success, got %s (err=%s)", final.Status, final.ErrorMessage)
	}
	if final.TriggeredBy != "tester" {
		t.Fatalf("expected triggeredBy=tester, got %q", final.TriggeredBy)
	}
	content, err := os.ReadFile(filepath.Join(h.sourceDir, "index.html"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(content) != "hello-restore" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}
	if len(h.dispatcher.snapshot()) != 0 {
		t.Fatalf("expected no dispatcher calls for local node, got %d", len(h.dispatcher.snapshot()))
	}
}

func TestRestoreServiceStart_RemoteNodeEnqueuesCommand(t *testing.T) {
	h := newRestoreTestHarness(t, true)
	ctx := context.Background()

	// 先在本地执行一次备份（备份路由对 RestoreService 无影响，仅用来生成源记录）
	// 备份执行服务的 isRemoteNode 同样走 nodeRepo，但因为 execution.SetClusterDependencies 未注入，
	// 会被判定为本地执行 → 测试保持纯粹。
	backupDetail, err := h.execution.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync: %v", err)
	}

	detail, err := h.service.Start(ctx, backupDetail.ID, "tester-remote")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if detail.Status != model.RestoreRecordStatusRunning {
		t.Fatalf("expected running, got %s", detail.Status)
	}
	calls := h.dispatcher.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 dispatcher call, got %d", len(calls))
	}
	if calls[0].CmdType != model.AgentCommandTypeRestoreRecord {
		t.Fatalf("expected cmdType %s, got %s", model.AgentCommandTypeRestoreRecord, calls[0].CmdType)
	}
	if rid, ok := calls[0].Payload["restoreRecordId"].(float64); !ok || uint(rid) != detail.ID {
		t.Fatalf("expected restoreRecordId=%d in payload, got %#v", detail.ID, calls[0].Payload)
	}
}

func TestRestoreServiceStart_UsesBackupRecordNodeForPooledTask(t *testing.T) {
	h := newRestoreTestHarness(t, true)
	ctx := context.Background()

	task, err := h.tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task: %v", err)
	}
	remoteNodeID := task.NodeID
	task.NodeID = 0
	task.NodePoolTag = "db"
	if err := h.tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update task: %v", err)
	}
	storedTask, err := h.tasks.FindByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("FindByID stored task: %v", err)
	}
	if storedTask.NodeID != 0 {
		t.Fatalf("expected stored task NodeID to be reset to 0, got %d", storedTask.NodeID)
	}

	startedAt := time.Now().UTC()
	completedAt := startedAt.Add(time.Second)
	backupRecord := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		NodeID:          remoteNodeID,
		Status:          model.BackupRecordStatusSuccess,
		FileName:        "pooled.tar.gz",
		StoragePath:     "file/2026/05/09/pooled.tar.gz",
		StartedAt:       startedAt,
		CompletedAt:     &completedAt,
	}
	if err := h.records.Create(ctx, backupRecord); err != nil {
		t.Fatalf("Create backup record: %v", err)
	}

	detail, err := h.service.Start(ctx, backupRecord.ID, "tester-pool")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if detail.NodeID != remoteNodeID {
		t.Fatalf("expected restore node %d, got %d", remoteNodeID, detail.NodeID)
	}
	calls := h.dispatcher.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly 1 dispatcher call, got %d", len(calls))
	}
	if calls[0].NodeID != remoteNodeID {
		t.Fatalf("expected dispatch to node %d, got %d", remoteNodeID, calls[0].NodeID)
	}
}

func TestRestoreServiceAgentRestoreAccessUsesRestoreRecordNode(t *testing.T) {
	h := newRestoreTestHarness(t, true)
	ctx := context.Background()

	task, err := h.tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task: %v", err)
	}
	owner, err := h.nodes.FindByID(ctx, task.NodeID)
	if err != nil {
		t.Fatalf("FindByID owner node: %v", err)
	}
	other := &model.Node{Name: "edge-2", Token: "other-token", Status: model.NodeStatusOnline, IsLocal: false, LastSeen: time.Now().UTC()}
	if err := h.nodes.Create(ctx, other); err != nil {
		t.Fatalf("Create other node: %v", err)
	}
	startedAt := time.Now().UTC()
	completedAt := startedAt.Add(time.Second)
	backupRecord := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		NodeID:          owner.ID,
		Status:          model.BackupRecordStatusSuccess,
		FileName:        "remote.tar.gz",
		StoragePath:     "file/2026/05/09/remote.tar.gz",
		StartedAt:       startedAt,
		CompletedAt:     &completedAt,
	}
	if err := h.records.Create(ctx, backupRecord); err != nil {
		t.Fatalf("Create backup record: %v", err)
	}
	restore := &model.RestoreRecord{
		BackupRecordID: backupRecord.ID,
		TaskID:         task.ID,
		NodeID:         owner.ID,
		Status:         model.RestoreRecordStatusRunning,
		StartedAt:      startedAt,
		TriggeredBy:    "agent-test",
	}
	if err := h.restores.Create(ctx, restore); err != nil {
		t.Fatalf("Create restore record: %v", err)
	}

	spec, err := h.service.GetAgentRestoreSpec(ctx, owner, restore.ID)
	if err != nil {
		t.Fatalf("owner GetAgentRestoreSpec returned error: %v", err)
	}
	if spec.RestoreRecordID != restore.ID || spec.StoragePath != backupRecord.StoragePath {
		t.Fatalf("unexpected restore spec: %#v", spec)
	}
	if _, err := h.service.GetAgentRestoreSpec(ctx, other, restore.ID); err == nil {
		t.Fatal("expected non-owner node to be forbidden from restore spec")
	}
	if err := h.service.UpdateAgentRestore(ctx, owner, restore.ID, AgentRestoreUpdate{
		Status:    model.RestoreRecordStatusSuccess,
		LogAppend: "done\n",
	}); err != nil {
		t.Fatalf("owner UpdateAgentRestore returned error: %v", err)
	}
	updated, err := h.restores.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID restore returned error: %v", err)
	}
	if updated.Status != model.RestoreRecordStatusSuccess || updated.NodeID != owner.ID {
		t.Fatalf("unexpected updated restore record: %#v", updated)
	}
	if err := h.service.UpdateAgentRestore(ctx, other, restore.ID, AgentRestoreUpdate{LogAppend: "bad\n"}); err == nil {
		t.Fatal("expected non-owner node to be forbidden from restore update")
	}
}

func TestRestoreServiceUpdateAgentRestoreDoesNotOverwriteTerminalRecord(t *testing.T) {
	h := newRestoreTestHarness(t, true)
	ctx := context.Background()

	task, err := h.tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task: %v", err)
	}
	owner, err := h.nodes.FindByID(ctx, task.NodeID)
	if err != nil {
		t.Fatalf("FindByID owner node: %v", err)
	}
	startedAt := time.Now().UTC().Add(-time.Hour)
	completedAt := time.Now().UTC().Add(-time.Minute)
	restore := &model.RestoreRecord{
		BackupRecordID: 1,
		TaskID:         task.ID,
		NodeID:         owner.ID,
		Status:         model.RestoreRecordStatusFailed,
		ErrorMessage:   "timeout",
		StartedAt:      startedAt,
		CompletedAt:    &completedAt,
		TriggeredBy:    "agent-test",
	}
	if err := h.restores.Create(ctx, restore); err != nil {
		t.Fatalf("Create restore record: %v", err)
	}

	if err := h.service.UpdateAgentRestore(ctx, owner, restore.ID, AgentRestoreUpdate{
		Status:       model.RestoreRecordStatusSuccess,
		ErrorMessage: "late success",
		LogAppend:    "late log\n",
	}); err != nil {
		t.Fatalf("UpdateAgentRestore returned error: %v", err)
	}

	updated, err := h.restores.FindByID(ctx, restore.ID)
	if err != nil {
		t.Fatalf("FindByID restore returned error: %v", err)
	}
	if updated.Status != model.RestoreRecordStatusFailed {
		t.Fatalf("expected terminal restore status to remain failed, got %#v", updated)
	}
	if updated.ErrorMessage != "timeout" {
		t.Fatalf("expected terminal restore error to remain unchanged, got %q", updated.ErrorMessage)
	}
}

func TestRestoreServiceStart_FailsOnNonSuccessBackup(t *testing.T) {
	h := newRestoreTestHarness(t, false)
	ctx := context.Background()

	// 手动构造一条 failed 状态的备份记录
	startedAt := time.Now().UTC()
	failed := &model.BackupRecord{
		TaskID:          1,
		StorageTargetID: 1,
		Status:          model.BackupRecordStatusFailed,
		FileName:        "never.tar.gz",
		StoragePath:     "tasks/1/never.tar.gz",
		StartedAt:       startedAt,
	}
	if err := h.records.Create(ctx, failed); err != nil {
		t.Fatalf("create failed record: %v", err)
	}

	if _, err := h.service.Start(ctx, failed.ID, "tester"); err == nil {
		t.Fatalf("expected error when restoring from failed backup, got nil")
	}
}
