package httpsupport

import (
	"hash/fnv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const shardCount = 256

// RateLimiter is an in-memory per-key token bucket rate limiter.
// Buckets are sharded by key hash to reduce lock contention.
type RateLimiter struct {
	limit           rate.Limit
	burst           int
	cleanupInterval time.Duration

	shards   [shardCount]*rateLimiterShard
	stop     chan struct{}
	stopOnce sync.Once
}

type rateLimiterShard struct {
	mu      sync.Mutex
	buckets map[string]*bucket
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
		stop:            make(chan struct{}),
	}
	for i := range rl.shards {
		rl.shards[i] = &rateLimiterShard{buckets: make(map[string]*bucket)}
	}
	go rl.cleanup()
	return rl
}

// Allow reports whether the request for the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	shard := rl.shardFor(key)
	shard.mu.Lock()
	b, ok := shard.buckets[key]
	if !ok {
		b = &bucket{Limiter: rate.NewLimiter(rl.limit, rl.burst)}
		shard.buckets[key] = b
	}
	b.lastUsed = time.Now()
	shard.mu.Unlock()

	return b.Allow()
}

// Stop halts the background cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() {
		close(rl.stop)
	})
}

func (rl *RateLimiter) shardFor(key string) *rateLimiterShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return rl.shards[h.Sum32()%shardCount]
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stop:
			return
		case <-ticker.C:
			for _, shard := range rl.shards {
				shard.mu.Lock()
				for key, b := range shard.buckets {
					if time.Since(b.lastUsed) > rl.cleanupInterval {
						delete(shard.buckets, key)
					}
				}
				shard.mu.Unlock()
			}
		}
	}
}
