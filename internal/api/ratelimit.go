package api

import (
	"sync"
	"time"
)

// failureLimiter blocks a key after max failures within window.
type failureLimiter struct {
	mu      sync.Mutex
	entries map[string]*failureEntry
	max     int
	window  time.Duration
	block   time.Duration
	now     func() time.Time
}

// failureEntry is the per-key window and block state.
type failureEntry struct {
	failures     int
	windowStart  time.Time
	blockedUntil time.Time
}

// newFailureLimiter creates a limiter that blocks a key after max failures in window.
func newFailureLimiter(max int, window, block time.Duration) *failureLimiter {
	return &failureLimiter{
		entries: make(map[string]*failureEntry),
		max:     max,
		window:  window,
		block:   block,
		now:     time.Now,
	}
}

// Allow reports whether the key may proceed and, if not, how long to wait.
func (l *failureLimiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok {
		return true, 0
	}
	if now.Before(e.blockedUntil) {
		return false, e.blockedUntil.Sub(now)
	}
	if now.Sub(e.windowStart) > l.window {
		delete(l.entries, key)
		return true, 0
	}
	if len(l.entries) > 1024 {
		l.pruneLocked(now)
	}
	return true, 0
}

// Fail records a failed attempt, blocking the key once the threshold is hit.
func (l *failureLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) > l.window {
		e = &failureEntry{windowStart: now}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= l.max {
		e.blockedUntil = now.Add(l.block)
		e.failures = 0
		e.windowStart = now
	}
	if len(l.entries) > 1024 {
		l.pruneLocked(now)
	}
}

// Reset clears the key after a successful attempt.
func (l *failureLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

// pruneLocked drops expired entries to bound memory. Callers hold the lock.
func (l *failureLimiter) pruneLocked(now time.Time) {
	for key, entry := range l.entries {
		if now.After(entry.blockedUntil) && now.Sub(entry.windowStart) > l.window {
			delete(l.entries, key)
		}
	}
}
