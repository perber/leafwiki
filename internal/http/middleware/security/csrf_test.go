package security

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestCSRFMiddleware_AllowsSafeMethodsWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrf := NewCSRFCookie(false, time.Hour, "/")

	router := gin.New()
	router.Use(CSRFMiddleware(csrf))
	router.GET("/safe", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/safe", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for GET without CSRF, got %d", w.Code)
	}
}

func TestCSRFMiddleware_BlocksPostWithoutCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrf := NewCSRFCookie(false, time.Hour, "/")

	router := gin.New()
	router.Use(CSRFMiddleware(csrf))
	router.POST("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/protected", nil)
	req.TLS = &tls.ConnectionState{} // ensure requireSecure in Read treats the request as "secure"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for POST without CSRF cookie, got %d", w.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "CSRF token missing" {
		t.Fatalf("expected error 'CSRF token missing', got '%s'", body["error"])
	}
}

func TestCSRFMiddleware_BlocksPostWithCookieButNoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrf := NewCSRFCookie(false, time.Hour, "/")

	router := gin.New()
	router.Use(CSRFMiddleware(csrf))
	router.POST("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/protected", nil)
	req.TLS = &tls.ConnectionState{}
	// Cookie vorhanden, aber kein Header/Form-Token
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_csrf",
		Value: "test-token",
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for POST with cookie but no header, got %d", w.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["error"] != "Invalid CSRF token" {
		t.Fatalf("expected error 'Invalid CSRF token', got '%s'", body["error"])
	}
}

func TestCSRFMiddleware_BlocksPostWithMismatchingTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrf := NewCSRFCookie(false, time.Hour, "/")

	router := gin.New()
	router.Use(CSRFMiddleware(csrf))
	router.POST("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	body := strings.NewReader("")
	req := httptest.NewRequest("POST", "/protected", body)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_csrf",
		Value: "cookie-token",
	})
	req.Header.Set("X-CSRF-Token", "different-header-token")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for POST with mismatching tokens, got %d", w.Code)
	}

	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "Invalid CSRF token" {
		t.Fatalf("expected error 'Invalid CSRF token', got '%s'", resp["error"])
	}
}

func TestCSRFMiddleware_AllowsPostWithMatchingTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	csrf := NewCSRFCookie(false, time.Hour, "/")

	router := gin.New()
	router.Use(CSRFMiddleware(csrf))
	router.POST("/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	const token = "same-token"
	body := strings.NewReader("")
	req := httptest.NewRequest("POST", "/protected", body)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_csrf",
		Value: token,
	})
	req.Header.Set("X-CSRF-Token", token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for POST with valid CSRF, got %d", w.Code)
	}
}
