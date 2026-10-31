package http_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// No request shape may put the token into the access log, including the ones
// that never reach a handler (redirects, 404s, odd methods, encoded paths).
func TestShares_AccessLog_NeverContainsTheToken(t *testing.T) {
	f := newSharesFixture(t)
	token := f.link(t, f.customer, true)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	tests := []struct{ method, url string }{
		{"GET", "/api/shared/" + token + "/page"},
		{"GET", "/api/shared/" + token + "/page/"},
		{"GET", "/api/shared/" + token + "/pages/" + f.contract + "/"},
		{"GET", "/api/shared//" + token + "/page"},
		{"GET", "/api/shared/" + token + "/page?t=" + token},
		{"GET", "/api/shared/" + token + "/nope"},
		{"GET", "/api/shared/" + token},
		{"GET", "/api/shared/%2e%2e/" + token + "/page"},
		{"GET", "/api/shared/" + url.PathEscape(token) + "%2Fpage"},
		{"GET", "/api/shared/" + token + "/pages/..%2f..%2f" + token},
		{"HEAD", "/api/shared/" + token + "/page"},
		{"OPTIONS", "/api/shared/" + token + "/page"},
		{"POST", "/api/shared/" + token + "/page"},
		{"DELETE", "/api/shared/" + token + "/pages/" + f.contract},
		{"GET", "/api/shared/" + strings.Repeat("A", 100000) + "/page"},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(tc.method, tc.url, nil)
		f.router.ServeHTTP(httptest.NewRecorder(), req)
	}

	if !strings.Contains(logs.String(), `msg="http request"`) {
		t.Fatalf("no request log lines captured")
	}
	if strings.Contains(logs.String(), token) || strings.Contains(logs.String(), token[4:20]) {
		t.Errorf("the token reached the log:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), strings.Repeat("A", 100)) {
		t.Errorf("a long path segment reached the log")
	}
}

// Odd URL shapes never serve a page they should not, and all failures keep the
// uniform 404 body.
func TestShares_GetSharedPage_OddURLShapesDoNotLeak(t *testing.T) {
	f := newSharesFixture(t)
	single := f.link(t, f.customer, false)

	tests := []struct {
		name   string
		method string
		url    string
		want   int
	}{
		{"encoded slash in id", "GET", "/api/shared/" + single + "/pages/" + f.customer + "%2F..%2F" + f.internal, 404},
		{"dot dot id", "GET", "/api/shared/" + single + "/pages/..", 404},
		{"token in query only", "GET", "/api/shared/page?token=" + single, 404},
		{"upper-case api segment", "GET", "/api/Shared/" + single + "/page", 404},
		{"upper-case token", "GET", "/api/shared/" + strings.ToUpper(single) + "/page", 404},
		{"very long token", "GET", "/api/shared/" + strings.Repeat("A", 100000) + "/page", 404},
		{"another page of a single link", "GET", "/api/shared/" + single + "/pages/" + f.internal, 404},
		{"post finds no route", "POST", "/api/shared/" + single + "/page", 404},
		{"head finds no route", "HEAD", "/api/shared/" + single + "/page", 404},
	}
	var wantBody string
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, nil)
			req.RemoteAddr = "192.0.2." + "1:1"
			rec := httptest.NewRecorder()
			f.router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == 404 && strings.Contains(rec.Body.String(), "share_link_not_found") {
				if wantBody == "" {
					wantBody = rec.Body.String()
				}
				if rec.Body.String() != wantBody {
					t.Errorf("body differs: %s vs %s", rec.Body.String(), wantBody)
				}
			}
			if tc.want == 404 && strings.Contains(rec.Body.String(), f.internal) {
				t.Errorf("404 body leaks a page id")
			}
		})
	}
}

// Exactly 20 failures are answered 404; a success resets the count; IPv6
// clients are keyed too.
func TestShares_GetSharedPage_LimitEdgeCases(t *testing.T) {
	f := newSharesFixture(t)
	valid := f.link(t, f.customer, false)
	bad := "/api/shared/not-a-token/page"
	const v6 = "[2001:db8::1]:4000"

	for i := 1; i <= 19; i++ {
		f.get(bad, v6)
	}
	if rec := f.get("/api/shared/"+valid+"/page", v6); rec.Code != http.StatusOK {
		t.Fatalf("valid link = %d", rec.Code)
	}
	// success reset the counter: another 20 failures are still plain 404s
	for i := 1; i <= 20; i++ {
		if rec := f.get(bad, v6); rec.Code != http.StatusNotFound {
			t.Fatalf("failure %d after reset = %d, want 404", i, rec.Code)
		}
	}
	if rec := f.get(bad, v6); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("21st failure = %d, want 429", rec.Code)
	}
	// the limited client must not be able to tell a real-but-out-of-scope page from an unknown one
	a := f.get("/api/shared/"+valid+"/pages/"+f.internal, v6)
	b := f.get("/api/shared/"+valid+"/pages/nope", v6)
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Errorf("throttled responses differ: %d %s vs %d %s", a.Code, a.Body.String(), b.Code, b.Body.String())
	}
}

func TestShares_GetSharedPage_ConcurrentReads(t *testing.T) {
	f := newSharesFixture(t)
	token := f.link(t, f.customer, true)
	done := make(chan int, 40)
	for i := 0; i < 40; i++ {
		go func(i int) {
			u := "/api/shared/" + token + "/page"
			if i%2 == 1 {
				u = "/api/shared/bad" + token[:5] + "/page"
			}
			done <- f.get(u, "203.0.113.50:1").Code
		}(i)
	}
	ok, nf := 0, 0
	for i := 0; i < 40; i++ {
		switch <-done {
		case 200:
			ok++
		case 404:
			nf++
		default:
			t.Errorf("unexpected status")
		}
	}
	if ok != 20 || nf != 20 {
		t.Errorf("ok=%d notfound=%d, want 20/20", ok, nf)
	}
}
