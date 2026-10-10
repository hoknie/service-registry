package auth

import (
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/time/rate"

	"svc-registry/internal/platform/config"
)

const maxKeys = 10_000

type buckets[K comparable] struct {
	limit rate.Limit
	burst int
	mu    sync.Mutex
	m     map[K]*rate.Limiter
}

func newBuckets[K comparable](n uint32, window time.Duration) *buckets[K] {
	return &buckets[K]{limit: rate.Limit(float64(n) / window.Seconds()), burst: int(n), m: map[K]*rate.Limiter{}}
}

func (b *buckets[K]) get(key K, now time.Time) *rate.Limiter {
	b.mu.Lock()
	defer b.mu.Unlock()
	if l, ok := b.m[key]; ok {
		return l
	}
	if len(b.m) >= maxKeys {
		b.prune(now)
	}
	l := rate.NewLimiter(b.limit, b.burst)
	b.m[key] = l
	return l
}

func (b *buckets[K]) peek(key K) (*rate.Limiter, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l, ok := b.m[key]
	return l, ok
}

func (b *buckets[K]) forget(key K) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, key)
}

func (b *buckets[K]) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.m)
}

func (b *buckets[K]) prune(now time.Time) {
	for k, l := range b.m {
		if l.TokensAt(now) >= float64(b.burst) {
			delete(b.m, k)
		}
	}
	for k := range b.m {
		if len(b.m) < maxKeys/2 {
			return
		}
		delete(b.m, k)
	}
}

func retryAfter(l *rate.Limiter, now time.Time) uint64 {
	missing := 1 - l.TokensAt(now)
	return uint64(max(1, math.Ceil(missing/float64(l.Limit()))))
}

type LoginLimiter struct{ b *buckets[[2]string] }

func NewLoginLimiter(cfg config.LoginLimitConfig) *LoginLimiter {
	return &LoginLimiter{b: newBuckets[[2]string](cfg.MaxFailures, time.Duration(cfg.WindowSecs)*time.Second)}
}

func (l *LoginLimiter) Check(ip, email string, now time.Time) (uint64, bool) {
	bucket, ok := l.b.peek([2]string{ip, email})
	if !ok || bucket.TokensAt(now) >= 1 {
		return 0, true
	}
	return retryAfter(bucket, now), false
}

func (l *LoginLimiter) RecordFailure(ip, email string, now time.Time) {
	l.b.get([2]string{ip, email}, now).AllowN(now, 1)
}

func (l *LoginLimiter) Reset(ip, email string) { l.b.forget([2]string{ip, email}) }

func (l *LoginLimiter) Len() int { return l.b.Len() }

type IngestLimiter struct{ b *buckets[uuid.UUID] }

func NewIngestLimiter(cfg config.IngestConfig) *IngestLimiter {
	return &IngestLimiter{b: newBuckets[uuid.UUID](cfg.RateLimitPerMinute, time.Minute)}
}

func (l *IngestLimiter) Hit(key uuid.UUID, now time.Time) (uint64, bool) {
	bucket := l.b.get(key, now)
	if bucket.AllowN(now, 1) {
		return 0, true
	}
	return retryAfter(bucket, now), false
}

func (l *IngestLimiter) Len() int { return l.b.Len() }
