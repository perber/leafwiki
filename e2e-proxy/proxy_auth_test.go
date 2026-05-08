package e2eproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// doProxy makes a GET request through the nginx proxy.
// Set testUser to non-empty to populate X-Test-User (nginx converts it to Remote-User).
func doProxy(t *testing.T, path, testUser string, extraHeaders map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, proxyURL+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if testUser != "" {
		req.Header.Set("X-Test-User", testUser)
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func readBody(t *testing.T, r *http.Response) string {
	t.Helper()
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return strings.TrimSpace(string(b))
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	body := readBody(t, resp)
	if resp.StatusCode != want {
		t.Errorf("expected HTTP %d, got %d — body: %s", want, resp.StatusCode, body)
	}
}

// loginAdmin obtains an access-token cookie by logging in as admin directly
// via the proxy (the login endpoint is public and unaffected by proxy auth).
func loginAdmin(t *testing.T) string {
	t.Helper()
	payload := `{"identifier":"admin","password":"admin"}`
	req, err := http.NewRequest(http.MethodPost, proxyURL+"/api/auth/login", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("build login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("login request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed %d: %s", resp.StatusCode, b)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "leafwiki_at" {
			return c.Value
		}
	}
	t.Fatal("no leafwiki_at cookie in login response")
	return ""
}

// TestProxyAuth_ValidUser_Admin verifies that a request with X-Test-User: admin
// (converted to Remote-User: admin by nginx) authenticates successfully.
func TestProxyAuth_ValidUser_Admin(t *testing.T) {
	// Use an authenticated endpoint: GET /api/users requires auth.
	resp := doProxy(t, "/api/users", "admin", nil)
	assertStatus(t, resp, http.StatusOK)
}

// TestProxyAuth_UnknownUser verifies that a proxy-provided username that does
// not exist in LeafWiki results in 401.
func TestProxyAuth_UnknownUser(t *testing.T) {
	resp := doProxy(t, "/api/users", "no-such-user-xyz", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestProxyAuth_NoHeader_ProtectedRoute verifies that a request without
// X-Test-User (so no Remote-User forwarded) is rejected on a protected route.
func TestProxyAuth_NoHeader_ProtectedRoute(t *testing.T) {
	resp := doProxy(t, "/api/users", "", nil)
	assertStatus(t, resp, http.StatusUnauthorized)
}

// TestProxyAuth_PublicRoute_NoHeader verifies that public endpoints remain
// reachable even when no Remote-User header is forwarded.
func TestProxyAuth_PublicRoute_NoHeader(t *testing.T) {
	resp := doProxy(t, "/api/config", "", nil)
	assertStatus(t, resp, http.StatusOK)
}

// TestProxyAuth_PublicRoute_WithUser verifies that public endpoints also work
// when a valid Remote-User is forwarded (the user is set in context, but
// unauthenticated access is still allowed on public routes).
func TestProxyAuth_PublicRoute_WithUser(t *testing.T) {
	resp := doProxy(t, "/api/config", "admin", nil)
	assertStatus(t, resp, http.StatusOK)
}

// TestProxyAuth_ConfigResponse_Roundtrip verifies the /api/config response is
// valid JSON — a basic smoke test that the proxy doesn't mangle the response.
func TestProxyAuth_ConfigResponse_Roundtrip(t *testing.T) {
	resp := doProxy(t, "/api/config", "", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["authDisabled"]; !ok {
		t.Error("expected 'authDisabled' field in /api/config response")
	}
}

// TestProxyAuth_FallbackToJWT verifies that standard JWT cookie auth still
// works when no Remote-User header is forwarded (proxy auth is additive, not
// a replacement for JWT).
func TestProxyAuth_FallbackToJWT(t *testing.T) {
	token := loginAdmin(t)

	req, err := http.NewRequest(http.MethodGet, proxyURL+"/api/users", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// No X-Test-User — no Remote-User forwarded by nginx.
	// But we carry the JWT access-token cookie.
	req.AddCookie(&http.Cookie{Name: "leafwiki_at", Value: token})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	assertStatus(t, resp, http.StatusOK)
}

// TestProxyAuth_DirectRemoteUserHeader verifies that a client cannot bypass
// proxy auth by sending Remote-User directly without going through nginx.
// Because nginx strips and re-sets the header, a client sending Remote-User
// directly to nginx will have it overwritten by the value of X-Test-User
// (which is empty here), so LeafWiki receives an empty Remote-User and falls
// back to requiring JWT — which we don't provide.
func TestProxyAuth_DirectRemoteUserInjection(t *testing.T) {
	// Send Remote-User directly without X-Test-User. nginx will replace it
	// with the (empty) X-Test-User value, so LeafWiki sees Remote-User: "".
	resp := doProxy(t, "/api/users", "", map[string]string{
		"Remote-User": "admin",
	})
	// Remote-User is overwritten by nginx → empty → no proxy auth → no JWT → 401
	assertStatus(t, resp, http.StatusUnauthorized)
}
