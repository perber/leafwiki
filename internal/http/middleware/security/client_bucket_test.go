package security

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// newBucketContext builds a gin context whose Request carries remoteAddr as
// the peer. Gin's default engine honours X-Forwarded-For from any peer, which
// lets the table reach every address form c.ClientIP() can return.
func newBucketContext(t *testing.T, remoteAddr, forwardedFor string, unixSocket bool) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if unixSocket {
		ctx := context.WithValue(req.Context(), http.LocalAddrContextKey, &net.UnixAddr{Name: "/run/leafwiki.sock", Net: "unix"})
		req = req.WithContext(ctx)
	}
	c.Request = req
	return c
}

func TestClientBucket_GroupsIPv6ByPrefix(t *testing.T) {
	tests := []struct {
		name         string
		remoteAddr   string
		forwardedFor string
		unixSocket   bool
		want         string
	}{
		{name: "IPv4 stays as is", remoteAddr: "192.0.2.10:4711", want: "192.0.2.10"},
		{name: "IPv4 from forwarding header stays as is", remoteAddr: "10.0.0.1:80", forwardedFor: "198.51.100.7", want: "198.51.100.7"},
		{name: "IPv4-mapped IPv6 becomes IPv4", remoteAddr: "[::ffff:192.0.2.10]:4711", want: "192.0.2.10"},
		{name: "IPv4-mapped IPv6 from forwarding header becomes IPv4", remoteAddr: "10.0.0.1:80", forwardedFor: "::ffff:198.51.100.7", want: "198.51.100.7"},
		{name: "IPv6 is masked to its /64", remoteAddr: "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:4711", want: "2001:db8:1:2::"},
		{name: "IPv6 in the same /64 shares the bucket", remoteAddr: "[2001:db8:1:2:1::1]:4711", want: "2001:db8:1:2::"},
		{name: "IPv6 in a neighbouring /64 gets its own bucket", remoteAddr: "[2001:db8:1:3::1]:4711", want: "2001:db8:1:3::"},
		{name: "IPv6 loopback is masked to its /64", remoteAddr: "[::1]:4711", want: "::"},
		{name: "zoned IPv6 is unknown", remoteAddr: "[fe80::1%eth0]:4711", want: "unknown"},
		{name: "empty peer address is unknown", remoteAddr: "", want: "unknown"},
		{name: "peer address without a port is unknown", remoteAddr: "192.0.2.10", want: "unknown"},
		{name: "unix socket peer without forwarding header is unknown", remoteAddr: "@", unixSocket: true, want: "unknown"},
		{name: "unix socket peer with forwarding header uses the header", remoteAddr: "@", forwardedFor: "198.51.100.7", unixSocket: true, want: "198.51.100.7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newBucketContext(t, tt.remoteAddr, tt.forwardedFor, tt.unixSocket)

			if got := ClientBucket(c); got != tt.want {
				t.Fatalf("ClientBucket() = %q, want %q (ClientIP=%q)", got, tt.want, c.ClientIP())
			}
		})
	}
}

func TestClientBucket_ReadsTheRequestPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var got string
	router := gin.New()
	router.GET("/", func(c *gin.Context) {
		got = ClientBucket(c)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "[2001:db8:5:6:7:8:9:a]:1234"
	router.ServeHTTP(httptest.NewRecorder(), req)

	if want := "2001:db8:5:6::"; got != want {
		t.Fatalf("ClientBucket() = %q, want %q", got, want)
	}
}
