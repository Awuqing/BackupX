package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNew_AppInfoVersionLabel(t *testing.T) {
	m := New("2.1.0")
	if got := testutil.ToFloat64(m.AppInfo.WithLabelValues("2.1.0")); got != 1 {
		t.Fatalf("app_info(version=2.1.0) expected 1, got %v", got)
	}
}

func TestObserveTaskRun_IncrementsCounterAndHistogram(t *testing.T) {
	m := New("test")
	m.ObserveTaskRun("mysql", "success", 12.5, 1024)
	m.ObserveTaskRun("mysql", "failed", 3.0, 0)
	if got := testutil.ToFloat64(m.TaskRunTotal.WithLabelValues("success", "mysql")); got != 1 {
		t.Fatalf("task_run_total{status=success,task_type=mysql}: expected 1, got %v", got)
	}
	if got := testutil.ToFloat64(m.TaskRunTotal.WithLabelValues("failed", "mysql")); got != 1 {
		t.Fatalf("task_run_total{status=failed,task_type=mysql}: expected 1, got %v", got)
	}
	if got := testutil.ToFloat64(m.TaskBytesTotal.WithLabelValues("mysql")); got != 1024 {
		t.Fatalf("task_bytes_total{task_type=mysql}: expected 1024, got %v", got)
	}
}

func TestObserveTaskRun_NilReceiverIsSafe(t *testing.T) {
	var m *Metrics // nil
	m.ObserveTaskRun("file", "success", 1, 1)
	m.ObserveRestore("success")
	m.ObserveVerify("failed")
	m.ObserveReplication("success")
	m.IncTaskRunning()
	m.DecTaskRunning()
	m.SetStorageUsed("a", "s3", 1)
	m.SetNodeOnline("n1", "master", true)
	m.SetSLABreach(3)
	m.ResetNodeOnline()
	m.ResetStorageUsed()
	// no panic -> pass
}

func TestHandler_ExposesBackupxMetrics(t *testing.T) {
	m := New("0.0.0-test")
	m.ObserveTaskRun("file", "success", 1.0, 2048)
	m.SetNodeOnline("n1", "master", true)
	m.SetSLABreach(1)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	m.Handler().ServeHTTP(recorder, req)

	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	content := string(body)
	for _, keyword := range []string{
		"backupx_task_run_total",
		"backupx_task_run_duration_seconds",
		"backupx_node_online",
		"backupx_sla_breach_tasks",
		"backupx_app_info",
	} {
		if !strings.Contains(content, keyword) {
			t.Errorf("expected /metrics to contain %q", keyword)
		}
	}
}
