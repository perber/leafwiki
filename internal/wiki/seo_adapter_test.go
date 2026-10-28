package wiki

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
)

// newSEOTestRouter builds a real wiki (with its welcome page) and the router
// on top of it; public access is settings-managed so tests can toggle it.
func newSEOTestRouter(t *testing.T, basePath string) (*Wiki, *publicaccess.Service, *gin.Engine) {
	t.Helper()
	dir := t.TempDir()
	public, err := publicaccess.NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}
	w, err := NewWiki(&WikiOptions{
		StorageDir:          dir,
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		PublicAccess:        public,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		BasePath:          basePath,
		PublicAccess:      public,
		DisableRequestLog: true,
	})
	return w, public, router
}

func setPublicAccess(t *testing.T, public *publicaccess.Service, enabled bool) {
	t.Helper()
	if err := public.SetEnabled(enabled); err != nil {
		t.Fatalf("SetEnabled(%v): %v", enabled, err)
	}
}

func serveGET(router http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func assertNoRedirect(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code == http.StatusFound || rec.Header().Get("Location") != "" {
		t.Fatalf("status = %d, Location = %q; want no redirect", rec.Code, rec.Header().Get("Location"))
	}
}

func TestWiki_RootRedirect_PublicWiki_RedirectsToFirstPage(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		path     string
		headers  map[string]string
		want     string
	}{
		{name: "root without base path", path: "/", want: "/welcome-to-leafwiki"},
		{name: "base path with trailing slash", basePath: "/wiki", path: "/wiki/", want: "/wiki/welcome-to-leafwiki"},
		{name: "base path without trailing slash", basePath: "/wiki", path: "/wiki", want: "/wiki/welcome-to-leafwiki"},
		{
			name:     "forwarded headers have no effect",
			basePath: "/wiki",
			path:     "/wiki/",
			headers:  map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http"},
			want:     "/wiki/welcome-to-leafwiki",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, public, router := newSEOTestRouter(t, tc.basePath)
			setPublicAccess(t, public, true)

			rec := serveGET(router, tc.path, tc.headers)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Errorf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWiki_RootRedirect_FollowsPublicAccessAtRuntime(t *testing.T) {
	_, public, router := newSEOTestRouter(t, "/wiki")

	assertNoRedirect(t, serveGET(router, "/wiki/", nil))

	setPublicAccess(t, public, true)
	if rec := serveGET(router, "/wiki/", nil); rec.Code != http.StatusFound {
		t.Fatalf("after enabling public access: status = %d, want 302", rec.Code)
	}

	setPublicAccess(t, public, false)
	assertNoRedirect(t, serveGET(router, "/wiki/", nil))
}

func TestWiki_RootRedirect_NoRedirect(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		prepare func(t *testing.T, w *Wiki)
	}{
		{name: "page route", path: "/wiki/welcome-to-leafwiki"},
		{
			name: "empty tree",
			path: "/wiki/",
			prepare: func(t *testing.T, w *Wiki) {
				deletePageForTest(t, w, SYSTEM_USER_ID, w.tree.GetTree().Children[0].ID, true)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, public, router := newSEOTestRouter(t, "/wiki")
			setPublicAccess(t, public, true)
			if tc.prepare != nil {
				tc.prepare(t, w)
			}

			assertNoRedirect(t, serveGET(router, tc.path, nil))
		})
	}
}
