package auth

import (
	"sync"
	"time"
)

const (
	MaxFailures   = 5
	failureWindow = time.Minute
)

// Limiter blocks an IP for one minute after five failed logins.
type Limiter struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[string][]time.Time
}

func NewLimiter(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{now: now, failures: make(map[string][]time.Time)}
}

func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-failureWindow)
	recent := make([]time.Time, 0, len(l.failures[ip]))
	for _, at := range l.failures[ip] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	l.failures[ip] = recent
	return len(recent) < MaxFailures
}

func (l *Limiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures[ip] = append(l.failures[ip], l.now())
}

func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, ip)
}
