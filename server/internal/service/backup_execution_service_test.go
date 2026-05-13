package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"backupx/server/internal/backup"
	backupretention "backupx/server/internal/backup/retention"
	"backupx/server/internal/config"
	"backupx/server/internal/database"
	"backupx/server/internal/logger"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
	storageRclone "backupx/server/internal/storage/rclone"
)

type testStorageFactory struct {
	providers map[string]*testStorageProvider
}

func (f *testStorageFactory) Type() storage.ProviderType {
	return "test_storage"
}

func (f *testStorageFactory) New(_ context.Context, config map[string]any) (storage.StorageProvider, error) {
	name, _ := config["name"].(string)
	provider := f.providers[name]
	if provider == nil {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return provider, nil
}

type testStorageProvider struct {
	name        string
	failUpload  bool
	blockUpload <-chan struct{}
	onUpload    func()
	objects     map[string][]byte
}

func (p *testStorageProvider) Type() storage.ProviderType { return "test_storage" }
func (p *testStorageProvider) TestConnection(context.Context) error {
	return nil
}
func (p *testStorageProvider) Upload(_ context.Context, objectKey string, reader io.Reader, _ int64, _ map[string]string) error {
	if p.blockUpload != nil {
		<-p.blockUpload
	}
	if p.onUpload != nil {
		p.onUpload()
	}
	if p.failUpload {
		return fmt.Errorf("upload failed for %s", p.name)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if p.objects == nil {
		p.objects = map[string][]byte{}
	}
	p.objects[objectKey] = data
	return nil
}
func (p *testStorageProvider) Download(_ context.Context, objectKey string) (io.ReadCloser, error) {
	data, ok := p.objects[objectKey]
	if !ok {
		return nil, fmt.Errorf("object %s not found", objectKey)
	}
	return io.NopCloser(strings.NewReader(string(data))), nil
}
func (p *testStorageProvider) Delete(_ context.Context, objectKey string) error {
	delete(p.objects, objectKey)
	return nil
}
func (p *testStorageProvider) List(context.Context, string) ([]storage.ObjectInfo, error) {
	return nil, nil
}

func newExecutionTestServices(t *testing.T) (*BackupExecutionService, *BackupRecordService, repository.BackupTaskRepository, repository.StorageTargetRepository, repository.BackupRecordRepository, string, string) {
	t.Helper()
	baseDir := t.TempDir()
	storageDir := filepath.Join(baseDir, "storage")
	sourceDir := filepath.Join(baseDir, "source")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "index.html"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatalf("logger.New returned error: %v", err)
	}
	db, err := database.Open(config.DatabaseConfig{Path: filepath.Join(baseDir, "backupx.db")}, log)
	if err != nil {
		t.Fatalf("database.Open returned error: %v", err)
	}
	cipher := codec.NewConfigCipher("execution-secret")
	tasks := repository.NewBackupTaskRepository(db)
	targets := repository.NewStorageTargetRepository(db)
	records := repository.NewBackupRecordRepository(db)
	configCiphertext, err := cipher.EncryptJSON(map[string]any{"basePath": storageDir})
	if err != nil {
		t.Fatalf("EncryptJSON returned error: %v", err)
	}
	if err := targets.Create(context.Background(), &model.StorageTarget{Name: "local", Type: string(storage.ProviderTypeLocalDisk), Enabled: true, ConfigCiphertext: configCiphertext, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("Create storage target returned error: %v", err)
	}
	if err := tasks.Create(context.Background(), &model.BackupTask{Name: "site-files", Type: "file", Enabled: true, SourcePath: sourceDir, StorageTargetID: 1, RetentionDays: 30, Compression: "gzip", MaxBackups: 10, LastStatus: "idle"}); err != nil {
		t.Fatalf("Create backup task returned error: %v", err)
	}
	logHub := backup.NewLogHub()
	runnerRegistry := backup.NewRegistry(backup.NewFileRunner(), backup.NewMySQLRunner(nil), backup.NewSQLiteRunner(), backup.NewPostgreSQLRunner(nil))
	storageRegistry := storage.NewRegistry(storageRclone.NewLocalDiskFactory())
	retentionService := backupretention.NewService(records)
	tempDir := filepath.Join(baseDir, "tmp")
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("MkdirAll tempDir returned error: %v", err)
	}
	executionService := NewBackupExecutionService(tasks, records, targets, storageRegistry, runnerRegistry, logHub, retentionService, cipher, nil, tempDir, 2, 10, "")
	recordService := NewBackupRecordService(records, executionService, logHub)
	return executionService, recordService, tasks, targets, records, sourceDir, storageDir
}

func TestBackupExecutionServiceRunTaskByIDSync(t *testing.T) {
	executionService, _, _, _, records, _, storageDir := newExecutionTestServices(t)
	detail, err := executionService.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if detail.Status != "success" || detail.StoragePath == "" {
		t.Fatalf("unexpected record detail: %#v", detail)
	}
	stored, err := records.FindByID(context.Background(), detail.ID)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stored == nil || stored.Status != "success" {
		t.Fatalf("unexpected stored record: %#v", stored)
	}
	if _, err := os.Stat(filepath.Join(storageDir, filepath.FromSlash(detail.StoragePath))); err != nil {
		t.Fatalf("expected artifact in local storage: %v", err)
	}
}

func TestBackupExecutionServiceNodePoolSelectionDoesNotPersistTaskNodeID(t *testing.T) {
	executionService, _, tasks, _, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()

	nodeRepo := &nodeRepoStub{nodes: []model.Node{
		{ID: 10, Name: "edge-a", Token: "edge-a-token", Status: model.NodeStatusOnline, Labels: "prod,db"},
		{ID: 11, Name: "edge-b", Token: "edge-b-token", Status: model.NodeStatusOnline, Labels: "prod,db"},
	}}
	dispatcher := &fakeDispatcher{}
	executionService.SetClusterDependencies(nodeRepo, dispatcher)

	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	task.NodeID = 0
	task.NodePoolTag = "db"
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update task returned error: %v", err)
	}

	detail, err := executionService.RunTaskByID(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByID returned error: %v", err)
	}
	storedTask, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID after run returned error: %v", err)
	}
	if storedTask.NodeID != 0 {
		t.Fatalf("expected pooled task NodeID to remain 0, got %d", storedTask.NodeID)
	}
	if storedTask.NodePoolTag != "db" {
		t.Fatalf("expected pooled task tag to remain db, got %q", storedTask.NodePoolTag)
	}
	storedRecord, err := records.FindByID(ctx, detail.ID)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if storedRecord == nil || storedRecord.NodeID != 10 {
		t.Fatalf("expected record to keep selected node 10, got %#v", storedRecord)
	}
	calls := dispatcher.snapshot()
	if len(calls) != 1 || calls[0].NodeID != 10 || calls[0].CmdType != model.AgentCommandTypeRunTask {
		t.Fatalf("unexpected dispatcher calls: %#v", calls)
	}
}

func TestBackupExecutionServiceRejectsDuplicateRunningTask(t *testing.T) {
	executionService, _, tasks, _, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()

	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	startedAt := time.Now().UTC()
	running := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		NodeID:          0,
		Status:          model.BackupRecordStatusRunning,
		StartedAt:       startedAt,
	}
	if err := records.Create(ctx, running); err != nil {
		t.Fatalf("Create running record returned error: %v", err)
	}

	_, err = executionService.RunTaskByIDSync(ctx, task.ID)
	if err == nil || !strings.Contains(err.Error(), "正在运行") {
		t.Fatalf("expected duplicate running task to be rejected, got %v", err)
	}
	items, err := records.List(ctx, repository.BackupRecordListOptions{Status: model.BackupRecordStatusRunning})
	if err != nil {
		t.Fatalf("List running records returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != running.ID {
		t.Fatalf("expected only the original running record, got %#v", items)
	}
}

func TestBackupExecutionServiceDeleteRecordDispatchesRemoteLocalDiskCleanup(t *testing.T) {
	executionService, _, tasks, _, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()
	nodeRepo := &nodeRepoStub{nodes: []model.Node{
		{ID: 10, Name: "edge-a", Token: "edge-a-token", Status: model.NodeStatusOnline},
	}}
	dispatcher := &fakeDispatcher{}
	executionService.SetClusterDependencies(nodeRepo, dispatcher)

	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	completedAt := time.Now().UTC()
	record := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		NodeID:          10,
		Status:          model.BackupRecordStatusSuccess,
		FileName:        "remote.tar.gz",
		StoragePath:     "file/2026/05/09/remote.tar.gz",
		StartedAt:       completedAt.Add(-time.Second),
		CompletedAt:     &completedAt,
	}
	if err := records.Create(ctx, record); err != nil {
		t.Fatalf("Create record returned error: %v", err)
	}

	if err := executionService.DeleteRecord(ctx, record.ID); err != nil {
		t.Fatalf("DeleteRecord returned error: %v", err)
	}
	deleted, err := records.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected record deleted, got %#v", deleted)
	}
	calls := dispatcher.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected one dispatcher call, got %#v", calls)
	}
	if calls[0].NodeID != 10 || calls[0].CmdType != model.AgentCommandTypeDeleteStorageObject {
		t.Fatalf("unexpected dispatcher call: %#v", calls[0])
	}
	if calls[0].Payload["storagePath"] != record.StoragePath {
		t.Fatalf("expected storagePath %q, got %#v", record.StoragePath, calls[0].Payload)
	}
	if calls[0].Payload["targetType"] != string(storage.ProviderTypeLocalDisk) {
		t.Fatalf("expected local_disk targetType, got %#v", calls[0].Payload)
	}
	if _, ok := calls[0].Payload["targetConfig"].(map[string]any); !ok {
		t.Fatalf("expected targetConfig map, got %#v", calls[0].Payload["targetConfig"])
	}
}

func TestBackupExecutionServiceRestoreRecordRejectsRemoteLocalDisk(t *testing.T) {
	executionService, _, tasks, _, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()
	executionService.SetClusterDependencies(&nodeRepoStub{nodes: []model.Node{
		{ID: 10, Name: "edge-a", Token: "edge-a-token", Status: model.NodeStatusOnline},
	}}, &fakeDispatcher{})
	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	completedAt := time.Now().UTC()
	record := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		NodeID:          10,
		Status:          model.BackupRecordStatusSuccess,
		FileName:        "remote.tar.gz",
		StoragePath:     "file/2026/05/09/remote.tar.gz",
		StartedAt:       completedAt.Add(-time.Second),
		CompletedAt:     &completedAt,
	}
	if err := records.Create(ctx, record); err != nil {
		t.Fatalf("Create record returned error: %v", err)
	}

	err = executionService.RestoreRecord(ctx, record.ID)
	if err == nil {
		t.Fatal("expected remote local_disk restore to be rejected")
	}
	if !strings.Contains(err.Error(), "Master 无法跨节点访问") {
		t.Fatalf("expected cross-node local_disk error, got %v", err)
	}
}

func TestBackupExecutionServiceRecordsFirstSuccessfulStorageTarget(t *testing.T) {
	executionService, _, tasks, targets, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()
	second := &testStorageProvider{name: "second", objects: map[string][]byte{}}
	executionService.storageRegistry = storage.NewRegistry(&testStorageFactory{providers: map[string]*testStorageProvider{
		"second": second,
	}})
	cipher := codec.NewConfigCipher("execution-secret")
	firstConfig, err := cipher.EncryptJSON(map[string]any{"name": "missing"})
	if err != nil {
		t.Fatalf("EncryptJSON first returned error: %v", err)
	}
	secondConfig, err := cipher.EncryptJSON(map[string]any{"name": "second"})
	if err != nil {
		t.Fatalf("EncryptJSON second returned error: %v", err)
	}
	if err := targets.Create(ctx, &model.StorageTarget{Name: "first", Type: "test_storage", Enabled: true, ConfigCiphertext: firstConfig, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("Create first target returned error: %v", err)
	}
	if err := targets.Create(ctx, &model.StorageTarget{Name: "second", Type: "test_storage", Enabled: true, ConfigCiphertext: secondConfig, ConfigVersion: 1, LastTestStatus: "unknown"}); err != nil {
		t.Fatalf("Create second target returned error: %v", err)
	}
	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	task.StorageTargetID = 2
	task.StorageTargets = []model.StorageTarget{{ID: 2}, {ID: 3}}
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update task returned error: %v", err)
	}

	detail, err := executionService.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if detail.Status != model.BackupRecordStatusSuccess {
		t.Fatalf("expected success, got %#v", detail)
	}
	storedRecord, err := records.FindByID(ctx, detail.ID)
	if err != nil {
		t.Fatalf("FindByID record returned error: %v", err)
	}
	if storedRecord.StorageTargetID != 3 {
		t.Fatalf("expected record StorageTargetID to point at successful target 3, got %d", storedRecord.StorageTargetID)
	}
	if _, ok := second.objects[storedRecord.StoragePath]; !ok {
		t.Fatalf("expected object in successful provider at %q", storedRecord.StoragePath)
	}
}

func TestBackupExecutionServiceUploadRetryStopsWhenContextCancelled(t *testing.T) {
	executionService, _, tasks, targets, records, _, _ := newExecutionTestServices(t)
	ctx, cancel := context.WithCancel(context.Background())
	var cancelOnce sync.Once
	failing := &testStorageProvider{
		name:       "failing",
		failUpload: true,
		onUpload: func() {
			cancelOnce.Do(cancel)
		},
	}
	executionService.storageRegistry = storage.NewRegistry(&testStorageFactory{providers: map[string]*testStorageProvider{
		"failing": failing,
	}})
	cipher := codec.NewConfigCipher("execution-secret")
	failingConfig, err := cipher.EncryptJSON(map[string]any{"name": "failing"})
	if err != nil {
		t.Fatalf("EncryptJSON returned error: %v", err)
	}
	if err := targets.Update(ctx, &model.StorageTarget{
		ID:               1,
		Name:             "local",
		Type:             "test_storage",
		Enabled:          true,
		ConfigCiphertext: failingConfig,
		ConfigVersion:    1,
		LastTestStatus:   "unknown",
	}); err != nil {
		t.Fatalf("Update target returned error: %v", err)
	}
	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	startedAt := time.Now().UTC()
	record := &model.BackupRecord{
		TaskID:          task.ID,
		StorageTargetID: task.StorageTargetID,
		Status:          model.BackupRecordStatusRunning,
		StartedAt:       startedAt,
	}
	if err := records.Create(ctx, record); err != nil {
		t.Fatalf("Create record returned error: %v", err)
	}

	done := make(chan struct{})
	go func() {
		executionService.executeTask(ctx, task, record.ID, startedAt)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("expected cancelled upload retry to stop without waiting for backoff sleep")
	}
}

func TestBackupExecutionServiceReadsStorageUsageOnceForMultiTargetQuotaChecks(t *testing.T) {
	executionService, _, tasks, targets, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()
	first := &testStorageProvider{name: "first", objects: map[string][]byte{}}
	second := &testStorageProvider{name: "second", objects: map[string][]byte{}}
	executionService.storageRegistry = storage.NewRegistry(&testStorageFactory{providers: map[string]*testStorageProvider{
		"first":  first,
		"second": second,
	}})
	cipher := codec.NewConfigCipher("execution-secret")
	firstConfig, err := cipher.EncryptJSON(map[string]any{"name": "first"})
	if err != nil {
		t.Fatalf("EncryptJSON first returned error: %v", err)
	}
	secondConfig, err := cipher.EncryptJSON(map[string]any{"name": "second"})
	if err != nil {
		t.Fatalf("EncryptJSON second returned error: %v", err)
	}
	if err := targets.Update(ctx, &model.StorageTarget{ID: 1, Name: "local", Type: "test_storage", Enabled: true, ConfigCiphertext: firstConfig, ConfigVersion: 1, LastTestStatus: "unknown", QuotaBytes: 1 << 30}); err != nil {
		t.Fatalf("Update first target returned error: %v", err)
	}
	if err := targets.Create(ctx, &model.StorageTarget{Name: "second", Type: "test_storage", Enabled: true, ConfigCiphertext: secondConfig, ConfigVersion: 1, LastTestStatus: "unknown", QuotaBytes: 1 << 30}); err != nil {
		t.Fatalf("Create second target returned error: %v", err)
	}
	task, err := tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByID task returned error: %v", err)
	}
	task.StorageTargets = []model.StorageTarget{{ID: 1}, {ID: 2}}
	if err := tasks.Update(ctx, task); err != nil {
		t.Fatalf("Update task returned error: %v", err)
	}
	executionService.records = &storageUsageCountingRecordRepo{BackupRecordRepository: records}

	detail, err := executionService.RunTaskByIDSync(ctx, task.ID)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if detail.Status != model.BackupRecordStatusSuccess {
		t.Fatalf("expected success, got %#v", detail)
	}
	countingRepo := executionService.records.(*storageUsageCountingRecordRepo)
	if countingRepo.usageCalls != 1 {
		t.Fatalf("expected StorageUsage to be called once for quota snapshot, got %d", countingRepo.usageCalls)
	}
	if len(first.objects) != 1 || len(second.objects) != 1 {
		t.Fatalf("expected both targets to receive upload, got first=%d second=%d", len(first.objects), len(second.objects))
	}
}

func TestBackupExecutionServiceContinuesWhenStorageUsageSnapshotFails(t *testing.T) {
	executionService, _, _, targets, records, _, _ := newExecutionTestServices(t)
	ctx := context.Background()
	provider := &testStorageProvider{name: "primary", objects: map[string][]byte{}}
	executionService.storageRegistry = storage.NewRegistry(&testStorageFactory{providers: map[string]*testStorageProvider{
		"primary": provider,
	}})
	cipher := codec.NewConfigCipher("execution-secret")
	configCiphertext, err := cipher.EncryptJSON(map[string]any{"name": "primary"})
	if err != nil {
		t.Fatalf("EncryptJSON returned error: %v", err)
	}
	if err := targets.Update(ctx, &model.StorageTarget{
		ID:               1,
		Name:             "local",
		Type:             "test_storage",
		Enabled:          true,
		ConfigCiphertext: configCiphertext,
		ConfigVersion:    1,
		LastTestStatus:   "unknown",
		QuotaBytes:       1 << 30,
	}); err != nil {
		t.Fatalf("Update target returned error: %v", err)
	}
	executionService.records = &storageUsageFailingRecordRepo{
		BackupRecordRepository: records,
		err:                    errStorageUsageFailed,
	}

	detail, err := executionService.RunTaskByIDSync(ctx, 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if detail.Status != model.BackupRecordStatusSuccess {
		t.Fatalf("expected success despite soft quota usage snapshot error, got %#v", detail)
	}
	if len(provider.objects) != 1 {
		t.Fatalf("expected upload to proceed, got %d uploaded objects", len(provider.objects))
	}
}

func TestBackupRecordServiceRestore(t *testing.T) {
	executionService, recordService, _, _, _, sourceDir, _ := newExecutionTestServices(t)
	detail, err := executionService.RunTaskByIDSync(context.Background(), 1)
	if err != nil {
		t.Fatalf("RunTaskByIDSync returned error: %v", err)
	}
	if err := os.RemoveAll(sourceDir); err != nil {
		t.Fatalf("RemoveAll returned error: %v", err)
	}
	if err := recordService.Restore(context.Background(), detail.ID); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(sourceDir, "index.html"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected restored content: %s", string(content))
	}
}

type storageUsageCountingRecordRepo struct {
	repository.BackupRecordRepository
	mu         sync.Mutex
	usageCalls int
}

func (r *storageUsageCountingRecordRepo) StorageUsage(ctx context.Context) ([]repository.BackupStorageUsageItem, error) {
	r.mu.Lock()
	r.usageCalls++
	r.mu.Unlock()
	return r.BackupRecordRepository.StorageUsage(ctx)
}

type storageUsageFailingRecordRepo struct {
	repository.BackupRecordRepository
	err error
}

func (r *storageUsageFailingRecordRepo) StorageUsage(context.Context) ([]repository.BackupStorageUsageItem, error) {
	return nil, r.err
}

var errStorageUsageFailed = errors.New("storage usage failed")
