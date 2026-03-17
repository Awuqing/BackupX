//go:build ignore

package security

import (
	"sync"
	"time"
)

type limiterEntry struct {
	Count   int
	ResetAt time.Time
}

type LoginLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	max     int
	records map[string]limiterEntry
}

func NewLoginLimiter(max int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{window: window, max: max, records: make(map[string]limiterEntry)}
}

func (l *LoginLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	entry, ok := l.records[key]
	if !ok || time.Now().After(entry.ResetAt) {
		delete(l.records, key)
		return true
	}
	return entry.Count < l.max
}

func (l *LoginLimiter) RegisterFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	entry, ok := l.records[key]
	if !ok || now.After(entry.ResetAt) {
		l.records[key] = limiterEntry{Count: 1, ResetAt: now.Add(l.window)}
		return
	}
	entry.Count++
	l.records[key] = entry
}

func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.records, key)
}
