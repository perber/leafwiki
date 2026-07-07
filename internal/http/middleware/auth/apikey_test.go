package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

type apiKeyFixture struct {
	userService *coreauth.UserService
	keyService  *coreauth.APIKeyService
	owner       *coreauth.User
	closeAll    func() error
}

func createAPIKeyFixture(t *testing.T) *apiKeyFixture {
	t.Helper()

	userStore, err := coreauth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("create user store: %v", err)
	}
	userService := coreauth.NewUserService(userStore)

	owner, err := userService.CreateUser("agent-owner", "agent-owner@example.com", "password123", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("create owner user: %v", err)
	}

	keyStore, err := coreauth.NewAPIKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("create api key store: %v", err)
	}
	keyService := coreauth.NewAPIKeyService(keyStore, userService)

	return &apiKeyFixture{
		userService: userService,
		keyService:  keyService,
		owner:       owner,
		closeAll: func() error {
			_ = keyStore.Close()
			return userStore.Close()
		},
	}
}

func apiKeyRouter(cfg authmw.APIKeyConfig, presetUser *coreauth.User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if presetUser != nil {
		r.Use(func(c *gin.Context) {
			c.Set("user", presetUser)
			c.Next()
		})
	}
	r.Use(authmw.InjectAPIKeyUser(cfg))
	r.GET("/test", func(c *gin.Context) {
		userVal, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no user"})
			return
		}
		u := userVal.(*coreauth.User)
		c.JSON(http.StatusOK, gin.H{"username": u.Username, "role": u.Role})
	})
	return r
}

func TestInjectAPIKeyUser_ServiceNotConfigured(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer lw_deadbeef_"+"a")
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: nil}, nil).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (no-op, no user set) when service is nil, got %d", w.Code)
	}
}

func TestInjectAPIKeyUser_NoAuthorizationHeader(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, nil).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no Authorization header, got %d", w.Code)
	}
}

func TestInjectAPIKeyUser_NonLeafWikiBearerPassesThrough(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-other-service-token")
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, nil).ServeHTTP(w, req)

	// Not shaped like a LeafWiki key → middleware no-ops, downstream sees no user.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 (pass-through, no user) for non-LeafWiki bearer, got %d", w.Code)
	}
}

func TestInjectAPIKeyUser_DoesNotOverrideAlreadySetUser(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	_, token, err := f.keyService.CreateAPIKey(coreauth.CreateAPIKeyParams{
		Name: "k", UserID: f.owner.ID, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	preset := &coreauth.User{ID: "preset-id", Username: "preset-user", Role: coreauth.RoleAdmin}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, preset).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"role":"admin","username":"preset-user"}` {
		t.Errorf("expected preset user to remain, got body: %s", body)
	}
}

func TestInjectAPIKeyUser_ValidKeySetsUser(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	_, token, err := f.keyService.CreateAPIKey(coreauth.CreateAPIKeyParams{
		Name: "k", UserID: f.owner.ID, Role: coreauth.RoleViewer, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, nil).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"role":"viewer","username":"agent-owner"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestInjectAPIKeyUser_InvalidKeyRejected(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer lw_deadbeef_0000000000000000000000000000000000000000000000000000000000000000")
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, nil).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid key, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInjectAPIKeyUser_RevokedKeyRejected(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	key, token, err := f.keyService.CreateAPIKey(coreauth.CreateAPIKeyParams{
		Name: "k", UserID: f.owner.ID, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if err := f.keyService.RevokeAPIKey(key.ID); err != nil {
		t.Fatalf("RevokeAPIKey err: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	apiKeyRouter(authmw.APIKeyConfig{Service: f.keyService}, nil).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestInjectAPIKeyUser_WithRequireAuth verifies the full middleware chain:
// InjectAPIKeyUser sets the user, then RequireAuth short-circuits JWT validation
// — the same contract TestInjectRemoteUser_WithRequireAuth verifies for proxy auth.
func TestInjectAPIKeyUser_WithRequireAuth(t *testing.T) {
	f := createAPIKeyFixture(t)
	cleanupWithErrorCheck(t, "api key fixture", f.closeAll)

	sessionStore, err := coreauth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("create session store: %v", err)
	}
	cleanupWithErrorCheck(t, "session store", sessionStore.Close)

	authService := coreauth.NewAuthService(f.userService, sessionStore, "test-secret-key-for-unit-tests-1", 0, 0)
	authCookies := authmw.NewAuthCookies(true, 0, 0)

	_, token, err := f.keyService.CreateAPIKey(coreauth.CreateAPIKeyParams{
		Name: "k", UserID: f.owner.ID, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authmw.InjectAPIKeyUser(authmw.APIKeyConfig{Service: f.keyService}))
	r.Use(authmw.RequireAuth(authService, authCookies, false))
	r.GET("/test", func(c *gin.Context) {
		u := c.MustGet("user").(*coreauth.User)
		c.JSON(http.StatusOK, gin.H{"username": u.Username})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// no JWT cookie — api key auth should take over
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with api key auth + RequireAuth chain, got %d: %s", w.Code, w.Body.String())
	}
}
