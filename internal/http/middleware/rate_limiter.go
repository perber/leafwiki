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

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)

	for key, events := range rl.hits {
		n := 0
		for _, t := range events {
			if t.After(cutoff) {
				// keep this event within the active window
				events[n] = t
				n++
			}
		}

		if n == 0 {
			// no more events in the current window for this key; remove entry
			delete(rl.hits, key)
		} else {
			// shrink slice to the kept events
			rl.hits[key] = events[:n]
		}
	}
}

func NewRateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	rl := &rateLimiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}

	return func(c *gin.Context) {
		// perform cleanup based on the current time before processing this request
		rl.cleanup()
		// currently we use the clientIP
		// the rate limiting is not set on every endpoint, so this is acceptable
		key := c.ClientIP()
		now := time.Now()
		cutoff := now.Add(-rl.window)

		rl.mu.Lock()
		defer rl.mu.Unlock()

		var events []time.Time
		if old, ok := rl.hits[key]; ok {
			events = old[:0]
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
