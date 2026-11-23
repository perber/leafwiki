package security

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// keyedLimiterMaxKeys bounds the table, mirroring loginAttemptTracker's
// loginTrackerMaxEntries (internal/core/auth/login_attempt_tracker.go) for
// the same reason: key (see ClientBucket) comes from the network, so the
// table cannot be left to grow with it.
const keyedLimiterMaxKeys = 10000

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
	maxKeys        int
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
		maxKeys:        keyedLimiterMaxKeys,
	}
}

// Allow reports whether a hit for key is permitted right now, recording it if
// so. Returns false (without recording) once key is at its limit within the
// current window. Only touches key's own history — no per-call scan of every
// other key — and drops the entry entirely once it has no events left in the
// window, so a key that goes quiet doesn't keep a permanent map slot (see
// makeRoom for how the table as a whole stays bounded).
func (rl *KeyedLimiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	old, existed := rl.hits[key]
	events := old[:0]
	for _, t := range old {
		if t.After(cutoff) {
			events = append(events, t)
		}
	}

	if len(events) >= rl.limit {
		rl.store(key, events, existed)
		return false
	}

	events = append(events, now)
	rl.store(key, events, existed)
	return true
}

// store saves key's filtered event list, or forgets key entirely once it has
// none left. Must be called with rl.mu held.
func (rl *KeyedLimiter) store(key string, events []time.Time, existed bool) {
	if len(events) == 0 {
		delete(rl.hits, key)
		return
	}
	if !existed {
		rl.makeRoom()
	}
	rl.hits[key] = events
}

// makeRoom evicts one entry when the table is at its cap, picking the key
// whose most recent hit is oldest — the one closest to aging out of its own
// window anyway, so the cheapest to lose. Same idea as
// loginAttemptTracker.makeRoom, adapted to this type's data (a slice of hit
// times rather than a single next-allowed deadline). Must be called with
// rl.mu held, and only for a key not yet in the table — eviction never
// targets the caller's own about-to-be-inserted key. Never refuses to track
// and never empties the table, so a full table cannot switch the limiter off.
func (rl *KeyedLimiter) makeRoom() {
	if len(rl.hits) < rl.maxKeys {
		return
	}
	var victim string
	var victimLast time.Time
	found := false
	for k, events := range rl.hits {
		last := events[len(events)-1]
		if !found || last.Before(victimLast) {
			victim, victimLast = k, last
			found = true
		}
	}
	if found {
		delete(rl.hits, victim)
	}
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

// NewRateLimiter returns Gin middleware that rate-limits per client (see
// ClientBucket: the address behind a trusted proxy, IPv6 grouped by /64),
// aborting with 429 once a key exceeds limit hits within window. If
// resetOnSuccess is true, a 2xx response clears that key's count immediately.
func NewRateLimiter(limit int, window time.Duration, resetOnSuccess bool) gin.HandlerFunc {
	rl := NewKeyedLimiter(limit, window, resetOnSuccess)

	return func(c *gin.Context) {
		key := ClientBucket(c)
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
