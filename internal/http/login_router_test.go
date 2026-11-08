package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

type loginWireResponse struct {
	Status      int
	ContentType string
	RetryAfter  string
	Body        string // compacted JSON
	Headers     string // every response header, sorted, for comparing answers with each other
}

func loginWire(t *testing.T, router http.Handler, remoteAddr, identifier, password string) loginWireResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"identifier": identifier, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var compact bytes.Buffer
	if err := json.Compact(&compact, rec.Body.Bytes()); err != nil {
		t.Fatalf("login response is not JSON: %v (%q)", err, rec.Body.String())
	}
	return loginWireResponse{
		Status:      rec.Code,
		ContentType: rec.Header().Get("Content-Type"),
		RetryAfter:  rec.Header().Get("Retry-After"),
		Body:        compact.String(),
		Headers:     sortedHeaders(rec.Header()),
	}
}

func sortedHeaders(h http.Header) string {
	lines := make([]string, 0, len(h))
	for name, values := range h {
		lines = append(lines, name+": "+strings.Join(values, ", "))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// A locked account must not be told apart from a wrong identifier or
// password: a distinct answer would confirm that the account exists to anyone
// who can trip the lock. Each request comes from its own client address so
// the per-client rate limit never interferes.
func TestLogin_LockedAccountRespondsLikeInvalidCredentials(t *testing.T) {
	w := createWikiTestInstance(t) // seeds admin "admin" / "adminpassword"
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	unknownUser := loginWire(t, router, "198.51.100.1:40000", "no-such-user", "whatever")
	wrongPassword := loginWire(t, router, "198.51.100.2:40000", "admin", "wrong-guess")
	// Five failures lock the account (core/auth loginMaxFailures); together
	// with the wrong-password request above this trips the lock with one to
	// spare, so the correct password below is answered by the lock.
	for i := 0; i < 5; i++ {
		loginWire(t, router, "203.0.113.7:51000", "admin", "wrong-guess")
	}
	locked := loginWire(t, router, "198.51.100.3:40000", "admin", "adminpassword")

	want := loginWireResponse{
		Status:      http.StatusUnauthorized,
		ContentType: "application/json; charset=utf-8",
		Body:        `{"error":{"code":"auth_invalid_credentials","message":"Invalid credentials","template":"invalid credentials"}}`,
	}
	for name, got := range map[string]loginWireResponse{
		"unknown user":   unknownUser,
		"wrong password": wrongPassword,
		"locked account": locked,
	} {
		// Headers are compared against the wrong-password answer rather than a
		// fixed list, so any header that differs (a cookie, a Retry-After)
		// shows up without pinning middleware details here.
		if got.Headers != wrongPassword.Headers {
			t.Errorf("%s headers:\n got  %s\n want %s", name, got.Headers, wrongPassword.Headers)
		}
		got.Headers = ""
		if got != want {
			t.Errorf("%s:\n got  %+v\n want %+v", name, got, want)
		}
	}
}
