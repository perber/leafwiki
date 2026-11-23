package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter_NewKey(t *testing.T) {
	// This test ensures that the rate limiter doesn't panic when encountering a new key
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(3, time.Minute, false)

	// Create a test router with the rate limiter
	router := gin.New()
	router.Use(limiter)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make a request with a new IP (this would panic with the old code)
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRateLimiter_ExceedsLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(3, time.Minute, false)

	router := gin.New()
	router.Use(limiter)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make requests up to the limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:1234"
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// The next request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.2:1234"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestRateLimiter_ReleasesLockAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := NewRateLimiter(1, time.Minute, false)

	router := gin.New()
	router.Use(limiter)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.4:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.4:1234"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected status 429, got %d", w.Code)
	}

	done := make(chan int, 1)

	go func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.5:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		done <- w.Code
	}()

	timeout := 2 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) / 2; remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case code := <-done:
		if code != http.StatusOK {
			t.Fatalf("Expected status 200 for different key after limit hit, got %d", code)
		}
	case <-timer.C:
		t.Fatal("Request blocked after limit hit; mutex was not released")
	}
}

func TestKeyedLimiter_AllowAndDeny(t *testing.T) {
	kl := NewKeyedLimiter(2, time.Minute, false)

	if !kl.Allow("k1") {
		t.Fatalf("expected first hit to be allowed")
	}
	if !kl.Allow("k1") {
		t.Fatalf("expected second hit to be allowed")
	}
	if kl.Allow("k1") {
		t.Fatalf("expected third hit within the limit window to be denied")
	}
	if !kl.Allow("k2") {
		t.Fatalf("expected a different key to be unaffected by k1's count")
	}
}

func TestKeyedLimiter_NotifyResultResetsOnSuccess(t *testing.T) {
	kl := NewKeyedLimiter(1, time.Minute, true)

	if !kl.Allow("k1") {
		t.Fatalf("expected first hit to be allowed")
	}
	kl.NotifyResult("k1", true)

	if !kl.Allow("k1") {
		t.Fatalf("expected hit to be allowed again after a successful result reset the count")
	}
}

func TestKeyedLimiter_NotifyResultDoesNotResetOnFailure(t *testing.T) {
	kl := NewKeyedLimiter(1, time.Minute, true)

	if !kl.Allow("k1") {
		t.Fatalf("expected first hit to be allowed")
	}
	kl.NotifyResult("k1", false)

	if kl.Allow("k1") {
		t.Fatalf("expected the count to remain after a failed result")
	}
}

// A key that goes quiet (all its events age out of the window) and is then
// hit again must not keep accumulating a growing, mostly-expired history —
// Allow filters it back down to just the new hit and, if that's its only
// remaining event, the entry stays minimal rather than growing unbounded in
// memory per key. Checked here by confirming a key that ages out and comes
// back is treated exactly like a fresh key (allowed up to the full limit
// again), not still "mostly at its limit" from stale events.
func TestKeyedLimiter_StaleEventsDontAccumulate(t *testing.T) {
	kl := NewKeyedLimiter(2, 50*time.Millisecond, false)

	if !kl.Allow("k1") {
		t.Fatalf("expected first hit to be allowed")
	}
	if !kl.Allow("k1") {
		t.Fatalf("expected second hit to be allowed")
	}
	time.Sleep(60 * time.Millisecond)

	// Both earlier events are now outside the window: this key should be
	// treated as if fresh again, allowed up to the full limit once more.
	if !kl.Allow("k1") {
		t.Fatalf("expected a hit after the window aged out to be allowed")
	}
	if !kl.Allow("k1") {
		t.Fatalf("expected a second hit after the window aged out to be allowed")
	}
	if kl.Allow("k1") {
		t.Fatalf("expected a third hit within the new window to be denied")
	}

	kl.mu.Lock()
	n := len(kl.hits["k1"])
	kl.mu.Unlock()
	if n != 2 {
		t.Fatalf("expected exactly 2 live events for k1, got %d (stale events not dropped)", n)
	}
}

// store's empty-events branch isn't reachable through Allow for any realistic
// limit (>= 1): the deny path only reaches store with zero events when
// limit <= 0, and the allow path always just appended "now". It still exists
// as a defensive guard (e.g. against a future caller or a limit of 0), and
// the table staying bounded in practice comes from makeRoom capping key
// count, not from this branch — covered directly since the public API can't
// reach it.
func TestKeyedLimiter_StoreDropsAKeyWithNoEventsLeft(t *testing.T) {
	kl := NewKeyedLimiter(2, time.Minute, false)
	kl.hits["k1"] = []time.Time{time.Now()}

	kl.mu.Lock()
	kl.store("k1", nil, true)
	_, stillThere := kl.hits["k1"]
	kl.mu.Unlock()

	if stillThere {
		t.Fatalf("expected store to remove a key given an empty events list")
	}
}

// The table must never grow past maxKeys, however many distinct keys are
// seen — the whole point of the bound (see keyedLimiterMaxKeys).
func TestKeyedLimiter_TableStaysBoundedUnderManyDistinctKeys(t *testing.T) {
	kl := NewKeyedLimiter(5, time.Hour, false)
	kl.maxKeys = 50

	for i := range 500 {
		kl.Allow(fmt.Sprintf("client-%d", i))
	}

	kl.mu.Lock()
	n := len(kl.hits)
	kl.mu.Unlock()
	if n > kl.maxKeys {
		t.Fatalf("table has %d entries, want at most maxKeys (%d)", n, kl.maxKeys)
	}
}

// Eviction must never target the key currently being inserted — otherwise a
// client could be rate-limited (wrongly denied or, worse, its own fresh hit
// silently discarded) by the very act of making room for itself.
func TestKeyedLimiter_MakeRoomNeverEvictsTheKeyBeingInserted(t *testing.T) {
	kl := NewKeyedLimiter(5, time.Hour, false)
	kl.maxKeys = 10

	for i := range kl.maxKeys {
		kl.Allow(fmt.Sprintf("client-%d", i))
	}

	// The table is now at capacity; inserting one more distinct key must
	// still succeed and be recorded, not be the one evicted to make room.
	if !kl.Allow("newcomer") {
		t.Fatalf("expected a brand-new key to be allowed even at capacity")
	}
	kl.mu.Lock()
	_, present := kl.hits["newcomer"]
	kl.mu.Unlock()
	if !present {
		t.Fatalf("expected the newly inserted key to remain in the table, not be its own eviction victim")
	}
}

// At capacity, the key whose most recent hit is oldest — the one closest to
// aging out of its own window anyway — is the one evicted, same idea as
// loginAttemptTracker.makeRoom.
func TestKeyedLimiter_MakeRoomEvictsTheLeastRecentlyActiveKey(t *testing.T) {
	kl := NewKeyedLimiter(5, time.Hour, false)
	kl.maxKeys = 2

	kl.Allow("oldest")
	time.Sleep(5 * time.Millisecond)
	kl.Allow("newer")

	kl.Allow("triggers-eviction")

	kl.mu.Lock()
	_, oldestStillThere := kl.hits["oldest"]
	_, newerStillThere := kl.hits["newer"]
	kl.mu.Unlock()
	if oldestStillThere {
		t.Fatalf("expected the least recently active key to be evicted")
	}
	if !newerStillThere {
		t.Fatalf("expected the more recently active key to survive")
	}
}

func TestRateLimiter_WindowExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use a very short window for testing
	limiter := NewRateLimiter(2, 100*time.Millisecond, false)

	router := gin.New()
	router.Use(limiter)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make requests up to the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.3:1234"
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// Wait for the window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be able to make another request
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.3:1234"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 after window expired, got %d", w.Code)
	}
}
