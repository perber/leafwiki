package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewRateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	rl := &rateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}

	return func(c *gin.Context) {
		// currently we use the clientIP
		// the rate limiting is not set on every endpoint, so this is acceptable
		key := c.ClientIP()
		now := time.Now()
		cutoff := now.Add(-rl.window)

		rl.mu.Lock()
		defer rl.mu.Unlock()

		events := rl.hits[key][:0]
		if old, ok := rl.hits[key]; ok {
			for _, t := range old {
				if t.After(cutoff) {
					events = append(events, t)
				}
			}
		}
		if len(events) >= rl.limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests, please try again later",
			})
			return
		}

		events = append(events, now)
		rl.hits[key] = events

		c.Next()
	}
}
