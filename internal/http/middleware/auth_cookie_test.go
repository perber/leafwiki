package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthCookies_RequireSecure_TLS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthCookies_RequireSecure_XForwardedProto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{"https lowercase", "https", true},
		{"HTTPS uppercase", "HTTPS", true},
		{"https with scheme", "https://example.com", true},
		{"http", "http", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-Proto", tc.value)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tc.expected {
				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			} else {
				if w.Code != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", w.Code)
				}
			}
		})
	}
}

func TestAuthCookies_RequireSecure_XForwardedSsl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{"on lowercase", "on", true},
		{"ON uppercase", "ON", true},
		{"On mixed", "On", true},
		{"off", "off", false},
		{"empty", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Forwarded-Ssl", tc.value)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tc.expected {
				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			} else {
				if w.Code != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", w.Code)
				}
			}
		})
	}
}

func TestAuthCookies_RequireSecure_FrontEndHttps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{"on lowercase", "on", true},
		{"ON uppercase", "ON", true},
		{"On mixed", "On", true},
		{"off", "off", false},
		{"empty", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Front-End-Https", tc.value)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if tc.expected {
				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}
			} else {
				if w.Code != http.StatusBadRequest {
					t.Errorf("Expected status 400, got %d", w.Code)
				}
			}
		})
	}
}

func TestAuthCookies_RequireSecure_AllowInsecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	// Make a request without any HTTPS indicators
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed when AllowInsecure is true
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 with AllowInsecure=true, got %d", w.Code)
	}
}

func TestAuthCookies_RequireSecure_ErrorWhenHTTPSRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		secure, err := auth.requireSecure(c)
		if err != nil {
			if err == ErrHTTPSRequired {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"secure": secure})
	})

	// Make a request without any HTTPS indicators
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should fail when AllowInsecure is false and no HTTPS indicators
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when HTTPS required but not present, got %d", w.Code)
	}
}

func TestAuthCookies_CookieNames_Secure(t *testing.T) {
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	accessName, refreshName := auth.cookieNames(true)

	if accessName != "__Host-leafwiki_at" {
		t.Errorf("Expected access cookie name '__Host-leafwiki_at', got '%s'", accessName)
	}

	if refreshName != "__Host-leafwiki_rt" {
		t.Errorf("Expected refresh cookie name '__Host-leafwiki_rt', got '%s'", refreshName)
	}
}

func TestAuthCookies_CookieNames_Insecure(t *testing.T) {
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	accessName, refreshName := auth.cookieNames(false)

	if accessName != "leafwiki_at" {
		t.Errorf("Expected access cookie name 'leafwiki_at', got '%s'", accessName)
	}

	if refreshName != "leafwiki_rt" {
		t.Errorf("Expected refresh cookie name 'leafwiki_rt', got '%s'", refreshName)
	}
}

func TestAuthCookies_Set_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Set(c, "access-token-123", "refresh-token-456")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that cookies were set correctly
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("Expected 2 cookies, got %d", len(cookies))
	}

	// Find access token cookie
	var accessCookie, refreshCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "__Host-leafwiki_at" {
			accessCookie = cookie
		} else if cookie.Name == "__Host-leafwiki_rt" {
			refreshCookie = cookie
		}
	}

	if accessCookie == nil {
		t.Fatal("Access token cookie not found")
	}
	if refreshCookie == nil {
		t.Fatal("Refresh token cookie not found")
	}

	// Verify access cookie properties
	if accessCookie.Value != "access-token-123" {
		t.Errorf("Expected access token value 'access-token-123', got '%s'", accessCookie.Value)
	}
	if !accessCookie.HttpOnly {
		t.Error("Expected access cookie to be HttpOnly")
	}
	if !accessCookie.Secure {
		t.Error("Expected access cookie to be Secure")
	}
	if accessCookie.Path != "/" {
		t.Errorf("Expected access cookie path '/', got '%s'", accessCookie.Path)
	}
	if accessCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected access cookie SameSite StrictMode, got %v", accessCookie.SameSite)
	}
	if accessCookie.MaxAge != 3600 {
		t.Errorf("Expected access cookie MaxAge 3600, got %d", accessCookie.MaxAge)
	}

	// Verify refresh cookie properties
	if refreshCookie.Value != "refresh-token-456" {
		t.Errorf("Expected refresh token value 'refresh-token-456', got '%s'", refreshCookie.Value)
	}
	if !refreshCookie.HttpOnly {
		t.Error("Expected refresh cookie to be HttpOnly")
	}
	if !refreshCookie.Secure {
		t.Error("Expected refresh cookie to be Secure")
	}
	if refreshCookie.Path != "/api/auth/refresh-token" {
		t.Errorf("Expected refresh cookie path '/api/auth/refresh-token', got '%s'", refreshCookie.Path)
	}
	if refreshCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected refresh cookie SameSite StrictMode, got %v", refreshCookie.SameSite)
	}
	if refreshCookie.MaxAge != 86400 {
		t.Errorf("Expected refresh cookie MaxAge 86400, got %d", refreshCookie.MaxAge)
	}
}

func TestAuthCookies_Set_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Set(c, "access-token-789", "refresh-token-012")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that cookies were set correctly
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("Expected 2 cookies, got %d", len(cookies))
	}

	// Find access token cookie
	var accessCookie, refreshCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "leafwiki_at" {
			accessCookie = cookie
		} else if cookie.Name == "leafwiki_rt" {
			refreshCookie = cookie
		}
	}

	if accessCookie == nil {
		t.Fatal("Access token cookie not found")
	}
	if refreshCookie == nil {
		t.Fatal("Refresh token cookie not found")
	}

	// Verify access cookie doesn't have __Host- prefix
	if accessCookie.Name != "leafwiki_at" {
		t.Errorf("Expected cookie name 'leafwiki_at', got '%s'", accessCookie.Name)
	}
	if accessCookie.Secure {
		t.Error("Expected access cookie to NOT be Secure in insecure mode")
	}

	// Verify refresh cookie doesn't have __Host- prefix
	if refreshCookie.Name != "leafwiki_rt" {
		t.Errorf("Expected cookie name 'leafwiki_rt', got '%s'", refreshCookie.Name)
	}
	if refreshCookie.Secure {
		t.Error("Expected refresh cookie to NOT be Secure in insecure mode")
	}
}

func TestAuthCookies_Set_ErrorWhenHTTPSRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Set(c, "access-token", "refresh-token")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when HTTPS required, got %d", w.Code)
	}
}

func TestAuthCookies_Clear_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Clear(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that cookies were cleared
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("Expected 2 cookies, got %d", len(cookies))
	}

	// Find cookies
	var accessCookie, refreshCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == "__Host-leafwiki_at" {
			accessCookie = cookie
		} else if cookie.Name == "__Host-leafwiki_rt" {
			refreshCookie = cookie
		}
	}

	if accessCookie == nil {
		t.Fatal("Access token cookie not found")
	}
	if refreshCookie == nil {
		t.Fatal("Refresh token cookie not found")
	}

	// Verify cookies are expired
	if accessCookie.Value != "" {
		t.Errorf("Expected access cookie to have empty value, got '%s'", accessCookie.Value)
	}
	if accessCookie.MaxAge != -1 {
		t.Errorf("Expected access cookie MaxAge -1, got %d", accessCookie.MaxAge)
	}

	if refreshCookie.Value != "" {
		t.Errorf("Expected refresh cookie to have empty value, got '%s'", refreshCookie.Value)
	}
	if refreshCookie.MaxAge != -1 {
		t.Errorf("Expected refresh cookie MaxAge -1, got %d", refreshCookie.MaxAge)
	}
}

func TestAuthCookies_Clear_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Clear(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that cookies were cleared with correct names
	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("Expected 2 cookies, got %d", len(cookies))
	}

	// Verify cookie names don't have __Host- prefix
	for _, cookie := range cookies {
		if cookie.Name != "leafwiki_at" && cookie.Name != "leafwiki_rt" {
			t.Errorf("Unexpected cookie name: %s", cookie.Name)
		}
		if cookie.MaxAge != -1 {
			t.Errorf("Expected cookie MaxAge -1, got %d", cookie.MaxAge)
		}
	}
}

func TestAuthCookies_ReadAccess_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadAccess(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_at",
		Value: "test-access-token",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthCookies_ReadAccess_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadAccess(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_at",
		Value: "test-access-token-insecure",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthCookies_ReadAccess_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadAccess(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when cookie is missing, got %d", w.Code)
	}
}

func TestAuthCookies_ReadRefresh_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadRefresh(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_rt",
		Value: "test-refresh-token",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthCookies_ReadRefresh_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(true, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadRefresh(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_rt",
		Value: "test-refresh-token-insecure",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthCookies_ReadRefresh_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auth := NewAuthCookies(false, time.Hour, time.Hour*24)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := auth.ReadRefresh(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when cookie is missing, got %d", w.Code)
	}
}

func TestAuthCookies_CustomTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	accessTTL := 30 * time.Minute
	refreshTTL := 7 * 24 * time.Hour
	auth := NewAuthCookies(false, accessTTL, refreshTTL)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := auth.Set(c, "access-token", "refresh-token")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == "__Host-leafwiki_at" {
			expectedMaxAge := int(accessTTL.Seconds())
			if cookie.MaxAge != expectedMaxAge {
				t.Errorf("Expected access cookie MaxAge %d, got %d", expectedMaxAge, cookie.MaxAge)
			}
		} else if cookie.Name == "__Host-leafwiki_rt" {
			expectedMaxAge := int(refreshTTL.Seconds())
			if cookie.MaxAge != expectedMaxAge {
				t.Errorf("Expected refresh cookie MaxAge %d, got %d", expectedMaxAge, cookie.MaxAge)
			}
		}
	}
}
