package backup

import (
	"context"
	"testing"
)

type stubRunner struct{ taskType string }

func (r stubRunner) Type() string                                                 { return r.taskType }
func (r stubRunner) Run(context.Context, TaskSpec, LogWriter) (*RunResult, error) { return nil, nil }
func (r stubRunner) Restore(context.Context, TaskSpec, string, LogWriter) error   { return nil }

func TestRegistryResolvesNormalizedType(t *testing.T) {
	registry := NewRegistry(stubRunner{taskType: "postgresql"})
	runner, err := registry.Runner("pgsql")
	if err != nil {
		t.Fatalf("Runner returned error: %v", err)
	}
	if runner.Type() != "postgresql" {
		t.Fatalf("unexpected runner type: %s", runner.Type())
	}
}
