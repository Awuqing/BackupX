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
}

type TaskSpec struct {
	ID                uint
	Name              string
	Type              string
	SourcePath        string
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
	RecordID  uint      `json:"recordId"`
	Sequence  int64     `json:"sequence"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Completed bool      `json:"completed"`
	Status    string    `json:"status"`
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
