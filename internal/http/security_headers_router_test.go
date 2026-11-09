package http_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/test_utils"
)

var baselineSecurityHeaders = map[string]string{
	"X-Frame-Options":        "SAMEORIGIN",
	"Referrer-Policy":        "strict-origin-when-cross-origin",
	"X-Content-Type-Options": "nosniff",
}

func assertBaselineSecurityHeaders(t *testing.T, label string, h http.Header) {
	t.Helper()
	for name, value := range baselineSecurityHeaders {
		if got := h.Values(name); len(got) != 1 || got[0] != value {
			t.Errorf("%s: %s = %q, want exactly [%q]", label, name, got, value)
		}
	}
}

func assertNoCSP(t *testing.T, label string, h http.Header) {
	t.Helper()
	if got := h.Values("Content-Security-Policy"); len(got) != 0 {
		t.Errorf("%s: Content-Security-Policy = %q, want none", label, got)
	}
}

func assertShellCSP(t *testing.T, label string, h http.Header) {
	t.Helper()
	got := h.Values("Content-Security-Policy")
	if len(got) != 1 {
		t.Fatalf("%s: Content-Security-Policy = %q, want exactly one policy", label, got)
	}
	for _, directive := range []string{"frame-ancestors 'self'", "form-action 'self'", "object-src 'none'", "base-uri 'self'"} {
		if !strings.Contains(got[0], directive) {
			t.Errorf("%s: Content-Security-Policy = %q, want it to contain %q", label, got[0], directive)
		}
	}
}

func serveRouter(router http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestSecurityHeaders_ShellRoutes_CarryBaselineAndDocumentPolicy(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	for _, path := range []string{"/", "/login", "/some/wiki/page"} {
		rec := serveRouter(router, http.MethodGet, path)
		assertBaselineSecurityHeaders(t, "GET "+path, rec.Header())
		assertShellCSP(t, "GET "+path, rec.Header())
	}
}

func TestSecurityHeaders_LoginResponse_HasBaselineWithoutCSP(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"identifier":"admin","password":"adminpassword"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	assertBaselineSecurityHeaders(t, "POST /api/auth/login", rec.Header())
	assertNoCSP(t, "POST /api/auth/login", rec.Header())
}

// Requests the shell rejects get a bare 404 and no document policy, whether
// gin matched a route or not.
func TestSecurityHeaders_APIAndUnmatchedNonShellPaths_HaveBaselineWithoutCSP(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/config"},
		{http.MethodGet, "/api/typo"},
		{http.MethodPost, "/some/wiki/page"},
	} {
		label := tc.method + " " + tc.path
		rec := serveRouter(router, tc.method, tc.path)
		assertBaselineSecurityHeaders(t, label, rec.Header())
		assertNoCSP(t, label, rec.Header())
	}

	// The asset route sets its sandbox before it knows whether the file
	// exists; the shell policy must not appear next to it.
	rec := authenticatedRequest(t, router, http.MethodGet, "/assets/missing/x.png", nil)
	assertBaselineSecurityHeaders(t, "GET /assets/missing/x.png", rec.Header())
	if got := rec.Header().Values("Content-Security-Policy"); len(got) > 1 || (len(got) == 1 && got[0] != "sandbox") {
		t.Errorf("GET /assets/missing/x.png: Content-Security-Policy = %q, want at most [sandbox]", got)
	}
}

func TestSecurityHeaders_UploadedFiles_KeepTheirOwnPolicy(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithRevision(w, t)
	page := createPageViaAPI(t, router, "Files", "files", nil, nil)

	htmlURL := uploadAssetViaAPI(t, router, page.ID, "note.html", "<p>hi</p>")
	rec := authenticatedRequest(t, router, http.MethodGet, htmlURL, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d", htmlURL, rec.Code)
	}
	if got := rec.Header().Values("Content-Security-Policy"); len(got) != 1 || got[0] != "sandbox" {
		t.Errorf("GET %s: Content-Security-Policy = %q, want exactly [sandbox]", htmlURL, got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "SAMEORIGIN" {
		t.Errorf("GET %s: X-Frame-Options = %q, want SAMEORIGIN", htmlURL, got)
	}

	// The PDF embed frames same-origin PDFs, live and from a revision.
	pdfURL := uploadAssetViaAPI(t, router, page.ID, "doc.pdf", "%PDF-1.4")
	rec = authenticatedRequest(t, router, http.MethodGet, pdfURL, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d", pdfURL, rec.Code)
	}
	assertBaselineSecurityHeaders(t, "GET "+pdfURL, rec.Header())
	assertNoCSP(t, "GET "+pdfURL, rec.Header())

	rev := getLatestRevisionViaAPI(t, router, page.ID)
	revID, _ := rev["id"].(string)
	if revID == "" {
		t.Fatalf("latest revision has no id: %#v", rev)
	}
	revPath := "/api/pages/" + page.ID + "/revisions/" + revID + "/assets/doc.pdf"
	rec = authenticatedRequest(t, router, http.MethodGet, revPath, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, body = %s", revPath, rec.Code, rec.Body.String())
	}
	assertBaselineSecurityHeaders(t, "GET "+revPath, rec.Header())
	assertNoCSP(t, "GET "+revPath, rec.Header())
}

func TestSecurityHeaders_BasePath_PolicyFollowsTheShell(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.BasePath = "/wiki" })

	rec := serveRouter(router, http.MethodGet, "/wiki/some/page")
	assertBaselineSecurityHeaders(t, "GET /wiki/some/page", rec.Header())
	assertShellCSP(t, "GET /wiki/some/page", rec.Header())

	rec = serveRouter(router, http.MethodGet, "/wiki/api/config")
	assertBaselineSecurityHeaders(t, "GET /wiki/api/config", rec.Header())
	assertNoCSP(t, "GET /wiki/api/config", rec.Header())
}

func TestSecurityHeaders_Disabled_SendsNone(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.DisableSecurityHeaders = true })

	for _, path := range []string{"/", "/some/wiki/page", "/api/config"} {
		rec := serveRouter(router, http.MethodGet, path)
		for name := range baselineSecurityHeaders {
			if got := rec.Header().Values(name); len(got) != 0 {
				t.Errorf("GET %s: %s = %q, want none", path, name, got)
			}
		}
		assertNoCSP(t, "GET "+path, rec.Header())
	}
}

func TestServeShell_EmbeddedShell_CarriesDocumentPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httpinternal.ServeShell(shellTestTemplate, httpinternal.FrontendConfig{}, httpinternal.RouterOptions{}, "/deep/page")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertShellCSP(t, "embedded shell", rec.Header())

	rec = httpinternal.ServeShell(shellTestTemplate, httpinternal.FrontendConfig{}, httpinternal.RouterOptions{DisableSecurityHeaders: true}, "/deep/page")
	assertNoCSP(t, "embedded shell, headers disabled", rec.Header())
}

// Only shell routes get the document policy (HEAD is answered like GET, so it
// carries the same headers); every method and every path the shell rejects
// still gets the baseline.
func TestSecurityHeaders_MethodsAndPathShapes_PolicyOnlyOnShellRoutes(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstance(w, t)

	for _, tc := range []struct {
		method, path string
		wantCSP      bool
	}{
		{http.MethodHead, "/some/wiki/page", true},
		{http.MethodOptions, "/some/wiki/page", false},
		{http.MethodOptions, "/api/config", false},
		{http.MethodDelete, "/", false},
		{http.MethodGet, "/apifoo", false},
		{http.MethodGet, "/staticx", false},
		{http.MethodGet, "/%2e%2e/x", true},
		{http.MethodGet, "/a/../login", true},
	} {
		label := tc.method + " " + tc.path
		rec := serveRouter(router, tc.method, tc.path)
		assertBaselineSecurityHeaders(t, label, rec.Header())
		if tc.wantCSP {
			assertShellCSP(t, label, rec.Header())
		} else {
			assertNoCSP(t, label, rec.Header())
		}
	}
}

func TestSecurityHeaders_MetricsEnabled_KeepsHeaders(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := createRouterTestInstanceWithMetricsEnabled(w, t)

	for _, path := range []string{"/", "/api/config", "/api/typo"} {
		assertBaselineSecurityHeaders(t, "GET "+path, serveRouter(router, http.MethodGet, path).Header())
	}
	assertShellCSP(t, "GET /", serveRouter(router, http.MethodGet, "/").Header())
}

// With a base path, only requests below it are shell routes, and only
// {base}/static/ goes without nosniff.
func TestSecurityHeaders_BasePathBoundaries_PolicyAndNosniffFollowThePrefix(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.BasePath = "/wiki" })

	for _, tc := range []struct {
		path    string
		wantCSP bool
	}{
		{"/wiki", true},
		{"/wiki/", true},
		{"/wikistatic/x.js", false},
		{"/static/x.js", false},
		{"/other/page", false},
		{"/wiki/staticx", false},
	} {
		rec := serveRouter(router, http.MethodGet, tc.path)
		assertBaselineSecurityHeaders(t, "GET "+tc.path, rec.Header())
		if tc.wantCSP {
			assertShellCSP(t, "GET "+tc.path, rec.Header())
		} else {
			assertNoCSP(t, "GET "+tc.path, rec.Header())
		}
	}
}

func TestSecurityHeaders_ShellPolicy_RestrictsScriptsUnlessHeaderCodeRunsScript(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	for _, tc := range []struct {
		name       string
		headerCode string
		wantScript bool
	}{
		{"no header code", "", true},
		{"meta-only header code", `<meta name="google-site-verification" content="x">`, true},
		{"script in header code", `<script src="https://stats.example/a.js"></script>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := newTestRouter(w, func(o *httpinternal.RouterOptions) { o.InjectCodeInHeader = tc.headerCode })
			rec := serveRouter(router, http.MethodGet, "/some/wiki/page")
			assertShellCSP(t, "GET /some/wiki/page", rec.Header())
			got := strings.Contains(rec.Header().Get("Content-Security-Policy"), "script-src 'self'")
			if got != tc.wantScript {
				t.Errorf("script-src 'self' present = %v, want %v (policy %q)", got, tc.wantScript, rec.Header().Get("Content-Security-Policy"))
			}
		})
	}
}
