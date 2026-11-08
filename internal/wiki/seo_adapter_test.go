package wiki

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

// newSEOTestRouter builds a real wiki (with its welcome page) and the router
// on top of it; public access is settings-managed so tests can toggle it.
func newSEOTestRouter(t *testing.T, basePath string) (*Wiki, *publicaccess.Service, *gin.Engine) {
	t.Helper()
	return newSEOTestRouterWithPublicURL(t, basePath, "")
}

// newSEOTestRouterWithPublicURL is newSEOTestRouter with --public-url set.
func newSEOTestRouterWithPublicURL(t *testing.T, basePath, publicURL string) (*Wiki, *publicaccess.Service, *gin.Engine) {
	t.Helper()
	w, public := newSEOTestWiki(t, publicURL)
	router := httpinternal.NewRouter(w.Registrars(), w.FrontendConfig(), httpinternal.RouterOptions{
		BasePath:          basePath,
		PublicAccess:      public,
		DisableRequestLog: true,
	})
	return w, public, router
}

// newSEOTestWiki builds a real wiki (with its welcome page) with public access
// off and settings-managed, so tests can toggle it.
func newSEOTestWiki(t *testing.T, publicURL string) (*Wiki, *publicaccess.Service) {
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
		PublicURL:           publicURL,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w, public
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
		prepare  func(t *testing.T, w *Wiki)
		want     string
	}{
		{name: "root without base path", path: "/", want: "/welcome-to-leafwiki"},
		{name: "query string on root is dropped like the SPA does", basePath: "/wiki", path: "/wiki/?utm_source=x", want: "/wiki/welcome-to-leafwiki"},
		{
			name: "first page renamed after boot",
			path: "/",
			prepare: func(t *testing.T, w *Wiki) {
				first := w.tree.GetTree().Children[0]
				updatePageForTest(t, w, SYSTEM_USER_ID, first.ID, "Start", "start", nil, nil)
			},
			want: "/start",
		},
		{
			name: "pages reordered after boot, section first",
			path: "/",
			prepare: func(t *testing.T, w *Wiki) {
				welcome := w.tree.GetTree().Children[0]
				sectionKind := tree.NodeKindSection
				section := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Docs", "docs", &sectionKind)
				createPageForTest(t, w, SYSTEM_USER_ID, &section.ID, "Setup", "setup", pageNodeKind())
				if err := w.tree.SortPages("root", []string{section.ID, welcome.ID}); err != nil {
					t.Fatalf("SortPages: %v", err)
				}
			},
			want: "/docs",
		},
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
			w, public, router := newSEOTestRouter(t, tc.basePath)
			setPublicAccess(t, public, true)
			if tc.prepare != nil {
				tc.prepare(t, w)
			}

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

// HEAD / answers like GET without a body, so crawlers and monitors that
// probe with HEAD see the same redirect.
func TestWiki_RootRedirect_Head_SameAsGet(t *testing.T) {
	_, public, router := newSEOTestRouter(t, "/wiki")
	setPublicAccess(t, public, true)

	get := serveGET(router, "/wiki/", nil)
	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/wiki/", nil))

	if head.Code != http.StatusFound || head.Code != get.Code {
		t.Fatalf("status HEAD=%d GET=%d, want 302", head.Code, get.Code)
	}
	if got, want := head.Header().Get("Location"), get.Header().Get("Location"); got != want || got == "" {
		t.Fatalf("Location HEAD=%q GET=%q", got, want)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q, want empty", head.Body.String())
	}
}

func TestWiki_RootRedirect_NoRedirect(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		prepare func(t *testing.T, w *Wiki)
	}{
		{name: "page route", path: "/wiki/welcome-to-leafwiki"},
		{name: "index.html is not the root", path: "/wiki/index.html"},
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

// contributeShell runs the wiki's registered shell contributors the way the
// router does for a GET of basePath+routePath and merges their parts.
func contributeShell(t *testing.T, w *Wiki, basePath, routePath string, headers map[string]string) httpinternal.ShellParts {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, basePath+routePath, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	var merged httpinternal.ShellParts
	for _, c := range w.FrontendConfig().ShellContributors {
		parts := c.ContributeShell(req.Context(), httpinternal.ShellRequest{Request: req, RoutePath: routePath, BasePath: basePath})
		if merged.Title == "" {
			merged.Title = parts.Title
		}
		merged.Head += parts.Head
		merged.Body += parts.Body
		merged.NoCache = merged.NoCache || parts.NoCache
	}
	return merged
}

func createDescribedPage(t *testing.T, w *Wiki, title, slug, body, description string) {
	t.Helper()
	page := createPageForTest(t, w, SYSTEM_USER_ID, nil, title, slug, pageNodeKind())
	_, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log, nil).Execute(
		context.Background(),
		wikipages.UpdatePageInput{
			UserID: SYSTEM_USER_ID, ID: page.ID, Version: page.Version(), Title: title, Slug: slug,
			Content: &body, Kind: pageNodeKind(), Properties: map[string]string{"description": description},
		},
	)
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
}

func TestWiki_PageMeta_PublicWiki_AddsMetaTags(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		basePath  string
		siteName  string
		headers   map[string]string
		wantTitle string
		wantHead  string
	}{
		{
			name:      "without public url: no canonical link",
			wantTitle: "Setup - LeafWiki",
			wantHead: `<meta name="description" content="How to set it up.">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="Setup">` +
				`<meta property="og:description" content="How to set it up.">` +
				`<meta property="og:site_name" content="LeafWiki">`,
		},
		{
			name:      "with public url, base path and branding site name",
			publicURL: "https://wiki.example.com/wiki",
			basePath:  "/wiki",
			siteName:  "Team Wiki",
			headers:   map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http"},
			wantTitle: "Setup - Team Wiki",
			wantHead: `<meta name="description" content="How to set it up.">` +
				`<link rel="canonical" href="https://wiki.example.com/wiki/setup">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="Setup">` +
				`<meta property="og:description" content="How to set it up.">` +
				`<meta property="og:site_name" content="Team Wiki">` +
				`<meta property="og:url" content="https://wiki.example.com/wiki/setup">`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, public := newSEOTestWiki(t, tc.publicURL)
			setPublicAccess(t, public, true)
			if tc.siteName != "" {
				if err := w.branding.UpdateBranding(tc.siteName); err != nil {
					t.Fatalf("UpdateBranding: %v", err)
				}
			}
			createDescribedPage(t, w, "Setup", "setup", "# Setup\n\nInstall it.", "How to set it up.")

			got := contributeShell(t, w, tc.basePath, "/setup", tc.headers)

			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if string(got.Head) != tc.wantHead {
				t.Errorf("Head =\n%s\nwant\n%s", got.Head, tc.wantHead)
			}
			if !got.NoCache {
				t.Error("NoCache = false, want true")
			}
		})
	}
}

func TestWiki_PageMeta_ExcerptFallback_UsesPageBody(t *testing.T) {
	w, public := newSEOTestWiki(t, "")
	setPublicAccess(t, public, true)
	body := "# Setup\n\nInstall it."
	page := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Setup", "setup", pageNodeKind())
	updatePageForTest(t, w, SYSTEM_USER_ID, page.ID, "Setup", "setup", &body, pageNodeKind())

	got := contributeShell(t, w, "", "/setup", nil)

	if want := `<meta name="description" content="Setup Install it.">`; !strings.Contains(string(got.Head), want) {
		t.Fatalf("Head = %s, want it to contain %s", got.Head, want)
	}
}

func TestWiki_PageMeta_FollowsPublicAccessAtRuntime(t *testing.T) {
	w, public := newSEOTestWiki(t, "https://wiki.example.com")

	if got := contributeShell(t, w, "", "/welcome-to-leafwiki", nil); got != (httpinternal.ShellParts{}) {
		t.Fatalf("public access off: parts = %+v, want none", got)
	}

	setPublicAccess(t, public, true)
	if got := contributeShell(t, w, "", "/welcome-to-leafwiki", nil); got.Title != "Welcome to LeafWiki - LeafWiki" {
		t.Fatalf("after enabling public access: Title = %q", got.Title)
	}

	setPublicAccess(t, public, false)
	if got := contributeShell(t, w, "", "/welcome-to-leafwiki", nil); got != (httpinternal.ShellParts{}) {
		t.Fatalf("after disabling public access: parts = %+v, want none", got)
	}
}

func TestWiki_PageMeta_NotAPage_ContributesNothing(t *testing.T) {
	tests := []struct {
		name      string
		routePath string
	}{
		{name: "root", routePath: "/"},
		{name: "unknown page", routePath: "/missing"},
		{name: "unknown child of a page", routePath: "/welcome-to-leafwiki/missing"},
		{name: "trailing slash", routePath: "/welcome-to-leafwiki/"},
		{name: "SPA route", routePath: "/settings/branding"},
	}
	w, public := newSEOTestWiki(t, "https://wiki.example.com")
	setPublicAccess(t, public, true)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := contributeShell(t, w, "", tc.routePath, nil); got != (httpinternal.ShellParts{}) {
				t.Fatalf("parts = %+v, want none", got)
			}
		})
	}
}

func TestSEOPageReader_TreeNotLoaded_ReturnsNoPage(t *testing.T) {
	reader := seoPageReader{uc: wikipages.NewGetPageViewByPathUseCase(tree.NewTreeService(t.TempDir()))}

	page, err := reader.PageByRoutePath(context.Background(), "docs")
	if page != nil || err != nil {
		t.Fatalf("PageByRoutePath = %+v, %v; want nil, nil", page, err)
	}
}

// HEAD on the real router answers like GET without a body. Routes that
// registrars add for GET only must not let HEAD fall through to the SPA
// shell: a link checker would see 200 text/html for an icon or a missing
// avatar.
func TestWiki_Head_MatchesGetOnRealRouter(t *testing.T) {
	old := httpinternal.EmbedFrontend
	httpinternal.EmbedFrontend = "true"
	t.Cleanup(func() { httpinternal.EmbedFrontend = old })
	_, public, router := newSEOTestRouter(t, "/wiki")
	setPublicAccess(t, public, true)
	if rec := serveGET(router, "/wiki/welcome-to-leafwiki", nil); !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Skip("requires built frontend (make ui)")
	}

	tests := []struct {
		name   string
		public bool
		path   string
		// sameAsGet: status and headers match GET. Otherwise HEAD only has
		// to stay out of the shell (GET-only routes answer HEAD with 404).
		sameAsGet bool
	}{
		{name: "root, public", public: true, path: "/wiki/", sameAsGet: true},
		{name: "root, private", public: false, path: "/wiki/", sameAsGet: true},
		{name: "page, public", public: true, path: "/wiki/welcome-to-leafwiki", sameAsGet: true},
		{name: "page, private", public: false, path: "/wiki/welcome-to-leafwiki", sameAsGet: true},
		{name: "SPA route", public: true, path: "/wiki/settings/branding", sameAsGet: true},
		{name: "favicon.svg", public: true, path: "/wiki/favicon.svg", sameAsGet: true},
		{name: "favicon.ico", public: true, path: "/wiki/favicon.ico"},
		{name: "robots.txt, public", public: true, path: "/wiki/robots.txt", sameAsGet: true},
		{name: "robots.txt, private", public: false, path: "/wiki/robots.txt", sameAsGet: true},
		{name: "sitemap.xml, private", public: false, path: "/wiki/sitemap.xml", sameAsGet: true},
		{name: "avatar of unknown user", public: true, path: "/wiki/avatars/nobody"},
		{name: "branding asset", public: true, path: "/wiki/branding/missing.png"},
		{name: "api health", public: true, path: "/wiki/api/health"},
		{name: "unknown api", public: true, path: "/wiki/api/unknown", sameAsGet: true},
		{name: "unknown static", public: true, path: "/wiki/static/missing.js", sameAsGet: true},
		{name: "unknown asset", public: true, path: "/wiki/assets/missing.png", sameAsGet: true},
		{name: "outside base path", public: true, path: "/other/page", sameAsGet: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setPublicAccess(t, public, tc.public)
			get := serveGET(router, tc.path, nil)
			head := httptest.NewRecorder()
			router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, tc.path, nil))
			headCT, getCT := head.Header().Get("Content-Type"), get.Header().Get("Content-Type")
			// The recorder keeps bodies that net/http drops for HEAD; only the
			// shell's own writer has to drop it.
			if strings.HasPrefix(headCT, "text/html") && head.Body.Len() != 0 {
				t.Fatalf("HEAD shell body = %q, want empty", head.Body.String())
			}
			if !tc.sameAsGet {
				if strings.HasPrefix(headCT, "text/html") && !strings.HasPrefix(getCT, "text/html") {
					t.Fatalf("HEAD got the SPA shell (%d %q), GET %d %q", head.Code, headCT, get.Code, getCT)
				}
				return
			}
			if head.Code != get.Code || headCT != getCT {
				t.Fatalf("HEAD %d %q, GET %d %q", head.Code, headCT, get.Code, getCT)
			}
			for _, h := range []string{"Location", "Cache-Control", "Vary", "Content-Length"} {
				if got, want := head.Header().Get(h), get.Header().Get(h); got != want {
					t.Errorf("%s: HEAD %q, GET %q", h, got, want)
				}
			}
		})
	}
}

func TestWiki_RobotsAndSitemap_FollowPublicAccessAtRuntime(t *testing.T) {
	w, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", "https://wiki.example.com/wiki")
	sectionKind := tree.NodeKindSection
	section := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Docs", "docs", &sectionKind)
	createPageForTest(t, w, SYSTEM_USER_ID, &section.ID, "Setup", "setup", pageNodeKind())

	tests := []struct {
		name        string
		public      bool
		path        string
		wantStatus  int
		wantType    string
		wantContain []string
		wantMissing []string
	}{
		{
			name: "robots.txt, private", path: "/wiki/robots.txt", wantStatus: http.StatusOK, wantType: "text/plain; charset=utf-8",
			wantContain: []string{"Disallow: /"}, wantMissing: []string{"Sitemap:"},
		},
		{name: "sitemap.xml, private", path: "/wiki/sitemap.xml", wantStatus: http.StatusNotFound},
		{
			name: "robots.txt, public", public: true, path: "/wiki/robots.txt", wantStatus: http.StatusOK, wantType: "text/plain; charset=utf-8",
			wantContain: []string{"Allow: /", "Sitemap: https://wiki.example.com/wiki/sitemap.xml"}, wantMissing: []string{"Disallow"},
		},
		{
			name: "sitemap.xml, public", public: true, path: "/wiki/sitemap.xml", wantStatus: http.StatusOK, wantType: "application/xml; charset=utf-8",
			wantContain: []string{
				"<loc>https://wiki.example.com/wiki/welcome-to-leafwiki</loc>",
				"<loc>https://wiki.example.com/wiki/docs</loc>",
				"<loc>https://wiki.example.com/wiki/docs/setup</loc>",
				"<lastmod>",
			},
			wantMissing: []string{"<loc>https://wiki.example.com/wiki/</loc>"},
		},
		{name: "robots.txt outside the base path", public: true, path: "/robots.txt", wantStatus: http.StatusNotFound},
		{name: "robots.txt, private again", path: "/wiki/robots.txt", wantStatus: http.StatusOK, wantContain: []string{"Disallow: /"}},
		{name: "sitemap.xml, private again", path: "/wiki/sitemap.xml", wantStatus: http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setPublicAccess(t, public, tc.public)

			rec := serveGET(router, tc.path, nil)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantType != "" && rec.Header().Get("Content-Type") != tc.wantType {
				t.Errorf("Content-Type = %q, want %q", rec.Header().Get("Content-Type"), tc.wantType)
			}
			for _, s := range tc.wantContain {
				if !strings.Contains(rec.Body.String(), s) {
					t.Errorf("body does not contain %q:\n%s", s, rec.Body.String())
				}
			}
			for _, s := range tc.wantMissing {
				if strings.Contains(rec.Body.String(), s) {
					t.Errorf("body contains %q:\n%s", s, rec.Body.String())
				}
			}
		})
	}
}

func TestWiki_Sitemap_WithoutPublicURL_NotFound(t *testing.T) {
	_, public, router := newSEOTestRouter(t, "")
	setPublicAccess(t, public, true)

	if rec := serveGET(router, "/sitemap.xml", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if rec := serveGET(router, "/robots.txt", nil); strings.Contains(rec.Body.String(), "Sitemap:") {
		t.Fatalf("robots.txt announces a sitemap without --public-url:\n%s", rec.Body.String())
	}
}

func TestSEOPageRouteLister_TreeNotLoaded_ReturnsError(t *testing.T) {
	lister := seoPageRouteLister{uc: wikipages.NewListPageRoutesUseCase(tree.NewTreeService(t.TempDir()))}

	routes, truncated, err := lister.ListPageRoutes(context.Background(), 0)
	if !errors.Is(err, tree.ErrTreeNotLoaded) || routes != nil || truncated {
		t.Fatalf("ListPageRoutes = %v, %v, %v; want nil, false, ErrTreeNotLoaded", routes, truncated, err)
	}
}
