// Package metrics 暴露 BackupX 的 Prometheus 采集器。
//
// 设计要点：
//   - 使用独立 Registry，避免与 default registry 中的 Go runtime metrics 混淆
//   - Counter/Gauge/Histogram 全部以 backupx_ 为前缀，遵循 Prometheus 命名规范
//   - 所有指标都支持零值：未注入时调用方法是 no-op，不会 panic
//   - 组件只依赖本包，不反向引用 service/repository，避免循环
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics 聚合所有采集器，由 app 层组装一次并按需注入到 service。
type Metrics struct {
	registry *prometheus.Registry

	// 任务执行计数（labels: status, task_type）
	TaskRunTotal *prometheus.CounterVec
	// 任务耗时分布（labels: task_type）
	TaskRunDuration *prometheus.HistogramVec
	// 任务产出字节数（labels: task_type）
	TaskBytesTotal *prometheus.CounterVec
	// 正在运行的任务数
	TaskRunningGauge prometheus.Gauge
	// 存储目标用量（labels: target_name, target_type）
	StorageUsedBytes *prometheus.GaugeVec
	// 节点在线状态（labels: node_name, role；value: 0/1）
	NodeOnline *prometheus.GaugeVec
	// 验证演练结果（labels: status）
	VerifyRunTotal *prometheus.CounterVec
	// 恢复操作结果（labels: status）
	RestoreRunTotal *prometheus.CounterVec
	// 副本复制结果（labels: status）
	ReplicationRunTotal *prometheus.CounterVec
	// SLA 违约数（gauge）
	SLABreachGauge prometheus.Gauge
	// 应用信息（label: version）
	AppInfo *prometheus.GaugeVec
}

// New 构造并注册所有采集器。
// 失败时 panic：采集器注册失败属于启动期编程错误，没有合理 fallback。
func New(version string) *Metrics {
	reg := prometheus.NewRegistry()
	// 注入标准 Go runtime + process 指标
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		registry: reg,
		TaskRunTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "backupx_task_run_total",
			Help: "备份任务执行总数，按状态和任务类型细分",
		}, []string{"status", "task_type"}),
		TaskRunDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "backupx_task_run_duration_seconds",
			Help:    "备份任务耗时分布",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1800, 3600, 7200},
		}, []string{"task_type"}),
		TaskBytesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "backupx_task_bytes_total",
			Help: "备份任务累计产出字节数",
		}, []string{"task_type"}),
		TaskRunningGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "backupx_task_running",
			Help: "当前正在执行的备份任务数",
		}),
		StorageUsedBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "backupx_storage_used_bytes",
			Help: "存储目标已用字节数",
		}, []string{"target_name", "target_type"}),
		NodeOnline: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "backupx_node_online",
			Help: "集群节点在线状态（1 在线 / 0 离线）",
		}, []string{"node_name", "role"}),
		VerifyRunTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "backupx_verify_run_total",
			Help: "备份验证演练执行总数",
		}, []string{"status"}),
		RestoreRunTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "backupx_restore_run_total",
			Help: "恢复操作执行总数",
		}, []string{"status"}),
		ReplicationRunTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "backupx_replication_run_total",
			Help: "备份副本复制执行总数",
		}, []string{"status"}),
		SLABreachGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "backupx_sla_breach_tasks",
			Help: "当前违反 SLA/RPO 的任务数",
		}),
		AppInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "backupx_app_info",
			Help: "BackupX 应用元信息（恒为 1，通过 label 暴露版本号）",
		}, []string{"version"}),
	}
	reg.MustRegister(
		m.TaskRunTotal,
		m.TaskRunDuration,
		m.TaskBytesTotal,
		m.TaskRunningGauge,
		m.StorageUsedBytes,
		m.NodeOnline,
		m.VerifyRunTotal,
		m.RestoreRunTotal,
		m.ReplicationRunTotal,
		m.SLABreachGauge,
		m.AppInfo,
	)
	m.AppInfo.WithLabelValues(version).Set(1)
	return m
}

// Handler 返回 /metrics 的 HTTP handler。
// 使用本包专属 registry，避免混入其他组件的默认 metrics。
func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "metrics disabled", http.StatusServiceUnavailable)
		})
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})
}

// ObserveTaskRun 记录一次任务执行结果。
// status 常用值：success / failed / cancelled。nil 接收器安全。
func (m *Metrics) ObserveTaskRun(taskType, status string, durationSec float64, bytes int64) {
	if m == nil {
		return
	}
	m.TaskRunTotal.WithLabelValues(status, taskType).Inc()
	m.TaskRunDuration.WithLabelValues(taskType).Observe(durationSec)
	if bytes > 0 {
		m.TaskBytesTotal.WithLabelValues(taskType).Add(float64(bytes))
	}
}

// IncTaskRunning / DecTaskRunning 配套使用，反映并发中任务数。
func (m *Metrics) IncTaskRunning() {
	if m == nil {
		return
	}
	m.TaskRunningGauge.Inc()
}

func (m *Metrics) DecTaskRunning() {
	if m == nil {
		return
	}
	m.TaskRunningGauge.Dec()
}

// ObserveRestore / ObserveVerify / ObserveReplication 记录子动作结果。
// 所有方法对 nil 接收器安全：未注入 Metrics 时静默降级，不 panic。
func (m *Metrics) ObserveRestore(status string) {
	if m == nil {
		return
	}
	m.RestoreRunTotal.WithLabelValues(status).Inc()
}

func (m *Metrics) ObserveVerify(status string) {
	if m == nil {
		return
	}
	m.VerifyRunTotal.WithLabelValues(status).Inc()
}

func (m *Metrics) ObserveReplication(status string) {
	if m == nil {
		return
	}
	m.ReplicationRunTotal.WithLabelValues(status).Inc()
}

// SetStorageUsed 刷新某存储目标的用量。调用方负责周期采集。
func (m *Metrics) SetStorageUsed(name, targetType string, bytes int64) {
	if m == nil {
		return
	}
	m.StorageUsedBytes.WithLabelValues(name, targetType).Set(float64(bytes))
}

// SetNodeOnline 刷新节点在线状态。
func (m *Metrics) SetNodeOnline(name, role string, online bool) {
	if m == nil {
		return
	}
	val := 0.0
	if online {
		val = 1
	}
	m.NodeOnline.WithLabelValues(name, role).Set(val)
}

// ResetNodeOnline 清空节点 gauge（当节点被删除时避免残留指标）。
func (m *Metrics) ResetNodeOnline() {
	if m == nil {
		return
	}
	m.NodeOnline.Reset()
}

// ResetStorageUsed 清空存储目标 gauge。
func (m *Metrics) ResetStorageUsed() {
	if m == nil {
		return
	}
	m.StorageUsedBytes.Reset()
}

// SetSLABreach 刷新 SLA 违约任务数。
func (m *Metrics) SetSLABreach(count int) {
	if m == nil {
		return
	}
	m.SLABreachGauge.Set(float64(count))
}
