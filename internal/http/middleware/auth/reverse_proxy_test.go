package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

type proxyFixture struct {
	userService *coreauth.UserService
	close       func() error
}

func cleanupWithErrorCheck(t *testing.T, name string, closeFn func() error) {
	t.Helper()

	t.Cleanup(func() {
		if err := closeFn(); err != nil {
			t.Errorf("close %s: %v", name, err)
		}
	})
}

func createProxyFixture(t *testing.T) *proxyFixture {
	t.Helper()

	storageDir := t.TempDir()
	userStore, err := coreauth.NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("create user store: %v", err)
	}

	userService := coreauth.NewUserService(userStore)
	if err := userService.InitDefaultAdmin("admin"); err != nil {
		_ = userStore.Close()
		t.Fatalf("init default admin: %v", err)
	}

	return &proxyFixture{
		userService: userService,
		close:       userStore.Close,
	}
}

func mustParseTrustedProxies(t *testing.T, raw string) *authmw.TrustedProxies {
	t.Helper()
	tp, err := authmw.ParseTrustedProxies(raw)
	if err != nil {
		t.Fatalf("ParseTrustedProxies(%q): %v", raw, err)
	}
	return tp
}

func proxyRouter(cfg authmw.RemoteUserConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authmw.InjectRemoteUser(cfg))
	r.GET("/test", func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		u := userVal.(*coreauth.User)
		c.JSON(http.StatusOK, gin.H{"username": u.Username})
	})
	return r
}

func TestInjectRemoteUser_Disabled(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        false,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	// Disabled → no user injected → handler returns 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when disabled, got %d", w.Code)
	}
}

func TestInjectRemoteUser_UntrustedIP(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "10.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.99:1234" // not trusted
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	// Untrusted → header ignored → no user → 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 from untrusted IP, got %d", w.Code)
	}
}

func TestInjectRemoteUser_TrustedIP_NoHeader(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	// no Remote-User header
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	// No header → no user injected → handler returns 401
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when header absent, got %d", w.Code)
	}
}

func TestInjectRemoteUser_TrustedIP_ValidUser(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"username":"admin"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestInjectRemoteUser_TrustedIP_UnknownUser(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "ghost")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown user, got %d", w.Code)
	}
}

func TestInjectRemoteUser_CustomHeaderName(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "X-Forwarded-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInjectRemoteUser_CIDRMatch(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "172.18.0.0/16"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "172.18.5.10:1234"
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for CIDR-matched IP, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInjectRemoteUser_MisconfiguredTrustedProxies(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: nil,
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing trusted proxies config, got %d", w.Code)
	}
	if body := w.Body.String(); body != `{"error":"Reverse proxy authentication misconfigured"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestInjectRemoteUser_MisconfiguredUserService(t *testing.T) {
	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    nil,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing user service config, got %d", w.Code)
	}
	if body := w.Body.String(); body != `{"error":"Reverse proxy authentication misconfigured"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

// TestInjectRemoteUser_WithRequireAuth verifies the full middleware chain:
// InjectRemoteUser sets the user, then RequireAuth short-circuits JWT validation.
func TestInjectRemoteUser_WithRequireAuth(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	storageDir := t.TempDir()
	sessionStore, err := coreauth.NewSessionStore(storageDir)
	if err != nil {
		t.Fatalf("create session store: %v", err)
	}
	cleanupWithErrorCheck(t, "session store", sessionStore.Close)

	authService := coreauth.NewAuthService(f.userService, sessionStore, "test-secret-key-for-unit-tests-1", 0, 0)
	authCookies := authmw.NewAuthCookies(true, 0, 0)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authmw.InjectRemoteUser(cfg))
	r.Use(authmw.RequireAuth(authService, authCookies, false))
	r.GET("/test", func(c *gin.Context) {
		u := c.MustGet("user").(*coreauth.User)
		c.JSON(http.StatusOK, gin.H{"username": u.Username})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "admin")
	// no JWT cookie — proxy auth should take over
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with proxy auth + RequireAuth chain, got %d: %s", w.Code, w.Body.String())
	}
}
