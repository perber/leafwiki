package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

type proxyFixture struct {
	userService func() *coreauth.UserService
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
	if err := userService.InitDefaultAdmin("", "", "adminpassword"); err != nil {
		_ = userStore.Close()
		t.Fatalf("init default admin: %v", err)
	}

	return &proxyFixture{
		userService: func() *coreauth.UserService { return userService },
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

func TestInjectRemoteUser_TrustedIP_ValidUser_ByEmail(t *testing.T) {
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
	// default admin's email (see InitDefaultAdmin), not its username
	req.Header.Set("Remote-User", "admin@localhost")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when header carries the user's email, got %d: %s", w.Code, w.Body.String())
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

// TestInjectRemoteUser_ReflectsLiveRestore is a regression test for the
// remote-user half of "User-Management Routes Go Stale After Live Restore":
// RemoteUserConfig.UserService used to be a *coreauth.UserService captured
// once when the router was built, so it never saw a live restore's
// AuthService.ReplaceUserStore swap. A resolver func must be called fresh on
// every request instead.
func TestInjectRemoteUser_ReflectsLiveRestore(t *testing.T) {
	preDir := t.TempDir()
	preStore, err := coreauth.NewUserStore(preDir)
	if err != nil {
		t.Fatalf("NewUserStore(pre): %v", err)
	}
	preSvc := coreauth.NewUserService(preStore)

	sessionStore, err := coreauth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authSvc := coreauth.NewAuthService(preSvc, sessions, nil)
	t.Cleanup(func() { _ = authSvc.Close() })

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    authSvc.UserService,
	}

	postDir := t.TempDir()
	postStore, err := coreauth.NewUserStore(postDir)
	if err != nil {
		t.Fatalf("NewUserStore(post): %v", err)
	}
	postSvc := coreauth.NewUserService(postStore)
	if _, err := postSvc.CreateUser("post-restore-admin", "post-restore-admin@example.com", "password123", coreauth.RoleAdmin); err != nil {
		t.Fatalf("CreateUser(post): %v", err)
	}
	if err := postStore.Close(); err != nil {
		t.Fatalf("Close(postStore): %v", err)
	}

	// Simulates what a live restore does to AuthService.
	if err := authSvc.ReplaceUserStore(postDir); err != nil {
		t.Fatalf("ReplaceUserStore: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "post-restore-admin")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 resolving a post-restore-only user, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInjectRemoteUser_AutoCreateDisabled_UnknownUserStill401s(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		AutoCreate:     false,
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "ghost")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown user with auto-create disabled, got %d", w.Code)
	}
}

func TestInjectRemoteUser_AutoCreateEnabled_ProvisionsUnknownUser(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		AutoCreate:     true,
		DefaultRole:    coreauth.RoleViewer,
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "newperson")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with auto-create enabled, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"username":"newperson"}` {
		t.Errorf("unexpected body: %s", body)
	}

	user, err := f.userService().GetUserByIdentifier("newperson")
	if err != nil {
		t.Fatalf("expected auto-created user to be persisted: %v", err)
	}
	if user.Role != coreauth.RoleViewer {
		t.Errorf("expected auto-created user to have role %q, got %q", coreauth.RoleViewer, user.Role)
	}
	if user.Email != "newperson@remote-user.invalid" {
		t.Errorf("expected synthesized placeholder email, got %q", user.Email)
	}
}

func TestInjectRemoteUser_AutoCreateEnabled_UsesEmailHeader(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:         true,
		HeaderName:      "Remote-User",
		AutoCreate:      true,
		EmailHeaderName: "Remote-Email",
		DefaultRole:     coreauth.RoleViewer,
		TrustedProxies:  mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:     f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "newperson")
	req.Header.Set("Remote-Email", "newperson@example.com")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	user, err := f.userService().GetUserByIdentifier("newperson")
	if err != nil {
		t.Fatalf("expected auto-created user to be persisted: %v", err)
	}
	if user.Email != "newperson@example.com" {
		t.Errorf("expected email from Remote-Email header, got %q", user.Email)
	}
}

func TestInjectRemoteUser_AutoCreateEnabled_EmailConflictWithDifferentUserReturns401(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:         true,
		HeaderName:      "Remote-User",
		AutoCreate:      true,
		EmailHeaderName: "Remote-Email",
		DefaultRole:     coreauth.RoleViewer,
		TrustedProxies:  mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:     f.userService,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("Remote-User", "newperson")
	// admin@localhost is the fixture's pre-existing default admin's email —
	// not a race on "newperson", but a genuine conflict with a different user.
	req.Header.Set("Remote-Email", "admin@localhost")
	w := httptest.NewRecorder()

	proxyRouter(cfg).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for email conflict with a different user, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := f.userService().GetUserByIdentifier("newperson"); err == nil {
		t.Error("expected 'newperson' to not have been created as a side effect of the conflict")
	}
}

func TestInjectRemoteUser_AutoCreateEnabled_SecondRequestReusesSameUser(t *testing.T) {
	f := createProxyFixture(t)
	cleanupWithErrorCheck(t, "proxy fixture", f.close)

	cfg := authmw.RemoteUserConfig{
		Enabled:        true,
		HeaderName:     "Remote-User",
		AutoCreate:     true,
		DefaultRole:    coreauth.RoleViewer,
		TrustedProxies: mustParseTrustedProxies(t, "127.0.0.1"),
		UserService:    f.userService,
	}

	router := proxyRouter(cfg)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("Remote-User", "newperson")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	users, err := f.userService().GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	count := 0
	for _, u := range users {
		if u.Username == "newperson" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'newperson' user after two requests, got %d", count)
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

	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", 0, 0)
	authService := coreauth.NewAuthService(f.userService(), sessions, nil)
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
