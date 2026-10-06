package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/storage"
	storageRclone "backupx/server/internal/storage/rclone"
)

func TestStorageTargetMinIOCapacityAndCredentials(t *testing.T) {
	svc, _, _, _, owner, _ := newAgentServicePoolTestHarness(t)
	var denied atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/minio/v2/metrics/cluster" || r.Header.Get("Authorization") != "Bearer monitoring-token" {
			t.Error("invalid monitoring request")
		}
		if denied.Load() {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if _, err := fmt.Fprint(w, "# TYPE minio_cluster_capacity_raw_total_bytes gauge\nminio_cluster_capacity_raw_total_bytes 1024\n# TYPE minio_cluster_capacity_raw_free_bytes gauge\nminio_cluster_capacity_raw_free_bytes 128\n"); err != nil {
			t.Errorf("write metrics: %v", err)
		}
	}))
	defer server.Close()
	ctx := context.Background()
	target, err := svc.storageRepo.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	target.Type = storage.TypeS3
	target.ConfigCiphertext, err = svc.cipher.EncryptJSON(map[string]any{"endpoint": server.URL, "bucket": "backups", "accessKeyId": "backup-key", "secretAccessKey": "backup-secret", "minioCapacity": true, "minioMetricsToken": "monitoring-token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.storageRepo.Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	registry := storage.NewRegistry(storageRclone.NewS3Factory())
	targets := NewStorageTargetService(svc.storageRepo, nil, registry, svc.cipher)
	detail, err := targets.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Config["minioMetricsToken"] != "********" {
		t.Fatal("monitoring token exposed in target details")
	}
	usage, err := targets.GetUsage(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if usage.DiskUsage == nil || *usage.DiskUsage.Total != 1024 || *usage.DiskUsage.Used != 896 || usage.DiskUsage.Scope != "minio_cluster" {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	warning := &minioCapacityWarningCapture{}
	var mu sync.Mutex
	targets.runCapacityCheckOnce(ctx, warning, &mu, map[uint]bool{})
	if warning.title != "BackupX MinIO 集群物理容量预警" || warning.fields["capacityScope"] != "minio_cluster" {
		t.Fatalf("capacity warning omitted cluster scope: %+v", warning)
	}
	denied.Store(true)
	usage, err = targets.GetUsage(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if usage.DiskUsage != nil || usage.CapacityError == "" || strings.Contains(usage.CapacityError, "monitoring-token") {
		t.Fatalf("invalid failure result: %+v", usage)
	}
	// 备份下发只能包含 S3 传输凭据，不能带上 Master 的监控权限。
	spec, err := svc.GetTaskSpec(ctx, owner, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertTransferConfig := func(raw json.RawMessage) {
		t.Helper()
		var config map[string]any
		if err := json.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		if _, ok := config["minioMetricsToken"]; ok {
			t.Fatal("monitoring token forwarded to Agent")
		}
		if _, ok := config["minioCapacity"]; ok {
			t.Fatal("Agent should not query cluster capacity")
		}
		if config["accessKeyId"] != "backup-key" || config["secretAccessKey"] != "backup-secret" {
			t.Fatal("S3 transfer credentials were lost")
		}
	}
	assertTransferConfig(spec.StorageTargets[0].Config)
	// 恢复下发使用相同的过滤规则。
	h := newRestoreTestHarness(t, true)
	restoreTarget, err := h.service.targets.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	restoreTarget.Type = target.Type
	var config map[string]any
	if err := svc.cipher.DecryptJSON(target.ConfigCiphertext, &config); err != nil {
		t.Fatal(err)
	}
	restoreTarget.ConfigCiphertext, err = h.service.cipher.EncryptJSON(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.service.targets.Update(ctx, restoreTarget); err != nil {
		t.Fatal(err)
	}
	task, err := h.tasks.FindByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	restoreOwner, err := h.nodes.FindByID(ctx, task.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	record := &model.BackupRecord{TaskID: task.ID, StorageTargetID: 1, NodeID: restoreOwner.ID, Status: model.BackupRecordStatusSuccess, FileName: "backup.tar.gz", StoragePath: "backup.tar.gz", StartedAt: time.Now().UTC()}
	if err := h.records.Create(ctx, record); err != nil {
		t.Fatal(err)
	}
	restore := &model.RestoreRecord{BackupRecordID: record.ID, TaskID: task.ID, NodeID: restoreOwner.ID, Status: model.RestoreRecordStatusRunning, StartedAt: time.Now().UTC()}
	if err := h.restores.Create(ctx, restore); err != nil {
		t.Fatal(err)
	}
	restoreSpec, err := h.service.GetAgentRestoreSpec(ctx, restoreOwner, restore.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertTransferConfig(restoreSpec.Storage.Config)
}

type minioCapacityWarningCapture struct {
	title  string
	fields map[string]any
}

func (c *minioCapacityWarningCapture) DispatchEvent(_ context.Context, _, title, _ string, fields map[string]any) error {
	c.title, c.fields = title, fields
	return nil
}
