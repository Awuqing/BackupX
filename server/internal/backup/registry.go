package backup

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu      sync.RWMutex
	runners map[string]BackupRunner
}

func NewRegistry(runners ...BackupRunner) *Registry {
	registry := &Registry{runners: make(map[string]BackupRunner)}
	for _, runner := range runners {
		registry.Register(runner)
	}
	return registry
}

func (r *Registry) Register(runner BackupRunner) {
	if runner == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.runners == nil {
		r.runners = make(map[string]BackupRunner)
	}
	r.runners[normalizeTaskType(runner.Type())] = runner
}

func (r *Registry) Runner(taskType string) (BackupRunner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runner, ok := r.runners[normalizeTaskType(taskType)]
	if !ok {
		return nil, fmt.Errorf("unsupported backup task type: %s", taskType)
	}
	return runner, nil
}

func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]string, 0, len(r.runners))
	for key := range r.runners {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func normalizeTaskType(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "pgsql" {
		return "postgresql"
	}
	return normalized
}
