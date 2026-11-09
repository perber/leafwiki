package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// limitedEngine is an engine configured the way the router configures it:
// forwarding headers are believed only from trusted peers, and only
// X-Forwarded-For is read.
func limitedEngine(t *testing.T, limit int, trusted ...string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	if trusted == nil {
		trusted = []string{}
	}
	if err := engine.SetTrustedProxies(trusted); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}
	engine.RemoteIPHeaders = []string{"X-Forwarded-For"}
	engine.Use(NewRateLimiter(limit, time.Minute, false))
	engine.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine
}

func hit(engine *gin.Engine, remoteAddr, forwardedFor string) int {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec.Code
}

// Addresses of one /64 belong to one subscriber. If each had a budget of its
// own, switching to a fresh address would be a way around the limit.
func TestRateLimiter_SharesTheBudgetAcrossAnIPv6Prefix(t *testing.T) {
	engine := limitedEngine(t, 2)

	for i, addr := range []string{"[2001:db8:1:2::1]:1000", "[2001:db8:1:2::2]:1000"} {
		if got := hit(engine, addr, ""); got != http.StatusOK {
			t.Fatalf("hit %d from %s: got %d, want 200", i+1, addr, got)
		}
	}
	if got := hit(engine, "[2001:db8:1:2::ffff]:1000", ""); got != http.StatusTooManyRequests {
		t.Fatalf("third address of the same /64: got %d, want 429", got)
	}
	if got := hit(engine, "[2001:db8:1:3::1]:1000", ""); got != http.StatusOK {
		t.Fatalf("address of a different /64: got %d, want 200", got)
	}
}

// Behind a listed proxy every request arrives from the proxy's address; the
// budget has to follow the client the proxy reports, or one visitor could use
// up everybody's.
func TestRateLimiter_FollowsTheClientBehindATrustedProxy(t *testing.T) {
	engine := limitedEngine(t, 2, "10.0.0.0/8")
	const proxy = "10.0.0.1:4000"

	for i := 0; i < 2; i++ {
		if got := hit(engine, proxy, "203.0.113.7"); got != http.StatusOK {
			t.Fatalf("hit %d for the first client: got %d, want 200", i+1, got)
		}
	}
	if got := hit(engine, proxy, "203.0.113.7"); got != http.StatusTooManyRequests {
		t.Fatalf("first client over its budget: got %d, want 429", got)
	}
	if got := hit(engine, proxy, "203.0.113.8"); got != http.StatusOK {
		t.Fatalf("a different client behind the same proxy: got %d, want 200", got)
	}
}

// A client that is not a listed proxy cannot choose its own budget by writing
// a forwarding header.
func TestRateLimiter_IgnoresAForwardingHeaderFromADirectClient(t *testing.T) {
	engine := limitedEngine(t, 2)
	const peer = "198.51.100.9:5000"

	for i, forwarded := range []string{"203.0.113.1", "203.0.113.2"} {
		if got := hit(engine, peer, forwarded); got != http.StatusOK {
			t.Fatalf("hit %d: got %d, want 200", i+1, got)
		}
	}
	if got := hit(engine, peer, "203.0.113.3"); got != http.StatusTooManyRequests {
		t.Fatalf("a rotated forwarding header must not give a fresh budget: got %d, want 429", got)
	}
}
