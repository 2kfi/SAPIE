package auth

import (
	"log"
	"strconv"
	"sync"
	"time"
)

const (
	MaxFailures   = 5
	FailureWindow = time.Minute
)

// LimitStore persists failure counters so a restart does not clear a lockout.
type LimitStore interface {
	RateCount(bucket string, since int64) (int64, error)
	AddRateEvent(bucket string, at int64) error
	ResetRate(bucket string) error
}

// Limiter is one sliding-window counter per bucket. A nil store keeps counters
// in memory, which is what tests use.
type Limiter struct {
	mu    sync.Mutex
	now   func() time.Time
	store LimitStore
	hits  map[string][]time.Time
}

func NewLimiter(now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{now: now, hits: make(map[string][]time.Time)}
}

func NewPersistentLimiter(store LimitStore, now func() time.Time) *Limiter {
	l := NewLimiter(now)
	l.store = store
	return l
}

func IPBucket(ip string) string           { return "login:" + ip }
func AccountBucket(steamID uint64) string { return "account:" + strconv.FormatUint(steamID, 10) }
func SocialBucket(steamID uint64) string  { return "social:" + strconv.FormatUint(steamID, 10) }

func (l *Limiter) Allow(ip string) bool {
	ok, err := l.AllowBucket(IPBucket(ip), MaxFailures, FailureWindow)
	if err != nil {
		log.Printf("limiter allow %s: %v", ip, err)
	}
	return ok
}

func (l *Limiter) RecordFailure(ip string) {
	if err := l.RecordBucket(IPBucket(ip)); err != nil {
		log.Printf("limiter record %s: %v", ip, err)
	}
}

func (l *Limiter) Reset(ip string) {
	if err := l.ResetBucket(IPBucket(ip)); err != nil {
		log.Printf("limiter reset %s: %v", ip, err)
	}
}

// AllowBucket reports whether bucket is below limit. It does not record the
// event; callers record only the failures they want to count.
func (l *Limiter) AllowBucket(bucket string, limit int, window time.Duration) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	since := l.now().Add(-window).Unix()
	if l.store != nil {
		n, err := l.store.RateCount(bucket, since)
		if err != nil {
			return false, err
		}
		return n < int64(limit), nil
	}
	recent := make([]time.Time, 0, len(l.hits[bucket]))
	for _, at := range l.hits[bucket] {
		if at.Unix() > since {
			recent = append(recent, at)
		}
	}
	l.hits[bucket] = recent
	return len(recent) < limit, nil
}

func (l *Limiter) RecordBucket(bucket string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		return l.store.AddRateEvent(bucket, l.now().Unix())
	}
	l.hits[bucket] = append(l.hits[bucket], l.now())
	return nil
}

func (l *Limiter) ResetBucket(bucket string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.store != nil {
		return l.store.ResetRate(bucket)
	}
	delete(l.hits, bucket)
	return nil
}
