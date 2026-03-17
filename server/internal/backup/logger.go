package backup

import (
	"fmt"
	"strings"
	"sync"
)

type ExecutionLogger struct {
	recordID uint
	hub      *LogHub
	mu       sync.Mutex
	buffer   strings.Builder
}

func NewExecutionLogger(recordID uint, hub *LogHub) *ExecutionLogger {
	return &ExecutionLogger{recordID: recordID, hub: hub}
}

func (l *ExecutionLogger) Write(level, message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buffer.Len() > 0 {
		l.buffer.WriteByte('\n')
	}
	l.buffer.WriteString(trimmed)
	if l.hub != nil {
		l.hub.Append(l.recordID, level, trimmed)
	}
}

func (l *ExecutionLogger) Infof(format string, args ...any) {
	l.Write("info", fmt.Sprintf(format, args...))
}

func (l *ExecutionLogger) Errorf(format string, args ...any) {
	l.Write("error", fmt.Sprintf(format, args...))
}

func (l *ExecutionLogger) Warnf(format string, args ...any) {
	l.Write("warn", fmt.Sprintf(format, args...))
}

func (l *ExecutionLogger) WriteLine(message string) {
	l.Infof("%s", message)
}

func (l *ExecutionLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}
