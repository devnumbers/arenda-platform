package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter is an in-memory per-key token bucket rate limiter.
type RateLimiter struct {
	limit           rate.Limit
	burst           int
	cleanupInterval time.Duration

	mu       sync.Mutex
	buckets  map[string]*bucket
	stop     chan struct{}
	stopOnce sync.Once
}

type bucket struct {
	*rate.Limiter
	lastUsed time.Time
}

// NewRateLimiter creates a rate limiter with the given per-key rate and burst.
// Stale buckets are cleaned up periodically using cleanupInterval.
func NewRateLimiter(limit rate.Limit, burst int, cleanupInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limit:           limit,
		burst:           burst,
		cleanupInterval: cleanupInterval,
		buckets:         make(map[string]*bucket),
		stop:            make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Allow reports whether the request for the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{Limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastUsed = time.Now()
	rl.mu.Unlock()

	return b.Allow()
}

// Stop halts the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			rl.mu.Lock()
			for key, b := range rl.buckets {
				if time.Since(b.lastUsed) > rl.cleanupInterval {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}
