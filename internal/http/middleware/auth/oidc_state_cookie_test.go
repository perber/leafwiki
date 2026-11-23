package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// assertCookiePrefixRules fails t if cookie's name uses a "__Host-" or
// "__Secure-" prefix without satisfying what browsers require for it (and
// silently drop the cookie otherwise, per RFC 6265bis) — this is exactly the
// class of bug this test guards against: Go's net/http and httptest's
// cookiejar don't enforce these rules, so a violation only breaks in a real
// browser, never in a Go test that doesn't check for it explicitly.
func assertCookiePrefixRules(t *testing.T, cookie *http.Cookie) {
	t.Helper()
	switch {
	case strings.HasPrefix(cookie.Name, "__Host-"):
		if cookie.Path != "/" {
			t.Errorf("cookie %q uses the __Host- prefix but Path is %q, not \"/\" — browsers reject such a cookie silently", cookie.Name, cookie.Path)
		}
		if cookie.Domain != "" {
			t.Errorf("cookie %q uses the __Host- prefix but sets Domain=%q — browsers reject such a cookie silently", cookie.Name, cookie.Domain)
		}
		if !cookie.Secure {
			t.Errorf("cookie %q uses the __Host- prefix but is not Secure — browsers reject such a cookie silently", cookie.Name)
		}
	case strings.HasPrefix(cookie.Name, "__Secure-"):
		if !cookie.Secure {
			t.Errorf("cookie %q uses the __Secure- prefix but is not Secure — browsers reject such a cookie silently", cookie.Name)
		}
	}
}

func TestOIDCStateCookie_CookieName(t *testing.T) {
	c := NewOIDCStateCookie(false, time.Minute)

	if got := c.cookieName(true); got != "__Secure-leafwiki_oidc_state" {
		t.Errorf("cookieName(true) = %q, want %q", got, "__Secure-leafwiki_oidc_state")
	}
	if got := c.cookieName(false); got != "leafwiki_oidc_state" {
		t.Errorf("cookieName(false) = %q, want %q", got, "leafwiki_oidc_state")
	}
}

func TestOIDCStateCookie_Issue_Secure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := NewOIDCStateCookie(false, 10*time.Minute)

	router := gin.New()
	router.GET("/login/oidc/login", func(ctx *gin.Context) {
		if err := c.Issue(ctx, "the-state", "the-nonce", "the-verifier"); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/login/oidc/login", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]

	assertCookiePrefixRules(t, cookie)

	if cookie.Name != "__Secure-leafwiki_oidc_state" {
		t.Errorf("cookie name = %q, want %q", cookie.Name, "__Secure-leafwiki_oidc_state")
	}
	if !cookie.HttpOnly {
		t.Error("expected the cookie to be HttpOnly")
	}
	if !cookie.Secure {
		t.Error("expected the cookie to be Secure in secure mode")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", cookie.SameSite)
	}
	if cookie.Path != "/login/oidc" {
		t.Errorf("expected Path=/login/oidc, got %q", cookie.Path)
	}
	if cookie.MaxAge != int((10 * time.Minute).Seconds()) {
		t.Errorf("expected MaxAge %d, got %d", int((10 * time.Minute).Seconds()), cookie.MaxAge)
	}
}

func TestOIDCStateCookie_Issue_Insecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := NewOIDCStateCookie(true, 10*time.Minute)

	router := gin.New()
	router.GET("/login/oidc/login", func(ctx *gin.Context) {
		if err := c.Issue(ctx, "the-state", "the-nonce", "the-verifier"); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/login/oidc/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	assertCookiePrefixRules(t, cookie)

	if cookie.Name != "leafwiki_oidc_state" {
		t.Errorf("cookie name = %q, want %q", cookie.Name, "leafwiki_oidc_state")
	}
	if cookie.Secure {
		t.Error("expected the cookie to NOT be Secure in insecure mode")
	}
}

// TestOIDCStateCookie_RoundTrip exercises Issue then ReadAndClear through a
// real gin router and cookie jar, the same path a browser actually takes —
// this is the scenario the __Host-/Path mismatch broke in production while
// every existing Go test using http.Cookie/httptest.NewRequest(...AddCookie)
// still passed.
func TestOIDCStateCookie_RoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := NewOIDCStateCookie(false, 10*time.Minute)

	router := gin.New()
	router.GET("/login/oidc/login", func(ctx *gin.Context) {
		if err := c.Issue(ctx, "the-state", "the-nonce", "the-verifier"); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.Status(http.StatusOK)
	})
	router.GET("/login/oidc/callback", func(ctx *gin.Context) {
		state, nonce, verifier, err := c.ReadAndClear(ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"state": state, "nonce": nonce, "verifier": verifier})
	})

	loginReq := httptest.NewRequest(http.MethodGet, "/login/oidc/login", nil)
	loginReq.TLS = &tls.ConnectionState{}
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie from Issue, got %d", len(cookies))
	}
	issued := cookies[0]

	// A real browser only resends a cookie whose Path is a prefix of the
	// request's path (among other matching rules) — simulate that instead
	// of just trusting that ReadAndClear would receive it.
	const callbackPath = "/login/oidc/callback"
	if !strings.HasPrefix(callbackPath, issued.Path) {
		t.Fatalf("cookie Path %q would not match a browser request to %s", issued.Path, callbackPath)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, callbackPath, nil)
	callbackReq.TLS = &tls.ConnectionState{}
	callbackReq.AddCookie(issued)
	callbackRec := httptest.NewRecorder()
	router.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d - %s", callbackRec.Code, callbackRec.Body.String())
	}
	if !strings.Contains(callbackRec.Body.String(), "the-state") ||
		!strings.Contains(callbackRec.Body.String(), "the-nonce") ||
		!strings.Contains(callbackRec.Body.String(), "the-verifier") {
		t.Fatalf("unexpected callback response: %s", callbackRec.Body.String())
	}

	clearCookies := callbackRec.Result().Cookies()
	if len(clearCookies) != 1 {
		t.Fatalf("expected 1 cookie clearing the state cookie, got %d", len(clearCookies))
	}
	cleared := clearCookies[0]
	assertCookiePrefixRules(t, cleared)
	if cleared.Name != issued.Name {
		t.Errorf("Clear used name %q, Issue used %q — a mismatch here means the browser wouldn't actually clear the cookie it stored", cleared.Name, issued.Name)
	}
	if cleared.Path != issued.Path {
		t.Errorf("Clear used Path %q, Issue used %q — a mismatch here means the browser wouldn't actually clear the cookie it stored", cleared.Path, issued.Path)
	}
	if cleared.MaxAge != -1 {
		t.Errorf("expected the clearing cookie to have MaxAge -1, got %d", cleared.MaxAge)
	}
}

func TestOIDCStateCookie_ReadAndClear_MissingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := NewOIDCStateCookie(false, 10*time.Minute)

	router := gin.New()
	router.GET("/login/oidc/callback", func(ctx *gin.Context) {
		_, _, _, err := c.ReadAndClear(ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/login/oidc/callback", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when the state cookie is missing, got %d", w.Code)
	}
}
