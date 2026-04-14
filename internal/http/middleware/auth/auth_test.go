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

type authFixture struct {
	auth  *coreauth.AuthService
	close func() error
}

func createTestAuthFixture(t *testing.T) *authFixture {
	t.Helper()

	storageDir := t.TempDir()
	userStore, err := coreauth.NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}
	sessionStore, err := coreauth.NewSessionStore(storageDir)
	if err != nil {
		_ = userStore.Close()
		t.Fatalf("Failed to create session store: %v", err)
	}

	userService := coreauth.NewUserService(userStore)
	if err := userService.InitDefaultAdmin("admin"); err != nil {
		_ = sessionStore.Close()
		_ = userStore.Close()
		t.Fatalf("Failed to init default admin: %v", err)
	}

	return &authFixture{
		auth: coreauth.NewAuthService(userService, sessionStore, "test-secret-key", 15*time.Minute, 7*24*time.Hour),
		close: func() error {
			if err := sessionStore.Close(); err != nil {
				_ = userStore.Close()
				return err
			}
			return userStore.Close()
		},
	}
}

func TestRequireAuth_WithAuthDisabled_UserExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Middleware to inject user (simulating authmw.InjectPublicEditor)
	router.Use(func(c *gin.Context) {
		c.Set("user", &coreauth.User{
			ID:       "public-editor",
			Username: "public-editor",
			Role:     coreauth.RoleEditor,
		})
		c.Next()
	})

	// Apply RequireAuth with authDisabled=true
	router.Use(authmw.RequireAuth(nil, authCookies, true))

	router.GET("/test", func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}

		user, ok := userValue.(*coreauth.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"username": user.Username,
			"role":     user.Role,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d - %s", w2.Code, w2.Body.String())
	}

	expectedBody := `{"role":"editor","username":"public-editor"}`
	if w2.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w2.Body.String())
	}
}

func TestRequireAuth_WithAuthDisabled_NoUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=true but no user injected
	router.Use(authmw.RequireAuth(nil, authCookies, true))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 when authDisabled=true but no user, got %d", w2.Code)
	}

	expectedBody := `{"error":"User not authenticated and auth is disabled"}`
	if w2.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w2.Body.String())
	}
}

func TestRequireAuth_WithAuthEnabled_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := createTestAuthFixture(t)
	defer func() {
		if err := fixture.close(); err != nil {
			t.Fatalf("close auth fixture: %v", err)
		}
	}()

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	// Login to get a valid token
	authToken, err := fixture.auth.Login("admin", "admin")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(authmw.RequireAuth(fixture.auth, authCookies, false))

	router.GET("/test", func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}

		u, ok := userValue.(*coreauth.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"username": u.Username,
			"role":     u.Role,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_at",
		Value: authToken.Token,
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d - %s", w.Code, w.Body.String())
	}

	expectedBody := `{"role":"admin","username":"admin"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestRequireAuth_WithAuthEnabled_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := createTestAuthFixture(t)
	defer func() {
		if err := fixture.close(); err != nil {
			t.Fatalf("close auth fixture: %v", err)
		}
	}()

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(authmw.RequireAuth(fixture.auth, authCookies, false))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 when no token provided, got %d", w.Code)
	}

	expectedBody := `{"error":"Missing or invalid access token"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestRequireAuth_WithAuthEnabled_NilAuthService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.Use(authmw.RequireAuth(nil, authCookies, false))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_at",
		Value: "some-token",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 when auth service is nil, got %d", w.Code)
	}

	expectedBody := `{"error":"Authentication service unavailable"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestRequireAuth_WithAuthEnabled_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := createTestAuthFixture(t)
	defer func() {
		if err := fixture.close(); err != nil {
			t.Fatalf("close auth fixture: %v", err)
		}
	}()

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(authmw.RequireAuth(fixture.auth, authCookies, false))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_at",
		Value: "invalid-token-123",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 when invalid token provided, got %d", w.Code)
	}

	expectedBody := `{"error":"Invalid or expired token"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestRequireAuth_WithAuthEnabled_UserSetInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := createTestAuthFixture(t)
	defer func() {
		if err := fixture.close(); err != nil {
			t.Fatalf("close auth fixture: %v", err)
		}
	}()

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	// Login to get a valid token
	authToken, err := fixture.auth.Login("admin", "admin")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(authmw.RequireAuth(fixture.auth, authCookies, false))

	userSetInContext := false

	router.GET("/test", func(c *gin.Context) {
		_, exists := c.Get("user")
		if exists {
			userSetInContext = true
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_at",
		Value: authToken.Token,
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if !userSetInContext {
		t.Error("Expected user to be set in context")
	}
}

func TestRequireAuth_NextNotCalledOnFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixture := createTestAuthFixture(t)
	defer func() {
		if err := fixture.close(); err != nil {
			t.Fatalf("close auth fixture: %v", err)
		}
	}()

	authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(authmw.RequireAuth(fixture.auth, authCookies, false))

	nextCalled := false

	router.Use(func(c *gin.Context) {
		nextCalled = true
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	if nextCalled {
		t.Error("Expected Next() not to be called when authentication fails")
	}
}

func TestRequireAuth_ComprehensiveScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		authDisabled   bool
		injectUser     bool
		provideToken   bool
		validToken     bool
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "authDisabled=true, user injected - should pass",
			authDisabled:   true,
			injectUser:     true,
			provideToken:   false,
			validToken:     false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "authDisabled=true, no user - should fail",
			authDisabled:   true,
			injectUser:     false,
			provideToken:   false,
			validToken:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "User not authenticated and auth is disabled",
		},
		{
			name:           "authDisabled=false, valid token - should pass",
			authDisabled:   false,
			injectUser:     false,
			provideToken:   true,
			validToken:     true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "authDisabled=false, no token - should fail",
			authDisabled:   false,
			injectUser:     false,
			provideToken:   false,
			validToken:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Missing or invalid access token",
		},
		{
			name:           "authDisabled=false, invalid token - should fail",
			authDisabled:   false,
			injectUser:     false,
			provideToken:   true,
			validToken:     false,
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid or expired token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := createTestAuthFixture(t)
			defer func() {
				if err := fixture.close(); err != nil {
					t.Fatalf("close auth fixture: %v", err)
				}
			}()

			authCookies := authmw.NewAuthCookies(true, time.Hour, time.Hour*24)

			router := gin.New()

			// Inject user if needed
			if tc.injectUser {
				router.Use(func(c *gin.Context) {
					c.Set("user", &coreauth.User{
						ID:       "public-editor",
						Username: "public-editor",
						Role:     coreauth.RoleEditor,
					})
					c.Next()
				})
			}

			// Apply RequireAuth
			router.Use(authmw.RequireAuth(fixture.auth, authCookies, tc.authDisabled))

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)

			// Add token if needed
			if tc.provideToken {
				var token string
				if tc.validToken {
					authToken, err := fixture.auth.Login("admin", "admin")
					if err != nil {
						t.Fatalf("Failed to login: %v", err)
					}
					token = authToken.Token
				} else {
					token = "invalid-token"
				}
				req.AddCookie(&http.Cookie{
					Name:  "leafwiki_at",
					Value: token,
				})
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d - %s", tc.expectedStatus, w.Code, w.Body.String())
			}

			if tc.expectedError != "" {
				expectedBody := `{"error":"` + tc.expectedError + `"}`
				if w.Body.String() != expectedBody {
					t.Errorf("Expected error %s, got %s", expectedBody, w.Body.String())
				}
			}
		})
	}
}
