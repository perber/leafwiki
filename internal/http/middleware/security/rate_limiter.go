package security

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// KeyedLimiter is a per-key sliding-window rate limiter. NewRateLimiter wraps
// one as a ready-to-use Gin middleware for the common case (rate-limit every
// request through a route, abort-and-continue). KeyedLimiter itself is used
// directly by callers that only want to rate-limit a subset of requests
// handled inside a larger middleware — e.g. only requests presenting a Bearer
// API key, checked inline within InjectAPIKeyUser — where wrapping the whole
// downstream chain the way NewRateLimiter does isn't the right shape.
type KeyedLimiter struct {
	mu             sync.Mutex
	hits           map[string][]time.Time
	limit          int
	window         time.Duration
	resetOnSuccess bool
}

// NewKeyedLimiter creates a limiter allowing up to limit hits per window per
// key. If resetOnSuccess is true, NotifyResult(key, true) clears that key's
// history immediately, so repeated successes never accumulate toward the cap.
func NewKeyedLimiter(limit int, window time.Duration, resetOnSuccess bool) *KeyedLimiter {
	return &KeyedLimiter{
		hits:           make(map[string][]time.Time),
		limit:          limit,
		window:         window,
		resetOnSuccess: resetOnSuccess,
	}
}

func (rl *KeyedLimiter) cleanup() {
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

// Allow reports whether a hit for key is permitted right now, recording it if
// so. Returns false (without recording) once key is at its limit within the
// current window.
func (rl *KeyedLimiter) Allow(key string) bool {
	rl.cleanup()

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
		rl.hits[key] = events
		return false
	}

	events = append(events, now)
	rl.hits[key] = events
	return true
}

// NotifyResult tells the limiter whether the request keyed by key ultimately
// succeeded, so it can reset that key's count when configured to do so.
func (rl *KeyedLimiter) NotifyResult(key string, success bool) {
	if !rl.resetOnSuccess || !success {
		return
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.hits, key)
}

// ClientKey extracts the rate-limit key (client host) from a request. Shared
// by NewRateLimiter and any caller using a KeyedLimiter directly, so every
// rate-limited path in the app keys by the same value.
func ClientKey(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

// NewRateLimiter returns Gin middleware that rate-limits by client host,
// aborting with 429 once a key exceeds limit hits within window. If
// resetOnSuccess is true, a 2xx response clears that key's count immediately.
func NewRateLimiter(limit int, window time.Duration, resetOnSuccess bool) gin.HandlerFunc {
	rl := NewKeyedLimiter(limit, window, resetOnSuccess)

	return func(c *gin.Context) {
		key := ClientKey(c)
		if !rl.Allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many requests, please try again later",
			})
			return
		}

		c.Next()

		status := c.Writer.Status()
		rl.NotifyResult(key, status >= 200 && status < 300)
	}
}
