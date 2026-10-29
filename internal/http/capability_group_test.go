package http_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

const capabilityTrustedAddr = "127.0.0.1:1234"

// capabilityProbe registers routes in a capability group. They report
// whether the request carries a user, so the tests can prove that no
// identity middleware ran.
type capabilityProbe struct{}

func (capabilityProbe) RegisterRoutes(ctx httpinternal.RouterContext) {
	g := ctx.CapabilityGroup("/probe")
	report := func(c *gin.Context) {
		_, hasUser := c.Get("user")
		c.JSON(http.StatusOK, gin.H{"hasUser": hasUser})
	}
	g.GET("/:secret", report)
	// A write in a capability group must never run: it would skip the
	// restore write gate and CSRF.
	g.POST("/:secret", report)
}

func newCapabilityWiki(t *testing.T) *wiki.Wiki {
	t.Helper()
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableAPIKeyManagement: true,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// newCapabilityRouter wires the probe next to the real registrars, with
// reverse-proxy auth (optionally auto-provisioning) and API keys enabled.
func newCapabilityRouter(t *testing.T, w *wiki.Wiki, autoCreate bool, override func(*httpinternal.RouterOptions)) *gin.Engine {
	t.Helper()
	trusted, err := authmw.ParseTrustedProxies("127.0.0.1")
	if err != nil {
		t.Fatalf("ParseTrustedProxies: %v", err)
	}
	opts := testRouterOptions()
	opts.HTTPRemoteUser = httpinternal.HTTPRemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		AutoCreate:     autoCreate,
		DefaultRole:    coreauth.RoleViewer,
		TrustedProxies: trusted,
		UserService:    w.UserService,
	}
	opts.APIKeyService = w.APIKeyService()
	if override != nil {
		override(&opts)
	}
	registrars := append(w.Registrars(), capabilityProbe{})
	return httpinternal.NewRouter(registrars, w.FrontendConfig(), opts)
}

func serveCapability(router http.Handler, method, url string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	req.RemoteAddr = capabilityTrustedAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// A capability route never sees an identity: not from a trusted proxy (known,
// unknown or to be auto-provisioned), not from an API key (valid-looking or
// broken), not from a session cookie.
func TestCapabilityGroup_RunsWithoutIdentity(t *testing.T) {
	tests := []struct {
		name       string
		autoCreate bool
		headers    map[string]string
	}{
		{name: "no identity", headers: nil},
		{name: "proxy asserts an unknown user", headers: map[string]string{"Remote-User": "nobody"}},
		{name: "proxy asserts a known user", headers: map[string]string{"Remote-User": "admin"}},
		{name: "proxy asserts a user that would be auto-created", autoCreate: true, headers: map[string]string{"Remote-User": "new-person"}},
		{name: "invalid API key", headers: map[string]string{"Authorization": "Bearer lw_bogus_secret"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newCapabilityWiki(t)
			router := newCapabilityRouter(t, w, tc.autoCreate, nil)

			rec := serveCapability(router, http.MethodGet, "/api/probe/s3cret", tc.headers)

			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hasUser":false`) {
				t.Fatalf("got %d %s, want 200 without a user", rec.Code, rec.Body.String())
			}
			if _, err := w.UserService().GetUserByIdentifier("new-person"); err == nil {
				t.Error("opening a capability route must never provision an account")
			}
		})
	}
}

func TestCapabilityGroup_IgnoresSessionCookie(t *testing.T) {
	w := newCapabilityWiki(t)
	router := newCapabilityRouter(t, w, false, nil)
	session := loginSession(t, router, "admin", "adminpassword")

	rec := session.send(router, http.MethodGet, "/api/probe/s3cret", strings.NewReader(""))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hasUser":false`) {
		t.Fatalf("got %d %s, want 200 without a user", rec.Code, rec.Body.String())
	}
}

// Normal routes keep their identity handling: the exemption is only for
// capability groups.
func TestCapabilityGroup_LeavesOtherRoutesUnchanged(t *testing.T) {
	w := newCapabilityWiki(t)
	router := newCapabilityRouter(t, w, false, nil)

	rec := serveCapability(router, http.MethodGet, "/api/tree", map[string]string{"Remote-User": "nobody"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/tree with an unknown proxy user = %d, want 401", rec.Code)
	}
}

func TestCapabilityGroup_SetsHardeningHeaders(t *testing.T) {
	w := newCapabilityWiki(t)
	router := newCapabilityRouter(t, w, false, nil)

	rec := serveCapability(router, http.MethodGet, "/api/probe/s3cret", nil)

	want := map[string]string{
		"Referrer-Policy": "no-referrer",
		"X-Robots-Tag":    "noindex, nofollow",
		"Cache-Control":   "no-store",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestCapabilityGroup_RejectsWrites(t *testing.T) {
	w := newCapabilityWiki(t)
	router := newCapabilityRouter(t, w, false, nil)

	rec := serveCapability(router, http.MethodPost, "/api/probe/s3cret", nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST in a capability group = %d, want 405", rec.Code)
	}
}

func TestCapabilityGroup_HonoursBasePath(t *testing.T) {
	w := newCapabilityWiki(t)
	router := newCapabilityRouter(t, w, false, func(o *httpinternal.RouterOptions) { o.BasePath = "/wiki" })

	if rec := serveCapability(router, http.MethodGet, "/wiki/api/probe/s3cret", nil); rec.Code != http.StatusOK {
		t.Fatalf("GET under base path = %d, want 200", rec.Code)
	}
}

// The secret in a capability URL never reaches the access log: matched
// requests log the route template, unmatched ones under a capability prefix
// log only the prefix. The query string is dropped in both cases.
func TestCapabilityGroup_KeepsSecretOutOfRequestLog(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantPath string
	}{
		{name: "matched route", url: "/api/probe/s3cret?also=s3cret", wantPath: "/api/probe/:secret"},
		{name: "unmatched path below the prefix", url: "/api/probe/s3cret/unknown?also=s3cret", wantPath: "/api/probe/…"},
		{name: "doubled slash before the prefix", url: "/api//probe/s3cret", wantPath: "/api/probe/…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			w := newCapabilityWiki(t)
			router := newCapabilityRouter(t, w, false, nil)
			serveCapability(router, http.MethodGet, tc.url, nil)

			var line string
			for _, l := range strings.Split(logs.String(), "\n") {
				if strings.Contains(l, `msg="http request"`) {
					line = l
				}
			}
			if line == "" {
				t.Fatalf("no request log line in:\n%s", logs.String())
			}
			if strings.Contains(line, "s3cret") {
				t.Errorf("request log leaks the secret: %s", line)
			}
			if !strings.Contains(line, "path="+tc.wantPath) {
				t.Errorf("request log path: got %s, want path=%s", line, tc.wantPath)
			}
		})
	}
}
