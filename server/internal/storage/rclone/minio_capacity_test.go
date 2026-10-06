package rclone

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"backupx/server/internal/storage"
	"backupx/server/internal/storage/codec"
)

func capacityMetrics(total, free string) string {
	return "# TYPE minio_cluster_capacity_raw_total_bytes gauge\nminio_cluster_capacity_raw_total_bytes{server=\"minio:9000\"} " + total +
		"\n# TYPE minio_cluster_capacity_raw_free_bytes gauge\nminio_cluster_capacity_raw_free_bytes{server=\"minio:9000\"} " + free + "\n"
}

func TestS3MinIOCapacity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/minio/v2/metrics/cluster" || r.Header.Get("Authorization") != "Bearer monitoring-token" {
			t.Errorf("unexpected metrics request path or authorization")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if _, err := fmt.Fprint(w, capacityMetrics("1.024e+3", "768")); err != nil {
			t.Errorf("write metrics: %v", err)
		}
	}))
	defer server.Close()
	factory := NewS3Factory()
	raw := map[string]any{"endpoint": server.URL, "bucket": "backups", "accessKeyId": "backup-key", "secretAccessKey": "backup-secret", "minioCapacity": true, "minioMetricsToken": "monitoring-token"}
	provider, err := factory.New(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	usage, err := provider.(storage.StorageAbout).About(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *usage.Total != 1024 || *usage.Free != 768 || *usage.Used != 256 || usage.Scope != "minio_cluster" {
		t.Fatalf("unexpected capacity: %+v", usage)
	}
	if codec.MaskConfig(raw, factory.SensitiveFields())["minioMetricsToken"] != "********" {
		t.Fatal("monitoring token must be masked")
	}
	raw["minioCapacity"] = false
	provider, err = factory.New(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.(storage.StorageAbout).About(context.Background()); err == nil {
		t.Fatal("ordinary S3 must not probe MinIO metrics")
	}
}

func TestMinIOCapacityValidation(t *testing.T) {
	cases := []struct {
		name, metrics string
		status        int
		wantError     bool
	}{
		{"public", capacityMetrics("1024", "1024"), 200, false},
		{"duplicate cluster samples", capacityMetrics("1024", "768") + "minio_cluster_capacity_raw_total_bytes{server=\"other:9000\"} 1024\n", 200, false},
		{"unauthorized", "do not return response bodies", 401, true},
		{"forbidden", "private metrics", 403, true},
		{"missing", "# HELP unrelated data\n", 200, true},
		{"zero capacity", capacityMetrics("0", "0"), 200, true},
		{"negative", capacityMetrics("1024", "-1"), 200, true},
		{"invalid free", capacityMetrics("1024", "2048"), 200, true},
		{"NaN", capacityMetrics("NaN", "10"), 200, true},
		{"overflow", capacityMetrics("9.223372036854776e18", "10"), 200, true},
		{"inconsistent replicas", capacityMetrics("1024", "768") + "minio_cluster_capacity_raw_total_bytes{server=\"other:9000\"} 2048\n", 200, true},
		{"invalid payload", "not metrics!", 200, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("public endpoint must not receive S3 credentials")
				}
				w.WriteHeader(tc.status)
				if _, err := fmt.Fprint(w, tc.metrics); err != nil {
					t.Errorf("write metrics: %v", err)
				}
			}))
			defer server.Close()
			usage, err := minioCapacity(context.Background(), server.URL, "")
			if (err != nil) != tc.wantError {
				t.Fatalf("usage=%+v error=%v", usage, err)
			}
			if tc.name == "duplicate cluster samples" && *usage.Total != 1024 {
				t.Fatal("cluster capacity was counted more than once")
			}
		})
	}
}

func TestMinIOCapacityDoesNotRedirectCredentials(t *testing.T) {
	var requests atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1) }))
	defer receiver.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, receiver.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	if _, err := minioCapacity(context.Background(), redirector.URL, "monitoring-token"); err == nil {
		t.Fatal("expected redirect failure")
	}
	if requests.Load() != 0 {
		t.Fatal("metrics client followed redirect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := minioCapacity(ctx, redirector.URL, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestMinIOMetricsURL(t *testing.T) {
	got, err := minioMetricsURL("https://minio.example.com/proxy/", "token")
	if err != nil || got != "https://minio.example.com/proxy/minio/v2/metrics/cluster" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, endpoint := range []string{"", "minio:9000", "ftp://minio", "https://user:pass@minio", "https://minio?token=secret", "https://minio#fragment"} {
		if _, err := minioMetricsURL(endpoint, ""); err == nil {
			t.Fatalf("accepted invalid endpoint %q", endpoint)
		}
	}
	if _, err := minioMetricsURL("https://minio", "token\r\ninjected: header"); err == nil {
		t.Fatal("accepted invalid token")
	}
}

func TestMinIOCapacityResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, strings.Repeat(" ", (8<<20)+1)); err != nil {
			t.Logf("client stopped oversized response: %v", err)
		}
	}))
	defer server.Close()
	if _, err := minioCapacity(context.Background(), server.URL, ""); err == nil {
		t.Fatal("accepted oversized response")
	}
}
