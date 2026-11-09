package http_test

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

const untrustedForwardingMessage = "forwarding header from an untrusted peer ignored"

// clientAddressRig is a gin engine configured by the production
// configureClientAddress, with one route that reports what handlers see.
type clientAddressRig struct {
	engine *gin.Engine
	logs   *bytes.Buffer
}

type resolvedClient struct{ ip, bucket string }

func newClientAddressRig(t *testing.T, trustedRaw string) *clientAddressRig {
	t.Helper()
	trusted, err := auth_middleware.ParseTrustedProxies(trustedRaw)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", trustedRaw, err)
	}
	return newClientAddressRigFor(t, trusted)
}

func newClientAddressRigFor(t *testing.T, trusted *auth_middleware.TrustedProxies) *clientAddressRig {
	t.Helper()
	return newClientAddressRigOn(t, trusted, false)
}

// newClientAddressRigOn is the rig for a server that listens on a unix socket
// (unixListener true) or on TCP.
func newClientAddressRigOn(t *testing.T, trusted *auth_middleware.TrustedProxies, unixListener bool) *clientAddressRig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rig := &clientAddressRig{engine: gin.New(), logs: &bytes.Buffer{}}
	if err := httpinternal.ConfigureClientAddress(rig.engine, trusted, unixListener); err != nil {
		t.Fatalf("ConfigureClientAddress: %v", err)
	}
	rig.engine.Use(httpinternal.WarnOnUntrustedForwarding(slog.New(slog.NewTextHandler(rig.logs, nil)), trusted))
	rig.engine.GET("/who", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP()+"|"+security.ClientBucket(c))
	})
	return rig
}

// get sends one request from peer; headers come in name, value pairs.
func (r *clientAddressRig) get(t *testing.T, peer string, headers ...string) resolvedClient {
	t.Helper()
	return r.serve(t, httptest.NewRequest(http.MethodGet, "/who", nil), peer, headers)
}

func (r *clientAddressRig) serve(t *testing.T, req *http.Request, peer string, headers []string) resolvedClient {
	t.Helper()
	req.RemoteAddr = peer
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	r.engine.ServeHTTP(rec, req)
	ip, bucket, _ := strings.Cut(rec.Body.String(), "|")
	return resolvedClient{ip: ip, bucket: bucket}
}

func TestClientAddress_IgnoresForwardingHeaderFromUntrustedPeer(t *testing.T) {
	tests := []struct {
		name    string
		trusted string // peer 198.51.100.9 is never in it
	}{
		{"no trusted proxies", ""},
		{"other proxies are trusted", "10.0.0.0/8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newClientAddressRig(t, tt.trusted)
			for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
				for _, forged := range []string{"1.2.3.4", "5.6.7.8", "9.9.9.9, 1.2.3.4"} {
					got := rig.get(t, "198.51.100.9:40000", header, forged)
					if got.ip != "198.51.100.9" || got.bucket != "198.51.100.9" {
						t.Errorf("%s: %s = %+v, want the TCP peer 198.51.100.9", header, forged, got)
					}
				}
			}
		})
	}
}

func TestClientAddress_ReadsForwardedForOnlyBehindATrustedProxy(t *testing.T) {
	const proxy = "10.0.0.1:5000"
	tests := []struct {
		name    string
		headers []string
		want    string
	}{
		{"first client", []string{"X-Forwarded-For", "203.0.113.7"}, "203.0.113.7"},
		{"second client", []string{"X-Forwarded-For", "203.0.113.8"}, "203.0.113.8"},
		{"forged left-hand entry does not count", []string{"X-Forwarded-For", "1.2.3.4, 203.0.113.7"}, "203.0.113.7"},
		{"every hop must be listed", []string{"X-Forwarded-For", "203.0.113.7, 198.51.100.20"}, "198.51.100.20"},
		{"listed hops are skipped from the right", []string{"X-Forwarded-For", "203.0.113.7, 10.0.0.2"}, "203.0.113.7"},
		{"X-Real-IP never overrides X-Forwarded-For", []string{"X-Forwarded-For", "203.0.113.7", "X-Real-IP", "1.2.3.4"}, "203.0.113.7"},
		{"X-Real-IP alone is not read", []string{"X-Real-IP", "203.0.113.7"}, "10.0.0.1"},
		{"no header", nil, "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newClientAddressRig(t, "10.0.0.0/8")
			got := rig.get(t, proxy, tt.headers...)
			if got.ip != tt.want || got.bucket != tt.want {
				t.Errorf("client = %+v, want %s", got, tt.want)
			}
		})
	}
}

func TestClientAddress_NilTrustedProxiesTrustNobody(t *testing.T) {
	rig := newClientAddressRigFor(t, nil)
	got := rig.get(t, "10.0.0.1:5000", "X-Forwarded-For", "1.2.3.4")
	if got.ip != "10.0.0.1" {
		t.Errorf("client = %+v, want the TCP peer 10.0.0.1", got)
	}
}

// An IPv4-mapped IPv6 entry names one host. Passed to gin as written it would
// become the IPv6 prefix ::ffff:10.0.0.1/32 and trust a far wider range.
func TestClientAddress_MappedEntryTrustsOnlyThatAddress(t *testing.T) {
	rig := newClientAddressRig(t, "::ffff:10.0.0.1")

	if got := rig.get(t, "10.0.0.1:5000", "X-Forwarded-For", "203.0.113.7"); got.ip != "203.0.113.7" {
		t.Errorf("listed peer: client = %+v, want 203.0.113.7", got)
	}
	if got := rig.get(t, "10.0.0.2:5000", "X-Forwarded-For", "203.0.113.7"); got.ip != "10.0.0.2" {
		t.Errorf("unlisted neighbour: client = %+v, want its own address 10.0.0.2", got)
	}
}

// requestOnUnixSocket is a request as the server sees it when it listens on a
// unix socket: the local address is a unix address and the peer has no IP.
func requestOnUnixSocket() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/who", nil)
	addr := net.Addr(&net.UnixAddr{Name: "/run/leafwiki.sock", Net: "unix"})
	return req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, addr))
}

// gin trusts every unix-socket peer whatever the list says, so anyone who can
// open the socket could pick their own address with the header. Without a
// listed proxy the header is not read on a socket.
func TestClientAddress_UnixSocketWithoutListedProxyIgnoresForwardingHeader(t *testing.T) {
	trusted, err := auth_middleware.ParseTrustedProxies("")
	if err != nil {
		t.Fatal(err)
	}
	rig := newClientAddressRigOn(t, trusted, true)

	for _, forwarded := range []string{"203.0.113.7", "203.0.113.8"} {
		got := rig.serve(t, requestOnUnixSocket(), "@", []string{"X-Forwarded-For", forwarded})
		if got.bucket != "unknown" {
			t.Errorf("X-Forwarded-For %s over a socket with no listed proxy: bucket = %q, want %q", forwarded, got.bucket, "unknown")
		}
	}
	if !strings.Contains(rig.logs.String(), untrustedForwardingMessage) {
		t.Errorf("an ignored header on a socket should log the warning once: %q", rig.logs.String())
	}
}

// Listing a proxy is how the operator says that a proxy in front of the socket
// sets the header, so it is read then; without the header the peer has no
// address of its own and falls into the shared bucket.
func TestClientAddress_UnixSocketWithListedProxyReadsForwardingHeader(t *testing.T) {
	trusted, err := auth_middleware.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	rig := newClientAddressRigOn(t, trusted, true)

	got := rig.serve(t, requestOnUnixSocket(), "@", []string{"X-Forwarded-For", "203.0.113.7"})
	if got.ip != "203.0.113.7" || got.bucket != "203.0.113.7" {
		t.Errorf("with header: client = %+v, want 203.0.113.7", got)
	}

	got = rig.serve(t, requestOnUnixSocket(), "@", nil)
	if got.bucket != "unknown" {
		t.Errorf("without header: bucket = %q, want %q", got.bucket, "unknown")
	}
	if strings.Contains(rig.logs.String(), untrustedForwardingMessage) {
		t.Errorf("a unix-socket peer with a listed proxy is believed and must not trigger the warning: %s", rig.logs.String())
	}
}

func TestClientAddress_WarnsOnceAboutForwardingHeaderFromUnlistedPeer(t *testing.T) {
	tests := []struct {
		name      string
		peer      string
		headers   []string
		wantCount int
	}{
		{"unlisted peer with X-Forwarded-For", "198.51.100.9:1", []string{"X-Forwarded-For", "1.2.3.4"}, 1},
		{"unlisted peer with X-Real-IP", "198.51.100.9:1", []string{"X-Real-IP", "1.2.3.4"}, 1},
		{"listed peer with X-Forwarded-For", "10.0.0.1:1", []string{"X-Forwarded-For", "1.2.3.4"}, 0},
		{"unlisted peer without a header", "198.51.100.9:1", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rig := newClientAddressRig(t, "10.0.0.0/8")
			for i := 0; i < 3; i++ {
				rig.get(t, tt.peer, tt.headers...)
			}
			logs := rig.logs.String()
			if got := strings.Count(logs, untrustedForwardingMessage); got != tt.wantCount {
				t.Errorf("warning logged %d times, want %d; log: %s", got, tt.wantCount, logs)
			}
			if tt.wantCount > 0 {
				for _, want := range []string{"level=WARN", "--trusted-proxy-ips"} {
					if !strings.Contains(logs, want) {
						t.Errorf("log lacks %q: %s", want, logs)
					}
				}
			}
		})
	}
}

// The once-ness belongs to the router, not the process: a second router warns
// again, which keeps tests independent and lets a restart warn afresh.
func TestClientAddress_WarningIsOncePerRouter(t *testing.T) {
	for i := 0; i < 2; i++ {
		rig := newClientAddressRig(t, "")
		rig.get(t, "198.51.100.9:1", "X-Forwarded-For", "1.2.3.4")
		if got := strings.Count(rig.logs.String(), untrustedForwardingMessage); got != 1 {
			t.Errorf("router %d: warning logged %d times, want 1", i, got)
		}
	}
}
