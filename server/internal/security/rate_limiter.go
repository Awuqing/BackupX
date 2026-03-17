package security

import (
	"sync"
	"time"
)

type rateEntry struct {
	count     int
	windowEnd time.Time
}

type LoginRateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	items  map[string]rateEntry
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		limit:  limit,
		window: window,
		items:  make(map[string]rateEntry),
	}
}

func (r *LoginRateLimiter) Allow(key string) bool {
	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, ok := r.items[key]
	if !ok || now.After(entry.windowEnd) {
		r.items[key] = rateEntry{count: 0, windowEnd: now.Add(r.window)}
		entry = r.items[key]
	}
	if entry.count >= r.limit {
		return false
	}
	entry.count++
	r.items[key] = entry
	return true
}

func (r *LoginRateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, key)
}
