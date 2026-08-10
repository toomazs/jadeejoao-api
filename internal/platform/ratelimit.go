package platform

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter keeps one token bucket per key (client IP). In-process state is
// safe because the API runs exactly one replica (AD-12); scaling out is a
// documented trigger to revisit this.
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucketEntry
	rate      rate.Limit
	burst     int
	lastSweep time.Time
}

type bucketEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// sweep configuration: forget buckets idle for 10 minutes, checking at most
// every 5 minutes, so the map cannot grow unbounded.
const (
	sweepEvery = 5 * time.Minute
	bucketTTL  = 10 * time.Minute
)

// NewRateLimiter allows perMinute sustained requests with the given burst,
// per key.
func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets:   map[string]*bucketEntry{},
		rate:      rate.Limit(float64(perMinute) / 60.0),
		burst:     burst,
		lastSweep: time.Now(),
	}
}

// Allow reports whether the key may proceed now.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if now.Sub(l.lastSweep) > sweepEvery {
		for k, b := range l.buckets {
			if now.Sub(b.lastSeen) > bucketTTL {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}

	entry, ok := l.buckets[key]
	if !ok {
		entry = &bucketEntry{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.buckets[key] = entry
	}
	entry.lastSeen = now
	return entry.limiter.Allow()
}
