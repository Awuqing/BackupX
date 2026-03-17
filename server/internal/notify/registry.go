package notify

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	notifiers map[string]Notifier
}

func NewRegistry(notifiers ...Notifier) *Registry {
	registry := &Registry{notifiers: make(map[string]Notifier)}
	for _, notifier := range notifiers {
		registry.Register(notifier)
	}
	return registry
}

func (r *Registry) Register(notifier Notifier) {
	if notifier == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.notifiers == nil {
		r.notifiers = make(map[string]Notifier)
	}
	r.notifiers[notifier.Type()] = notifier
}

func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]string, 0, len(r.notifiers))
	for key := range r.notifiers {
		items = append(items, key)
	}
	sort.Strings(items)
	return items
}

func (r *Registry) SensitiveFields(notificationType string) []string {
	notifier, ok := r.Notifier(notificationType)
	if !ok {
		return nil
	}
	return notifier.SensitiveFields()
}

func (r *Registry) Validate(notificationType string, config map[string]any) error {
	notifier, ok := r.Notifier(notificationType)
	if !ok {
		return fmt.Errorf("unsupported notification type: %s", notificationType)
	}
	return notifier.Validate(config)
}

func (r *Registry) Send(ctx context.Context, notificationType string, config map[string]any, message Message) error {
	notifier, ok := r.Notifier(notificationType)
	if !ok {
		return fmt.Errorf("unsupported notification type: %s", notificationType)
	}
	return notifier.Send(ctx, config, message)
}

func (r *Registry) Notifier(notificationType string) (Notifier, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	notifier, ok := r.notifiers[notificationType]
	return notifier, ok
}
