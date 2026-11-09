package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newHeadersTestEngine(cfg HeadersConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(Headers(cfg))
	engine.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	engine.GET("/wiki/static/*filepath", func(c *gin.Context) { c.String(http.StatusOK, "js") })
	engine.GET("/boom", func(*gin.Context) { panic("boom") })
	engine.GET("/override", func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Status(http.StatusNoContent)
	})
	return engine
}

func serveHeadersTest(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func assertBaselineHeaders(t *testing.T, label string, h http.Header) {
	t.Helper()
	want := map[string]string{
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-Content-Type-Options": "nosniff",
	}
	for name, value := range want {
		if got := h.Values(name); len(got) != 1 || got[0] != value {
			t.Errorf("%s: %s = %q, want exactly [%q]", label, name, got, value)
		}
	}
	if got := h.Get("Content-Security-Policy"); got != "" {
		t.Errorf("%s: Content-Security-Policy = %q, want none (the shell owns the document policy)", label, got)
	}
}

func TestHeaders_EveryResponse_GetsBaselineHeaders(t *testing.T) {
	engine := newHeadersTestEngine(HeadersConfig{NosniffExemptPrefix: "/wiki/static/"})

	for _, tc := range []struct {
		path       string
		wantStatus int
	}{
		{"/ok", http.StatusOK},
		{"/unrouted", http.StatusNotFound},
		{"/boom", http.StatusInternalServerError},
		{"/static/x.js", http.StatusNotFound},     // outside the exempt prefix
		{"/wikistatic/x.js", http.StatusNotFound}, // not below the base path
		{"/wiki/staticx.js", http.StatusNotFound}, // the prefix ends with a slash
	} {
		rec := serveHeadersTest(engine, tc.path)
		if rec.Code != tc.wantStatus {
			t.Fatalf("GET %s: status = %d, want %d", tc.path, rec.Code, tc.wantStatus)
		}
		assertBaselineHeaders(t, "GET "+tc.path, rec.Header())
	}
}

// Go derives the type of embedded frontend files from the extension, which
// on Windows reads the registry; a ".js → text/plain" entry plus nosniff would
// block every module script, so /static goes without it.
func TestHeaders_StaticPrefix_HasNoNosniff(t *testing.T) {
	engine := newHeadersTestEngine(HeadersConfig{NosniffExemptPrefix: "/wiki/static/"})

	rec := serveHeadersTest(engine, "/wiki/static/index-abc.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "" {
		t.Errorf("X-Content-Type-Options = %q, want none under the exempt prefix", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("X-Frame-Options = %q, want SAMEORIGIN", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy = %q, want strict-origin-when-cross-origin", got)
	}
}

func TestHeaders_EmptyExemptPrefix_KeepsNosniffEverywhere(t *testing.T) {
	engine := newHeadersTestEngine(HeadersConfig{})

	rec := serveHeadersTest(engine, "/wiki/static/index-abc.js")
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestHeaders_HandlerMayOverride(t *testing.T) {
	engine := newHeadersTestEngine(HeadersConfig{})

	rec := serveHeadersTest(engine, "/override")
	if got := rec.Header().Values("X-Frame-Options"); len(got) != 1 || got[0] != "DENY" {
		t.Errorf("X-Frame-Options = %q, want the handler's [DENY]", got)
	}
}
