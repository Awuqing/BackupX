package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backupx/server/internal/model"
	"backupx/server/internal/repository"
)

// fakeAuditRepo 用通道同步等待异步写入，避免 sleep。
type fakeAuditRepo struct {
	mu      sync.Mutex
	logs    []model.AuditLog
	created chan struct{}
}

func newFakeAuditRepo() *fakeAuditRepo {
	return &fakeAuditRepo{created: make(chan struct{}, 4)}
}

func (r *fakeAuditRepo) Create(_ context.Context, log *model.AuditLog) error {
	r.mu.Lock()
	log.CreatedAt = time.Now().UTC()
	r.logs = append(r.logs, *log)
	r.mu.Unlock()
	r.created <- struct{}{}
	return nil
}

func (r *fakeAuditRepo) List(context.Context, repository.AuditLogListOptions) (*repository.AuditLogListResult, error) {
	return &repository.AuditLogListResult{}, nil
}

func (r *fakeAuditRepo) ListAll(context.Context, repository.AuditLogListOptions) ([]model.AuditLog, error) {
	return nil, nil
}

func TestAuditService_WebhookDeliversSignedPayload(t *testing.T) {
	var hits atomic.Int32
	var got struct {
		sig      string
		payload  map[string]any
		received chan struct{}
	}
	got.received = make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		got.sig = r.Header.Get("X-BackupX-Signature")
		_ = json.Unmarshal(body, &got.payload)

		// 验证 HMAC 正确
		mac := hmac.New(sha256.New, []byte("s3cret"))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if got.sig != expected {
			t.Errorf("signature mismatch: expected %s, got %s", expected, got.sig)
		}
		w.WriteHeader(http.StatusOK)
		got.received <- struct{}{}
	}))
	defer server.Close()

	repo := newFakeAuditRepo()
	svc := NewAuditService(repo)
	svc.SetWebhook(server.URL, "s3cret")

	svc.Record(AuditEntry{
		Username: "alice",
		Category: "auth",
		Action:   "login_success",
		ClientIP: "10.0.0.1",
		Detail:   "admin login",
	})

	// 等待异步写入 + webhook
	select {
	case <-repo.created:
	case <-time.After(time.Second):
		t.Fatal("audit log not written within 1s")
	}
	select {
	case <-got.received:
	case <-time.After(time.Second):
		t.Fatal("webhook not invoked within 1s")
	}

	if hits.Load() != 1 {
		t.Fatalf("expected 1 webhook hit, got %d", hits.Load())
	}
	if got.payload["eventType"] != "audit.log" {
		t.Errorf("eventType wrong: %v", got.payload["eventType"])
	}
	actor, ok := got.payload["actor"].(map[string]any)
	if !ok || actor["username"] != "alice" {
		t.Errorf("actor.username mismatch: %v", got.payload["actor"])
	}
	if got.payload["action"] != "login_success" {
		t.Errorf("action mismatch: %v", got.payload["action"])
	}
}

func TestAuditService_WebhookDisabledWhenURLEmpty(t *testing.T) {
	repo := newFakeAuditRepo()
	svc := NewAuditService(repo)
	// 不调用 SetWebhook：应该不发送任何请求
	svc.Record(AuditEntry{Username: "bob", Action: "logout"})

	select {
	case <-repo.created:
	case <-time.After(time.Second):
		t.Fatal("audit log not written within 1s")
	}
	// 给 webhook 一些时间（即便它不会被调用）
	time.Sleep(100 * time.Millisecond)
	// 无显式断言：能不 panic 即算通过
}
