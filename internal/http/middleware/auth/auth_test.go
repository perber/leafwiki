package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/wiki"
)

func createTestWiki(t *testing.T) *wiki.Wiki {
	w, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "test-secret-key",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return w
}

func TestRequireAuth_WithAuthDisabled_UserExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createTestWiki(t)
	defer w.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Middleware to inject user (simulating InjectPublicEditor)
	router.Use(func(c *gin.Context) {
		c.Set("user", &auth.User{
			ID:       "public-editor",
			Username: "public-editor",
			Role:     auth.RoleEditor,
		})
		c.Next()
	})

	// Apply RequireAuth with authDisabled=true
	router.Use(RequireAuth(w, authCookies, true))

	router.GET("/test", func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}

		user, ok := userValue.(*auth.User)
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

	w := createTestWiki(t)
	defer w.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=true but no user injected
	router.Use(RequireAuth(w, authCookies, true))

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

	wikiInstance := createTestWiki(t)
	defer wikiInstance.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	// Login to get a valid token
	authToken, err := wikiInstance.GetAuthService().Login("admin", "admin")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(RequireAuth(wikiInstance, authCookies, false))

	router.GET("/test", func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}

		u, ok := userValue.(*auth.User)
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

	wikiInstance := createTestWiki(t)
	defer wikiInstance.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(RequireAuth(wikiInstance, authCookies, false))

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

func TestRequireAuth_WithAuthEnabled_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	wikiInstance := createTestWiki(t)
	defer wikiInstance.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(RequireAuth(wikiInstance, authCookies, false))

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

	wikiInstance := createTestWiki(t)
	defer wikiInstance.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	// Login to get a valid token
	authToken, err := wikiInstance.GetAuthService().Login("admin", "admin")
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(RequireAuth(wikiInstance, authCookies, false))

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

	wikiInstance := createTestWiki(t)
	defer wikiInstance.Close()

	authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()

	// Apply RequireAuth with authDisabled=false
	router.Use(RequireAuth(wikiInstance, authCookies, false))

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
			wikiInstance := createTestWiki(t)
			defer wikiInstance.Close()

			authCookies := NewAuthCookies(true, time.Hour, time.Hour*24)

			router := gin.New()

			// Inject user if needed
			if tc.injectUser {
				router.Use(func(c *gin.Context) {
					c.Set("user", &auth.User{
						ID:       "public-editor",
						Username: "public-editor",
						Role:     auth.RoleEditor,
					})
					c.Next()
				})
			}

			// Apply RequireAuth
			router.Use(RequireAuth(wikiInstance, authCookies, tc.authDisabled))

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)

			// Add token if needed
			if tc.provideToken {
				var token string
				if tc.validToken {
					authToken, err := wikiInstance.GetAuthService().Login("admin", "admin")
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
