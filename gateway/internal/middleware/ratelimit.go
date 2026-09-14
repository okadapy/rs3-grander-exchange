package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimit implements a simple per-IP token bucket. Cleanup runs every
// 5 minutes and drops buckets idle for more than 10 minutes. This is
// sufficient for a single gateway replica; if you scale the gateway
// horizontally, swap this out for a Redis-backed sliding window.
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	rps := cfg.RequestsPerSecond
	if rps <= 0 {
		rps = 50
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = 100
	}

	var (
		mu      sync.Mutex
		buckets = map[string]*bucket{}
	)

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			cutoff := time.Now().Add(-10 * time.Minute)
			mu.Lock()
			for k, b := range buckets {
				if b.lastSeen.Before(cutoff) {
					delete(buckets, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		// Skip WebSocket upgrades — those are long-lived and handled by
		// the realtime service's own message-level rate limit.
		if c.GetHeader("Upgrade") != "" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{tokens: float64(burst), lastSeen: now}
			buckets[ip] = b
		}
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * rps
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
		b.lastSeen = now

		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
