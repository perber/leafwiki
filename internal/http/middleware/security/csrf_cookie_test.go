package security

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

func TestCSRFCookie_CookieName_Secure(t *testing.T) {
	csrf := NewCSRFCookie(false, time.Hour)

	name := csrf.cookieName(true)
	if name != "__Host-leafwiki_csrf" {
		t.Errorf("Expected secure CSRF cookie name '__Host-leafwiki_csrf', got '%s'", name)
	}
}

func TestCSRFCookie_CookieName_Insecure(t *testing.T) {
	csrf := NewCSRFCookie(true, time.Hour)

	name := csrf.cookieName(false)
	if name != "leafwiki_csrf" {
		t.Errorf("Expected insecure CSRF cookie name 'leafwiki_csrf', got '%s'", name)
	}
}

func TestCSRFCookie_Issue_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(false, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		_, err := csrf.Issue(c)
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
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	// Check that CSRF cookie was set correctly
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	csrfCookie := cookies[0]
	if csrfCookie.Name != "__Host-leafwiki_csrf" {
		t.Errorf("Expected CSRF cookie name '__Host-leafwiki_csrf', got '%s'", csrfCookie.Name)
	}
	if csrfCookie.Value == "" {
		t.Error("Expected CSRF cookie to have a non-empty value")
	}
	if csrfCookie.HttpOnly {
		t.Error("Expected CSRF cookie to NOT be HttpOnly")
	}
	if !csrfCookie.Secure {
		t.Error("Expected CSRF cookie to be Secure in secure mode")
	}
	if csrfCookie.Path != "/" {
		t.Errorf("Expected CSRF cookie path '/', got '%s'", csrfCookie.Path)
	}
	if csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("Expected CSRF cookie SameSite StrictMode, got %v", csrfCookie.SameSite)
	}
	if csrfCookie.MaxAge != int(time.Hour.Seconds()) {
		t.Errorf("Expected CSRF cookie MaxAge %d, got %d", int(time.Hour.Seconds()), csrfCookie.MaxAge)
	}

	// Header should also contain the same CSRF token
	headerToken := w.Result().Header.Get("X-CSRF-Token")
	if headerToken == "" {
		t.Error("Expected X-CSRF-Token header to be set")
	}
	if headerToken != csrfCookie.Value {
		t.Errorf("Expected header token '%s' to match cookie value '%s'", headerToken, csrfCookie.Value)
	}
}

func TestCSRFCookie_Issue_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(true, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		_, err := csrf.Issue(c)
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
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	csrfCookie := cookies[0]
	if csrfCookie.Name != "leafwiki_csrf" {
		t.Errorf("Expected CSRF cookie name 'leafwiki_csrf', got '%s'", csrfCookie.Name)
	}
	if csrfCookie.Secure {
		t.Error("Expected CSRF cookie to NOT be Secure in insecure mode")
	}
	if csrfCookie.HttpOnly {
		t.Error("Expected CSRF cookie to NOT be HttpOnly")
	}
}

func TestCSRFCookie_Issue_ErrorWhenHTTPSRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(false, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		_, err := csrf.Issue(c)
		if err != nil {
			if err == utils.ErrHTTPSRequired {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusOK)
	})

	// No TLS / HTTPS indicators, AllowInsecure=false
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when HTTPS required for CSRF cookie, got %d", w.Code)
	}
}

func TestCSRFCookie_Read_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(false, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := csrf.Read(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  "__Host-leafwiki_csrf",
		Value: "test-csrf-token",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestCSRFCookie_Read_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(true, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := csrf.Read(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "leafwiki_csrf",
		Value: "test-csrf-token-insecure",
	})
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
}

func TestCSRFCookie_Read_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(false, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		token, err := csrf.Read(c)
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
		t.Errorf("Expected status 400 when CSRF cookie is missing, got %d", w.Code)
	}
}

func TestCSRFCookie_Clear_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(false, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := csrf.Clear(c)
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
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	csrfCookie := cookies[0]
	if csrfCookie.Name != "__Host-leafwiki_csrf" {
		t.Errorf("Expected CSRF cookie name '__Host-leafwiki_csrf', got '%s'", csrfCookie.Name)
	}
	if csrfCookie.Value != "" {
		t.Errorf("Expected CSRF cookie value to be empty, got '%s'", csrfCookie.Value)
	}
	if csrfCookie.MaxAge != -1 {
		t.Errorf("Expected CSRF cookie MaxAge -1, got %d", csrfCookie.MaxAge)
	}
}

func TestCSRFCookie_Clear_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	csrf := NewCSRFCookie(true, time.Hour)

	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		err := csrf.Clear(c)
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
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Expected 1 cookie, got %d", len(cookies))
	}

	csrfCookie := cookies[0]
	if csrfCookie.Name != "leafwiki_csrf" {
		t.Errorf("Expected CSRF cookie name 'leafwiki_csrf', got '%s'", csrfCookie.Name)
	}
	if csrfCookie.MaxAge != -1 {
		t.Errorf("Expected CSRF cookie MaxAge -1, got %d", csrfCookie.MaxAge)
	}
}
