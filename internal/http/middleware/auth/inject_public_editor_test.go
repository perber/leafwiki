package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

func TestInjectPublicEditor_AuthDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		authDisabled   bool
		existingUser   bool
		expectUser     bool
		expectUsername string
		expectRole     string
	}{
		{
			name:           "auth disabled with no existing user - should inject public editor",
			authDisabled:   true,
			existingUser:   false,
			expectUser:     true,
			expectUsername: "public-editor",
			expectRole:     auth.RoleEditor,
		},
		{
			name:           "auth disabled with existing user - should not override",
			authDisabled:   true,
			existingUser:   true,
			expectUser:     true,
			expectUsername: "existing-user",
			expectRole:     auth.RoleAdmin,
		},
		{
			name:         "auth enabled with no existing user - should not inject",
			authDisabled: false,
			existingUser: false,
			expectUser:   false,
		},
		{
			name:           "auth enabled with existing user - should not change",
			authDisabled:   false,
			existingUser:   true,
			expectUser:     true,
			expectUsername: "existing-user",
			expectRole:     auth.RoleAdmin,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()

			// Set up existing user if needed
			if tc.existingUser {
				router.Use(func(c *gin.Context) {
					c.Set("user", &auth.User{
						ID:       "existing-user-id",
						Username: "existing-user",
						Role:     auth.RoleAdmin,
					})
					c.Next()
				})
			}

			// Add the middleware under test
			router.Use(InjectPublicEditor(tc.authDisabled))

			// Test endpoint
			router.GET("/test", func(c *gin.Context) {
				userValue, exists := c.Get("user")
				if !exists {
					c.JSON(http.StatusOK, gin.H{"user": nil})
					return
				}

				user, ok := userValue.(*auth.User)
				if !ok {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user type"})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"user": gin.H{
						"username": user.Username,
						"role":     user.Role,
					},
				})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Verify response body
			if tc.expectUser {
				expectedBody := `{"user":{"role":"` + tc.expectRole + `","username":"` + tc.expectUsername + `"}}`
				if w.Body.String() != expectedBody {
					t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
				}
			} else {
				expectedBody := `{"user":null}`
				if w.Body.String() != expectedBody {
					t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
				}
			}
		})
	}
}

func TestInjectPublicEditor_PublicEditorProperties(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(InjectPublicEditor(true))

	router.GET("/test", func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "no user found"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user type"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	expectedBody := `{"id":"public-editor","role":"editor","username":"public-editor"}`
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %s, got %s", expectedBody, w.Body.String())
	}
}

func TestInjectPublicEditor_NextCalled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	nextCalled := false

	router := gin.New()
	router.Use(InjectPublicEditor(true))
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

	if !nextCalled {
		t.Error("Expected Next() to be called")
	}
}
