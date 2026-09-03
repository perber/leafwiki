package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/publicaccess"
)

// flipGate is a publicaccess.ReadGate whose value can be toggled between
// requests, standing in for the runtime-mutable publicaccess.Service.
type flipGate struct{ enabled bool }

func (g *flipGate) Enabled() bool { return g.enabled }

func newAuthOrPublicRouter(auth *coreauth.AuthService, authCookies *authmw.AuthCookies, authDisabled bool, gate publicaccess.ReadGate, inject gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if inject != nil {
		r.Use(inject)
	}
	r.Use(authmw.RequireAuthOrPublicRead(auth, authCookies, authDisabled, gate))
	r.GET("/test", func(c *gin.Context) {
		if v, ok := c.Get("user"); ok {
			c.JSON(http.StatusOK, gin.H{"user": v.(*coreauth.User).Username})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user": nil})
	})
	return r
}

func doGet(r http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRequireAuthOrPublicRead_UserAlreadyInContext_Passes(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	inject := func(c *gin.Context) {
		c.Set("user", &coreauth.User{ID: "x", Username: "proxied", Role: coreauth.RoleEditor})
		c.Next()
	}
	r := newAuthOrPublicRouter(nil, authCookies, false, &flipGate{enabled: false}, inject)

	w := doGet(r, nil)
	if w.Code != http.StatusOK || w.Body.String() != `{"user":"proxied"}` {
		t.Fatalf("expected 200 with proxied user, got %d %s", w.Code, w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_NoToken_PublicOff_Returns401(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(nil, authCookies, false, &flipGate{enabled: false}, nil)

	w := doGet(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"Missing or invalid access token"}` {
		t.Fatalf("expected RequireAuth's no-token body, got %s", w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_NoToken_PublicOn_PassesAnonymously(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(nil, authCookies, false, &flipGate{enabled: true}, nil)

	w := doGet(r, nil)
	if w.Code != http.StatusOK || w.Body.String() != `{"user":null}` {
		t.Fatalf("expected 200 anonymous, got %d %s", w.Code, w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_InvalidToken_PublicOff_Returns401(t *testing.T) {
	fixture := createTestAuthFixture(t)
	cleanupWithErrorCheck(t, "auth fixture", fixture.close)
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(fixture.auth, authCookies, false, &flipGate{enabled: false}, nil)

	w := doGet(r, &http.Cookie{Name: "leafwiki_at", Value: "bogus"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"Invalid or expired token"}` {
		t.Fatalf("expected RequireAuth's invalid-token body, got %s", w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_InvalidToken_PublicOn_PassesAnonymously(t *testing.T) {
	fixture := createTestAuthFixture(t)
	cleanupWithErrorCheck(t, "auth fixture", fixture.close)
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(fixture.auth, authCookies, false, &flipGate{enabled: true}, nil)

	w := doGet(r, &http.Cookie{Name: "leafwiki_at", Value: "bogus"})
	if w.Code != http.StatusOK || w.Body.String() != `{"user":null}` {
		t.Fatalf("expected 200 anonymous for invalid token in public mode, got %d %s", w.Code, w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_ValidToken_AttachesUser_BothModes(t *testing.T) {
	fixture := createTestAuthFixture(t)
	cleanupWithErrorCheck(t, "auth fixture", fixture.close)
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	tok, err := fixture.auth.Login("admin", "adminpassword")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	for _, public := range []bool{false, true} {
		r := newAuthOrPublicRouter(fixture.auth, authCookies, false, &flipGate{enabled: public}, nil)
		w := doGet(r, &http.Cookie{Name: "leafwiki_at", Value: tok.Token})
		if w.Code != http.StatusOK || w.Body.String() != `{"user":"admin"}` {
			t.Fatalf("public=%v: expected 200 with admin user attached, got %d %s", public, w.Code, w.Body.String())
		}
	}
}

func TestRequireAuthOrPublicRead_NilAuthServiceWithToken_Returns500(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	// Even in public mode a present token with no authService is a
	// misconfiguration, not a fall-through to anonymous.
	r := newAuthOrPublicRouter(nil, authCookies, false, &flipGate{enabled: true}, nil)

	w := doGet(r, &http.Cookie{Name: "leafwiki_at", Value: "something"})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d %s", w.Code, w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_AuthDisabled_NoUser_PublicOff_Returns401(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(nil, authCookies, true, &flipGate{enabled: false}, nil)

	w := doGet(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Body.String() != `{"error":"User not authenticated and auth is disabled"}` {
		t.Fatalf("expected RequireAuth's auth-disabled body, got %s", w.Body.String())
	}
}

func TestRequireAuthOrPublicRead_NilGate_TreatedAsPublicOff(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	r := newAuthOrPublicRouter(nil, authCookies, false, nil, nil)

	w := doGet(r, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for nil gate, got %d", w.Code)
	}
}

// TestRequireAuthOrPublicRead_GateFlipsAtRuntime_SameRouterChangesBehaviour is
// the "no restart" regression pin: one router, one request shape, behaviour
// follows the gate value at request time — not a value snapshotted when the
// route was registered.
func TestRequireAuthOrPublicRead_GateFlipsAtRuntime_SameRouterChangesBehaviour(t *testing.T) {
	authCookies := authmw.NewAuthCookies(true, time.Hour, 24*time.Hour)
	gate := &flipGate{enabled: false}
	r := newAuthOrPublicRouter(nil, authCookies, false, gate, nil)

	if w := doGet(r, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("public off: expected 401, got %d", w.Code)
	}

	gate.enabled = true
	if w := doGet(r, nil); w.Code != http.StatusOK {
		t.Fatalf("public on: expected 200 on the same router, got %d", w.Code)
	}

	gate.enabled = false
	if w := doGet(r, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("public off again: expected 401 on the same router, got %d", w.Code)
	}
}
