package rclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backupx/server/internal/storage"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

var minioMetricsClient = &http.Client{
	Timeout: 10 * time.Second,
	// 监控 Token 只发送到配置的 MinIO；重定向不能改变凭据接收方。
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func minioMetricsURL(endpoint, token string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u == nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("MinIO 容量查询需要不含凭据、查询参数或片段的 HTTP(S) Endpoint")
	}
	if strings.ContainsAny(strings.TrimSpace(token), "\r\n") {
		return "", errors.New("MinIO 监控 Token 不能包含换行")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/minio/v2/metrics/cluster"
	u.RawPath = ""
	return u.String(), nil
}

func minioCapacity(ctx context.Context, endpoint, token string) (*storage.StorageUsageInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create MinIO metrics request: %w", err)
	}
	req.Header.Set("Accept", "text/plain; version=0.0.4")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := minioMetricsClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request MinIO capacity: %w", err)
	}
	const maxMetricsBytes = 8 << 20
	payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxMetricsBytes+1))
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, fmt.Errorf("read MinIO metrics: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MinIO capacity returned HTTP %d", resp.StatusCode)
	}
	if len(payload) > maxMetricsBytes {
		return nil, errors.New("MinIO metrics response is too large")
	}
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("parse MinIO metrics: %w", err)
	}
	values := make([]int64, 2)
	for i, name := range []string{"minio_cluster_capacity_raw_total_bytes", "minio_cluster_capacity_raw_free_bytes"} {
		family := families[name]
		if family == nil || len(family.Metric) == 0 {
			return nil, fmt.Errorf("MinIO metrics missing %s", name)
		}
		for j, metric := range family.Metric {
			if metric.Gauge == nil {
				return nil, fmt.Errorf("MinIO capacity metric %s is not a gauge", name)
			}
			value := metric.Gauge.GetValue()
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value >= math.Exp2(63) || math.Trunc(value) != value {
				return nil, fmt.Errorf("MinIO capacity metric %s has an invalid value", name)
			}
			// 集群指标可能带 server 标签；相同总量不能按节点重复相加。
			if j > 0 && values[i] != int64(value) {
				return nil, fmt.Errorf("MinIO capacity metric %s has inconsistent samples", name)
			}
			values[i] = int64(value)
		}
	}
	total, free := values[0], values[1]
	if total <= 0 || free > total {
		return nil, errors.New("MinIO capacity metrics are unavailable or inconsistent")
	}
	used := total - free
	return &storage.StorageUsageInfo{Total: &total, Used: &used, Free: &free, Scope: "minio_cluster"}, nil
}
