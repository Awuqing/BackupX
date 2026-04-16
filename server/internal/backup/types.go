package backup

import (
	"context"
	"time"
)

type DatabaseSpec struct {
	Host     string
	Port     int
	User     string
	Password string
	Names    []string
	Path     string
	// SAP HANA 特有字段（其他类型忽略）
	InstanceNumber string // 实例编号（从端口推断或手动指定）
	BackupLevel    string // "full"(默认) / "incremental" / "differential"
	BackupType     string // "data"(默认) / "log"
	BackupChannels int    // 并行通道数（默认 1）
	MaxRetries     int    // 最大重试次数（默认 3）
}

type TaskSpec struct {
	ID                uint
	Name              string
	Type              string
	SourcePath        string
	SourcePaths       []string
	ExcludePatterns   []string
	Database          DatabaseSpec
	StorageTargetID   uint
	StorageTargetType string
	Compression       string
	Encrypt           bool
	RetentionDays     int
	MaxBackups        int
	StartedAt         time.Time
	TempDir           string
}

type RunResult struct {
	ArtifactPath string
	FileName     string
	TempDir      string
	Size         int64
	StorageKey   string
}

type LogEvent struct {
	RecordID  uint          `json:"recordId"`
	Sequence  int64         `json:"sequence"`
	Level     string        `json:"level"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Completed bool          `json:"completed"`
	Status    string        `json:"status"`
	Progress  *ProgressInfo `json:"progress,omitempty"`
}

// ProgressInfo 描述上传进度，通过 SSE 实时推送给前端。
type ProgressInfo struct {
	BytesSent  int64   `json:"bytesSent"`
	TotalBytes int64   `json:"totalBytes"`
	Percent    float64 `json:"percent"`
	SpeedBps   float64 `json:"speedBps"`   // bytes/sec
	TargetName string  `json:"targetName"`
}

type LogWriter interface {
	WriteLine(message string)
}

type LogSink interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

type NopLogWriter struct{}

func (NopLogWriter) WriteLine(string)      {}
func (NopLogWriter) Infof(string, ...any)  {}
func (NopLogWriter) Warnf(string, ...any)  {}
func (NopLogWriter) Errorf(string, ...any) {}

type BackupRunner interface {
	Type() string
	Run(ctx context.Context, task TaskSpec, writer LogWriter) (*RunResult, error)
	Restore(ctx context.Context, task TaskSpec, artifactPath string, writer LogWriter) error
}
