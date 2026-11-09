package http_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/test_utils"
)

// Peers whose address cannot be attributed (empty, no port, not an address,
// zoned) all share the "unknown" budget, so they are throttled together and
// never get a budget of their own.
func TestLogin_UnattributablePeersShareOneThrottledBudget(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	unattributable := []string{"", "203.0.113.7", "not-an-address", "[fe80::1%eth0]:5000"}
	for i := 0; i < freeLoginAttempts; i++ {
		peer := unattributable[i%len(unattributable)]
		if got := loginWire(t, router, peer, "admin", "wrong-guess"); got.Status != http.StatusUnauthorized {
			t.Fatalf("wrong password %d from %q: status %d, want 401", i+1, peer, got.Status)
		}
	}

	for _, peer := range unattributable {
		got := loginWire(t, router, peer, "admin", "adminpassword")
		if got.Status != http.StatusUnauthorized || got.Body != invalidCredentialsBody {
			t.Errorf("peer %q: expected the throttled answer, got %d %s", peer, got.Status, got.Body)
		}
	}
	if got := loginWire(t, router, "203.0.113.8:51000", "admin", "adminpassword"); got.Status != http.StatusOK {
		t.Errorf("an attributable client was held back: %d %s", got.Status, got.Body)
	}
}

// A peer written as an IPv4-mapped IPv6 address is the same client as its IPv4
// form, directly and in X-Forwarded-For behind a listed proxy.
func TestLogin_IPv4MappedPeerIsTheSameClientAsItsIPv4Form(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	trusted, err := authmw.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.TrustedProxies = trusted })

	spellings := []struct {
		peer    string
		headers []string
	}{
		{peer: "203.0.113.7:51000"},
		{peer: "[::ffff:203.0.113.7]:51000"},
		{peer: "10.0.0.1:5000", headers: []string{"X-Forwarded-For", "203.0.113.7"}},
		{peer: "10.0.0.1:5000", headers: []string{"X-Forwarded-For", "::ffff:203.0.113.7"}},
		{peer: "[::ffff:10.0.0.1]:5000", headers: []string{"X-Forwarded-For", "203.0.113.7"}},
	}
	for i := 0; i < freeLoginAttempts; i++ {
		s := spellings[i%len(spellings)]
		loginWire(t, router, s.peer, "admin", "wrong-guess", s.headers...)
	}
	for _, s := range spellings {
		got := loginWire(t, router, s.peer, "admin", "adminpassword", s.headers...)
		if got.Status != http.StatusUnauthorized || got.Body != invalidCredentialsBody {
			t.Errorf("peer %s headers %v: expected the throttled answer, got %d %s", s.peer, s.headers, got.Status, got.Body)
		}
	}
}

// An unparsable or zoned X-Forwarded-For from a listed proxy falls back to the
// proxy's own bucket, so changing the text never yields a fresh budget.
func TestLogin_UnparsableForwardedForFallsBackToTheProxyBucket(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	trusted, err := authmw.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.TrustedProxies = trusted })

	values := []string{"fe80::1%eth0", "garbage-1", "garbage-2", "203.0.113.7:51000", "1.2.3.4, garbage-3", ""}
	for i := 0; i < freeLoginAttempts; i++ {
		loginWire(t, router, "10.0.0.1:5000", "admin", "wrong-guess", "X-Forwarded-For", values[i])
	}
	got := loginWire(t, router, "10.0.0.1:5000", "admin", "adminpassword", "X-Forwarded-For", values[5])
	if got.Status != http.StatusUnauthorized || got.Body != invalidCredentialsBody {
		t.Errorf("expected the throttled answer for the proxy's bucket, got %d %s", got.Status, got.Body)
	}
}

// A very long forwarding header is read like a short one and never panics.
func TestLogin_VeryLongForwardedForIsHandled(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	trusted, err := authmw.ParseTrustedProxies("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.TrustedProxies = trusted })

	long := strings.Repeat("192.0.2.1, ", 2000) + "203.0.113.7"
	for i := 0; i < freeLoginAttempts; i++ {
		loginWire(t, router, "10.0.0.1:5000", "admin", "wrong-guess", "X-Forwarded-For", long)
	}
	got := loginWire(t, router, "10.0.0.1:5000", "admin", "adminpassword", "X-Forwarded-For", "203.0.113.7")
	if got.Status != http.StatusUnauthorized {
		t.Errorf("a long header and a short one for the same client must share a budget, got %d", got.Status)
	}
}

// Header-less traffic from an unlisted peer never logs the warning, however
// many requests arrive at once, and a header logs it exactly once.
func TestClientAddress_WarningStaysQuietForHeaderlessRequestsUnderLoad(t *testing.T) {
	rig := newClientAddressRig(t, "10.0.0.0/8")

	run := func(headers ...string) {
		var wg sync.WaitGroup
		for g := 0; g < 32; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; i < 25; i++ {
					rig.get(t, fmt.Sprintf("198.51.100.%d:40000", g+1), headers...)
				}
			}(g)
		}
		wg.Wait()
	}

	run()
	if n := strings.Count(rig.logs.String(), untrustedForwardingMessage); n != 0 {
		t.Fatalf("warning logged %d times for header-less requests, want 0", n)
	}
	run("X-Forwarded-For", "1.2.3.4")
	if n := strings.Count(rig.logs.String(), untrustedForwardingMessage); n != 1 {
		t.Fatalf("warning logged %d times under load, want exactly 1", n)
	}
}

// The text of --trusted-proxy-ips is trimmed, may name IPv4-mapped addresses
// and CIDR ranges, and every spelling reaches gin as a valid entry that
// trusts exactly what was written.
func TestClientAddress_TrustedProxyListSpellings(t *testing.T) {
	raw := " 10.0.0.0/8 , ::ffff:192.0.2.1 ,2001:db8:aa::/48,\t198.51.100.4 ,, "
	rig := newClientAddressRig(t, raw)

	tests := []struct {
		peer string
		want string // client address resolved for X-Forwarded-For: 203.0.113.7
	}{
		{"10.9.9.9:1", "203.0.113.7"},
		{"192.0.2.1:1", "203.0.113.7"},
		{"[::ffff:192.0.2.1]:1", "203.0.113.7"},
		{"[2001:db8:aa:1::5]:1", "203.0.113.7"},
		{"198.51.100.4:1", "203.0.113.7"},
		{"192.0.2.2:1", "192.0.2.2"},
		{"[2001:db8:ab::5]:1", "2001:db8:ab::5"},
		{"198.51.100.5:1", "198.51.100.5"},
	}
	for _, tt := range tests {
		if got := rig.get(t, tt.peer, "X-Forwarded-For", "203.0.113.7"); got.ip != tt.want {
			t.Errorf("peer %s: client = %q, want %q", tt.peer, got.ip, tt.want)
		}
	}
}

func TestClientAddress_InvalidTrustedProxyEntriesAreRejectedBeforeTheRouter(t *testing.T) {
	for _, raw := range []string{"10.0.0.1, not-an-ip", "10.0.0.0/33", "10.0.0.1/", "2001:db8::/129", "10.0.0.1;10.0.0.2", "10.0.0.1%eth0"} {
		if _, err := authmw.ParseTrustedProxies(raw); err == nil {
			t.Errorf("ParseTrustedProxies(%q) accepted an invalid entry", raw)
		}
	}
}
