package wiki

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
		{name: "permalink with unknown id", routePath: "/p/unknownid"},
		{name: "permalink of the root", routePath: "/p/root"},
		{name: "permalink route without id", routePath: "/p/"},
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

func movePageForTest(t *testing.T, w *Wiki, page *tree.Page, parentID string) {
	t.Helper()
	current, err := w.tree.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	err = wikipages.NewMovePageUseCase(w.tree, w.newPageOrchestrator(), w.log, nil).Execute(context.Background(), wikipages.MovePageInput{
		UserID: SYSTEM_USER_ID, ID: page.ID, Version: current.Version(), ParentID: parentID,
	})
	if err != nil {
		t.Fatalf("MovePage: %v", err)
	}
}

// A permalink gets the meta of the page its ID points at, with the page's
// current route path as canonical URL, also after the page moved.
func TestWiki_PageMeta_Permalink_CanonicalFollowsThePage(t *testing.T) {
	w, public := newSEOTestWiki(t, "https://wiki.example.com")
	page := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Setup", "setup", pageNodeKind())
	parent := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Guide", "guide", pageNodeKind())
	canonical := func(routePath string) string {
		t.Helper()
		head := string(contributeShell(t, w, "", routePath, nil).Head)
		_, href, ok := strings.Cut(head, `<link rel="canonical" href="`)
		if !ok {
			return ""
		}
		href, _, _ = strings.Cut(href, `"`)
		return href
	}

	if got := contributeShell(t, w, "", "/p/"+page.ID, nil); got != (httpinternal.ShellParts{}) {
		t.Fatalf("public access off: parts = %+v, want none", got)
	}
	setPublicAccess(t, public, true)
	for _, routePath := range []string{"/p/" + page.ID, "/p/" + page.ID + "/setup", "/p/" + page.ID + "/wrong-slug"} {
		if got := canonical(routePath); got != "https://wiki.example.com/setup" {
			t.Errorf("%s: canonical = %q, want https://wiki.example.com/setup", routePath, got)
		}
	}
	if got := contributeShell(t, w, "", "/p/"+page.ID+"/setup/more", nil); got != (httpinternal.ShellParts{}) {
		t.Errorf("extra segments: parts = %+v, want none", got)
	}

	movePageForTest(t, w, page, parent.ID)

	if got := canonical("/p/" + page.ID + "/setup"); got != "https://wiki.example.com/guide/setup" {
		t.Errorf("after the move: canonical = %q, want https://wiki.example.com/guide/setup", got)
	}
	if got := contributeShell(t, w, "", "/p/"+page.ID, nil).Title; got != "Setup - LeafWiki" {
		t.Errorf("after the move: Title = %q, want Setup - LeafWiki", got)
	}
}

// On the real router a permalink is the 200 shell with the target's
// canonical link, and Markdown clients get that shell too.
func TestWiki_PageMeta_Permalink_ServedInTheShell(t *testing.T) {
	useEmbeddedFrontend(t)
	w, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", "https://wiki.example.com/wiki")
	setPublicAccess(t, public, true)
	id := w.tree.GetTree().Children[0].ID
	path := "/wiki/p/" + id + "/old-slug"
	wantLink := `<link rel="canonical" href="https://wiki.example.com/wiki/welcome-to-leafwiki">`

	for _, accept := range []string{"", "text/markdown"} {
		rec := serveAccept(router, http.MethodGet, path, accept, nil)

		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("Accept %q: got %d %q, want the 200 shell", accept, rec.Code, rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), wantLink) {
			t.Errorf("Accept %q: shell does not contain %s", accept, wantLink)
		}
	}
}

// The node a permalink ID points at decides the meta, whatever its kind,
// position or history; the request path under "p/" never does.
func TestWiki_PageMeta_Permalink_TargetNode(t *testing.T) {
	sectionKind := tree.NodeKindSection
	tests := []struct {
		name string
		// prepare builds the wiki and returns the shell route path.
		prepare       func(t *testing.T, w *Wiki) string
		wantTitle     string // "" = no meta at all
		wantCanonical string
	}{
		{
			name: "section",
			prepare: func(t *testing.T, w *Wiki) string {
				section := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Docs", "docs", &sectionKind)
				return "/p/" + section.ID
			},
			wantTitle: "Docs - LeafWiki", wantCanonical: "https://wiki.example.com/docs",
		},
		{
			name: "nested page with a unicode title",
			prepare: func(t *testing.T, w *Wiki) string {
				section := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Docs", "docs", &sectionKind)
				page := createPageForTest(t, w, SYSTEM_USER_ID, &section.ID, "Größe", "groesse", pageNodeKind())
				return "/p/" + page.ID + "/stale"
			},
			wantTitle: "Größe - LeafWiki", wantCanonical: "https://wiki.example.com/docs/groesse",
		},
		{
			name: "renamed slug",
			prepare: func(t *testing.T, w *Wiki) string {
				page := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Setup", "setup", pageNodeKind())
				updatePageForTest(t, w, SYSTEM_USER_ID, page.ID, "Install", "install", nil, nil)
				return "/p/" + page.ID + "/setup"
			},
			wantTitle: "Install - LeafWiki", wantCanonical: "https://wiki.example.com/install",
		},
		{
			name: "deleted page",
			prepare: func(t *testing.T, w *Wiki) string {
				page := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Setup", "setup", pageNodeKind())
				deletePageForTest(t, w, SYSTEM_USER_ID, page.ID, false)
				return "/p/" + page.ID
			},
		},
		{
			// The SPA shows a not-found permalink at /p/legacy, not the page.
			name: "real page at p/<slug> is no permalink target",
			prepare: func(t *testing.T, w *Wiki) string {
				p := createPageForTest(t, w, SYSTEM_USER_ID, nil, "P", "p", pageNodeKind())
				createPageForTest(t, w, SYSTEM_USER_ID, &p.ID, "Legacy", "legacy", pageNodeKind())
				return "/p/legacy"
			},
		},
		{
			name: "real page p itself keeps its own meta",
			prepare: func(t *testing.T, w *Wiki) string {
				createPageForTest(t, w, SYSTEM_USER_ID, nil, "P", "p", pageNodeKind())
				return "/p"
			},
			wantTitle: "P - LeafWiki", wantCanonical: "https://wiki.example.com/p",
		},
		{
			// /p/a/b is the permalink "a" with slug "b" in the SPA too.
			name: "real page at p/<slug>/<slug> is no permalink target",
			prepare: func(t *testing.T, w *Wiki) string {
				p := createPageForTest(t, w, SYSTEM_USER_ID, nil, "P", "p", pageNodeKind())
				a := createPageForTest(t, w, SYSTEM_USER_ID, &p.ID, "A", "a", pageNodeKind())
				createPageForTest(t, w, SYSTEM_USER_ID, &a.ID, "B", "b", pageNodeKind())
				return "/p/a/b"
			},
		},
		{
			name: "real page three levels under p keeps its own meta",
			prepare: func(t *testing.T, w *Wiki) string {
				p := createPageForTest(t, w, SYSTEM_USER_ID, nil, "P", "p", pageNodeKind())
				a := createPageForTest(t, w, SYSTEM_USER_ID, &p.ID, "A", "a", pageNodeKind())
				b := createPageForTest(t, w, SYSTEM_USER_ID, &a.ID, "B", "b", pageNodeKind())
				createPageForTest(t, w, SYSTEM_USER_ID, &b.ID, "C", "c", pageNodeKind())
				return "/p/a/b/c"
			},
			wantTitle: "C - LeafWiki", wantCanonical: "https://wiki.example.com/p/a/b/c",
		},
		{
			name:    "id with path characters",
			prepare: func(t *testing.T, w *Wiki) string { return "/p/../welcome-to-leafwiki" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, public := newSEOTestWiki(t, "https://wiki.example.com")
			setPublicAccess(t, public, true)
			routePath := tc.prepare(t, w)

			got := contributeShell(t, w, "", routePath, nil)

			if tc.wantTitle == "" {
				if got != (httpinternal.ShellParts{}) {
					t.Fatalf("parts = %+v, want none", got)
				}
				return
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			wantLink := `<link rel="canonical" href="` + tc.wantCanonical + `">`
			if !strings.Contains(string(got.Head), wantLink) {
				t.Errorf("Head = %s\nwant it to contain %s", got.Head, wantLink)
			}
		})
	}
}

// The target page's title reaches the head escaped, as on its own route.
func TestWiki_PageMeta_Permalink_EscapesTheTargetTitle(t *testing.T) {
	w, public := newSEOTestWiki(t, "")
	setPublicAccess(t, public, true)
	page := createPageForTest(t, w, SYSTEM_USER_ID, nil, `Fish & <Chips> "x"`, "fish", pageNodeKind())

	viaPermalink := contributeShell(t, w, "", "/p/"+page.ID, nil)
	viaPath := contributeShell(t, w, "", "/fish", nil)

	if viaPermalink != viaPath {
		t.Fatalf("permalink parts = %+v\npath parts      = %+v", viaPermalink, viaPath)
	}
	if strings.Contains(string(viaPermalink.Head), "<Chips>") {
		t.Errorf("unescaped title in head: %s", viaPermalink.Head)
	}
}

// Host and X-Forwarded-* never reach a permalink's canonical link on the
// real router; without --public-url there is none at all.
func TestWiki_PageMeta_Permalink_IgnoresRequestHostHeaders(t *testing.T) {
	useEmbeddedFrontend(t)
	for _, publicURL := range []string{"https://wiki.example.com/wiki", ""} {
		w, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", publicURL)
		setPublicAccess(t, public, true)
		req := httptest.NewRequest(http.MethodGet, "/wiki/p/"+w.tree.GetTree().Children[0].ID, nil)
		req.Host = "evil.example"
		for k, v := range map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http", "X-Forwarded-Prefix": "/evil"} {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(body, `<meta property="og:title" content="Welcome to LeafWiki">`) {
			t.Fatalf("public url %q: got %d without the page meta", publicURL, rec.Code)
		}
		if strings.Contains(body, "evil") {
			t.Errorf("public url %q: shell uses a request header", publicURL)
		}
		if hasCanonical := strings.Contains(body, `rel="canonical"`); hasCanonical != (publicURL != "") {
			t.Errorf("public url %q: canonical link present = %v", publicURL, hasCanonical)
		}
	}
}

// On the real router the shell sees the decoded path, so an encoded slash
// splits the ID ("<id>%2Fx" is the ID <id> with slug x). The SPA keeps %2F
// inside the segment and asks for the ID "<id>/x", which does not exist; the
// canonical link still names the real page.
func TestWiki_PageMeta_Permalink_EncodedSlash(t *testing.T) {
	useEmbeddedFrontend(t)
	w, public, router := newSEOTestRouterWithPublicURL(t, "", "https://wiki.example.com")
	setPublicAccess(t, public, true)
	id := w.tree.GetTree().Children[0].ID

	rec := serveGET(router, "/p/"+id+"%2Fx", nil)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `<link rel="canonical" href="https://wiki.example.com/welcome-to-leafwiki">`) {
		t.Fatalf("got %d, want the shell with the target page canonical", rec.Code)
	}
}

// Outside the base path a permalink is no shell route at all.
func TestWiki_PageMeta_Permalink_OutsideBasePath_NoShell(t *testing.T) {
	useEmbeddedFrontend(t)
	w, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", "https://wiki.example.com/wiki")
	setPublicAccess(t, public, true)

	rec := serveGET(router, "/p/"+w.tree.GetTree().Children[0].ID, nil)

	if strings.Contains(rec.Body.String(), "og:title") {
		t.Fatalf("got %d with page meta outside the base path", rec.Code)
	}
}

func TestSEOPermalinkResolver_UnknownOrNotLoaded_ReturnsNoTarget(t *testing.T) {
	w, _ := newSEOTestWiki(t, "")
	tests := []struct {
		name  string
		trees *tree.TreeService
	}{
		{name: "unknown id", trees: w.tree},
		{name: "tree not loaded", trees: tree.NewTreeService(t.TempDir())},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := seoPermalinkResolver{queries: wikipages.NewPageQueries(tc.trees)}

			target, err := resolver.ResolvePermalink(context.Background(), "unknownid")
			if target != nil || err != nil {
				t.Fatalf("ResolvePermalink = %+v, %v; want nil, nil", target, err)
			}
		})
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
	w, public, router := newSEOTestRouter(t, "/wiki")
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
		{name: "permalink, public", public: true, path: "/wiki/p/" + w.tree.GetTree().Children[0].ID + "/stale", sameAsGet: true},
		{name: "permalink, private", public: false, path: "/wiki/p/" + w.tree.GetTree().Children[0].ID, sameAsGet: true},
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
		{
			name: "sitemap.xml, public again", public: true, path: "/wiki/sitemap.xml", wantStatus: http.StatusOK,
			wantContain: []string{"<loc>https://wiki.example.com/wiki/docs/setup</loc>"},
		},
		{name: "robots.txt, public again", public: true, path: "/wiki/robots.txt", wantStatus: http.StatusOK, wantMissing: []string{"Disallow"}},
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

// Look-alike paths on the real router (shell fallback, trailing-slash
// redirects) never answer with robots or sitemap content, and a private wiki
// lists no page through any of them.
func TestWiki_RobotsAndSitemap_LookalikePaths_LeakNothing(t *testing.T) {
	w, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", "https://wiki.example.com/wiki")
	createPageForTest(t, w, SYSTEM_USER_ID, nil, "Secret Plans", "secret-plans", pageNodeKind())
	paths := []string{
		"/wiki/Robots.txt", "/wiki/ROBOTS.TXT", "/wiki/Sitemap.xml", "/wiki/sitemap.XML", "/wiki/sitemap.xml.gz",
		"/wiki/robots.txt/", "/wiki/sitemap.xml/", "/wiki/sitemap.xml/x", "/wiki//sitemap.xml", "/wiki/./sitemap.xml",
		"/robots.txt", "/sitemap.xml", "/wiki/api/sitemap.xml",
	}
	// Percent-encoded unreserved characters are the same URL (RFC 3986 6.2.2.2):
	// they may serve the files, but only through the public-access gate.
	encoded := []string{"/wiki/%73itemap.xml", "/wiki/sitemap%2Exml", "/wiki/robots%2Etxt"}
	for _, enabled := range []bool{false, true} {
		setPublicAccess(t, public, enabled)
		for _, p := range paths {
			rec := serveGET(router, p, nil)
			if rec.Code >= 500 {
				t.Errorf("public=%v %s: status %d", enabled, p, rec.Code)
			}
			body := rec.Body.String()
			if strings.Contains(body, "<urlset") || strings.Contains(body, "User-agent") {
				// A redirect to the canonical path is fine, content is not.
				t.Errorf("public=%v %s: seo content (%d):\n%s", enabled, p, rec.Code, body)
			}
			if !enabled && strings.Contains(body, "secret-plans") {
				t.Errorf("private %s lists a page (%d):\n%s", p, rec.Code, body)
			}
			if loc := rec.Header().Get("Location"); loc != "" && !strings.HasPrefix(loc, "/wiki/") {
				t.Errorf("public=%v %s: redirect leaves the wiki: %q", enabled, p, loc)
			}
		}
		for _, p := range encoded {
			rec := serveGET(router, p, nil)
			if rec.Code >= 500 {
				t.Errorf("public=%v %s: status %d", enabled, p, rec.Code)
			}
			if !enabled && (strings.Contains(rec.Body.String(), "<loc>") || strings.Contains(rec.Body.String(), "Allow: /\n")) {
				t.Errorf("private %s passes the gate (%d):\n%s", p, rec.Code, rec.Body.String())
			}
		}
	}
}

// Pages whose slugs look like the seo files are ordinary pages: listed in the
// sitemap, and they do not shadow robots.txt or sitemap.xml.
func TestWiki_Sitemap_LookalikeSlugs_AreOrdinaryPages(t *testing.T) {
	w, public, router := newSEOTestRouterWithPublicURL(t, "", "https://wiki.example.com")
	setPublicAccess(t, public, true)
	sectionKind := tree.NodeKindSection
	createPageForTest(t, w, SYSTEM_USER_ID, nil, "Robots", "robots-txt", pageNodeKind())
	section := createPageForTest(t, w, SYSTEM_USER_ID, nil, "Sitemap", "sitemap-xml", &sectionKind)
	createPageForTest(t, w, SYSTEM_USER_ID, &section.ID, "Robots inside", "robots", pageNodeKind())

	sitemap := serveGET(router, "/sitemap.xml", nil)
	if sitemap.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d", sitemap.Code)
	}
	for _, want := range []string{"https://wiki.example.com/robots-txt", "https://wiki.example.com/sitemap-xml", "https://wiki.example.com/sitemap-xml/robots"} {
		if !strings.Contains(sitemap.Body.String(), "<loc>"+want+"</loc>") {
			t.Errorf("sitemap misses %s:\n%s", want, sitemap.Body.String())
		}
	}
	if robots := serveGET(router, "/robots.txt", nil); robots.Body.String() != "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/sitemap.xml\n" {
		t.Errorf("robots.txt = %q", robots.Body.String())
	}
}

// Deeply nested sections are listed with their full route path, parents first.
func TestWiki_Sitemap_DeeplyNestedSections(t *testing.T) {
	w, public, router := newSEOTestRouterWithPublicURL(t, "", "https://wiki.example.com")
	setPublicAccess(t, public, true)
	sectionKind := tree.NodeKindSection
	var parent *string
	path := ""
	for i := range 25 {
		slug := fmt.Sprintf("level-%d", i)
		node := createPageForTest(t, w, SYSTEM_USER_ID, parent, slug, slug, &sectionKind)
		parent = &node.ID
		path += "/" + slug
	}
	createPageForTest(t, w, SYSTEM_USER_ID, parent, "Leaf", "leaf", pageNodeKind())

	rec := serveGET(router, "/sitemap.xml", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	deepest := strings.Index(body, "<loc>https://wiki.example.com"+path+"/leaf</loc>")
	parentAt := strings.Index(body, "<loc>https://wiki.example.com"+path+"</loc>")
	if deepest < 0 || parentAt < 0 || parentAt > deepest {
		t.Errorf("nested routes missing or out of order (parent %d, leaf %d):\n%s", parentAt, deepest, body)
	}
	assertWellFormedSitemap(t, body)
}

// Run with -race: crawler fetches while editors create pages must neither race
// nor produce a broken document.
func TestWiki_Sitemap_ConcurrentWithPageCreation(t *testing.T) {
	w, public, router := newSEOTestRouterWithPublicURL(t, "", "https://wiki.example.com")
	setPublicAccess(t, public, true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 30 {
			slug := fmt.Sprintf("page-%d", i)
			createPageForTest(t, w, SYSTEM_USER_ID, nil, slug, slug, pageNodeKind())
		}
	}()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 15 {
				rec := serveGET(router, "/sitemap.xml", nil)
				if rec.Code != http.StatusOK {
					t.Errorf("status = %d", rec.Code)
					return
				}
				assertWellFormedSitemap(t, rec.Body.String())
			}
		}()
	}
	wg.Wait()
	<-done

	if n := strings.Count(serveGET(router, "/sitemap.xml", nil).Body.String(), "<loc>"); n != 31 {
		t.Errorf("final sitemap has %d entries, want 31 (welcome page + 30)", n)
	}
}

// Host and X-Forwarded-* never reach the sitemap or robots.txt on the real router.
func TestWiki_RobotsAndSitemap_IgnoreRequestHostHeaders(t *testing.T) {
	_, public, router := newSEOTestRouterWithPublicURL(t, "/wiki", "https://wiki.example.com/wiki")
	setPublicAccess(t, public, true)
	headers := map[string]string{"Host": "evil.example", "X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http", "X-Forwarded-Prefix": "/evil"}

	for _, p := range []string{"/wiki/robots.txt", "/wiki/sitemap.xml"} {
		rec := serveGET(router, p, headers)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "evil") {
			t.Errorf("%s uses a request header:\n%s", p, rec.Body.String())
		}
	}
}

// assertWellFormedSitemap parses the document strictly against the
// sitemaps.org namespace and checks every lastmod is RFC 3339.
func assertWellFormedSitemap(t *testing.T, doc string) {
	t.Helper()
	if !strings.HasPrefix(doc, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("sitemap does not start with the XML declaration: %.80q", doc)
	}
	var set struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []struct {
			Loc     string `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 loc"`
			LastMod string `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 lastmod"`
		} `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 url"`
	}
	dec := xml.NewDecoder(strings.NewReader(doc))
	dec.Strict = true
	if err := dec.Decode(&set); err != nil {
		t.Fatalf("sitemap is not well-formed: %v\n%s", err, doc)
	}
	for _, u := range set.URLs {
		if !strings.HasPrefix(u.Loc, "https://wiki.example.com/") {
			t.Errorf("loc %q is not under the public url", u.Loc)
		}
		if u.LastMod != "" {
			if _, err := time.Parse(time.RFC3339, u.LastMod); err != nil {
				t.Errorf("lastmod %q is not RFC 3339: %v", u.LastMod, err)
			}
		}
	}
}

func serveMarkdown(router http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Accept", "text/markdown")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestWiki_PageMarkdown_PublicWiki_ServesBodyWithoutFrontmatter(t *testing.T) {
	w, public, router := newSEOTestRouter(t, "/wiki")
	setPublicAccess(t, public, true)
	body := "# Setup\n\nInstall it."
	createDescribedPage(t, w, "Setup", "setup", body, "How to set it up.")
	onDisk, err := os.ReadFile(filepath.Join(w.GetStorageDir(), "root", "setup.md"))
	if err != nil || !strings.HasPrefix(string(onDisk), "---") || !strings.Contains(string(onDisk), "How to set it up.") {
		t.Fatalf("setup.md = %q, %v; want frontmatter with the description on disk", onDisk, err)
	}

	get := serveMarkdown(router, http.MethodGet, "/wiki/setup")
	head := serveMarkdown(router, http.MethodHead, "/wiki/setup")

	if get.Code != http.StatusOK || get.Body.String() != body {
		t.Fatalf("GET = %d %q, want 200 %q", get.Code, get.Body.String(), body)
	}
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD = %d %q, want 200 without body", head.Code, head.Body.String())
	}
	for _, h := range []string{"Content-Type", "X-Content-Type-Options", "Content-Security-Policy", "Cache-Control", "Vary", "Content-Length"} {
		if got, want := head.Header().Values(h), get.Header().Values(h); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: HEAD %q, GET %q", h, got, want)
		}
	}
	if got := get.Header().Get("Content-Type"); got != "text/markdown; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := get.Header().Get("Content-Security-Policy"); got != "sandbox" {
		t.Errorf("Content-Security-Policy = %q, want sandbox", got)
	}
	if got := get.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept" {
		t.Errorf("Vary = %q, want [Accept]", got)
	}
}

func TestWiki_PageMarkdown_NotAPageOrPrivate_ServesNoMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		public   bool
		path     string
		wantVary bool
	}{
		{name: "unknown page declines with Vary", public: true, path: "/wiki/missing", wantVary: true},
		{name: "SPA route declines with Vary", public: true, path: "/wiki/settings/branding", wantVary: true},
		{name: "permalink route is not resolved", public: true, path: "/wiki/p/someid", wantVary: true},
		{name: "private wiki: no Vary", public: false, path: "/wiki/welcome-to-leafwiki", wantVary: false},
	}
	_, public, router := newSEOTestRouter(t, "/wiki")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setPublicAccess(t, public, tc.public)

			rec := serveMarkdown(router, http.MethodGet, tc.path)

			if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/markdown") {
				t.Fatalf("got markdown %q, want the shell", rec.Body.String())
			}
			if got := rec.Header().Get("Vary") == "Accept"; got != tc.wantVary {
				t.Errorf("Vary = %q, want Accept: %v", rec.Header().Values("Vary"), tc.wantVary)
			}
		})
	}
}

func TestWiki_PageMarkdown_Root_StillRedirects(t *testing.T) {
	_, public, router := newSEOTestRouter(t, "/wiki")
	setPublicAccess(t, public, true)

	rec := serveMarkdown(router, http.MethodGet, "/wiki/")

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/wiki/welcome-to-leafwiki" {
		t.Fatalf("status = %d, Location = %q; want 302 to the first page", rec.Code, rec.Header().Get("Location"))
	}
}

func TestWiki_PageMarkdown_FollowsPublicAccessAtRuntime(t *testing.T) {
	_, public, router := newSEOTestRouter(t, "")
	isMarkdown := func() bool {
		return serveMarkdown(router, http.MethodGet, "/welcome-to-leafwiki").Header().Get("Content-Type") == "text/markdown; charset=utf-8"
	}

	if isMarkdown() {
		t.Fatal("public access off: got markdown")
	}
	setPublicAccess(t, public, true)
	if !isMarkdown() {
		t.Fatal("after enabling public access: no markdown")
	}
	setPublicAccess(t, public, false)
	if isMarkdown() {
		t.Fatal("after disabling public access: got markdown")
	}
}

// useEmbeddedFrontend serves the real shell, so tests can tell the shell
// from the Markdown variant; it skips without a built frontend.
func useEmbeddedFrontend(t *testing.T) {
	t.Helper()
	old := httpinternal.EmbedFrontend
	httpinternal.EmbedFrontend = "true"
	t.Cleanup(func() { httpinternal.EmbedFrontend = old })
	_, _, router := newSEOTestRouter(t, "")
	if rec := serveGET(router, "/welcome-to-leafwiki", nil); !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Skip("requires built frontend (make ui)")
	}
}

func serveAccept(router http.Handler, method, path, accept string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// The Markdown variant on the real router: GET and HEAD agree on every
// header, the body is the page content only (no shell, no meta tags) and
// Vary: Accept appears exactly once.
func TestWiki_PageMarkdown_MarkdownRequested_GetAndHeadServePageBody(t *testing.T) {
	useEmbeddedFrontend(t)
	type page struct{ title, slug, body string }
	tests := []struct {
		name     string
		basePath string
		pages    []page
		section  *page // created as a section, pages go below it
		path     string
		headers  map[string]string
		want     string
	}{
		{name: "page without base path", pages: []page{{"Setup", "setup", "# Setup\n\nInstall it."}}, path: "/setup", want: "# Setup\n\nInstall it."},
		{name: "page with base path", basePath: "/wiki", pages: []page{{"Setup", "setup", "x"}}, path: "/wiki/setup", want: "x"},
		{name: "nested base path", basePath: "/a/b", pages: []page{{"Setup", "setup", "x"}}, path: "/a/b/setup", want: "x"},
		{name: "empty body", pages: []page{{"Empty", "empty", ""}}, path: "/empty", want: ""},
		{name: "CRLF body is served with the LF endings LeafWiki stores", pages: []page{{"Crlf", "crlf", "a\r\nb\r\n"}}, path: "/crlf", want: "a\nb\n"},
		{name: "unicode body", pages: []page{{"Grüße", "gruesse", "# Grüße 🌿\n\n日本語"}}, path: "/gruesse", want: "# Grüße 🌿\n\n日本語"},
		{name: "underscore slug", pages: []page{{"Snake", "snake_case", "s"}}, path: "/snake_case", want: "s"},
		{name: "percent-encoded path resolves like the decoded one", pages: []page{{"Setup", "setup", "x"}}, path: "/set%75p", want: "x"},
		{name: "shell placeholders in the body stay raw", pages: []page{{"Raw", "raw", "{{__BODY__}}</head><script>x</script>"}}, path: "/raw", want: "{{__BODY__}}</head><script>x</script>"},
		{name: "section with children serves its own content", section: &page{"Docs", "docs", "# Docs index"}, pages: []page{{"Setup", "setup", "child"}}, path: "/docs", want: "# Docs index"},
		{name: "child of a section", section: &page{"Docs", "docs", "# Docs index"}, pages: []page{{"Setup", "setup", "child"}}, path: "/docs/setup", want: "child"},
		{name: "forwarded headers have no effect", pages: []page{{"Setup", "setup", "x"}}, path: "/setup", headers: map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Prefix": "/evil"}, want: "x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, public, router := newSEOTestRouter(t, tc.basePath)
			setPublicAccess(t, public, true)
			var parentID *string
			if tc.section != nil {
				sectionKind := tree.NodeKindSection
				s := createPageForTest(t, w, SYSTEM_USER_ID, nil, tc.section.title, tc.section.slug, &sectionKind)
				updatePageForTest(t, w, SYSTEM_USER_ID, s.ID, tc.section.title, tc.section.slug, &tc.section.body, &sectionKind)
				parentID = &s.ID
			}
			for _, p := range tc.pages {
				created := createPageForTest(t, w, SYSTEM_USER_ID, parentID, p.title, p.slug, pageNodeKind())
				body := p.body
				updatePageForTest(t, w, SYSTEM_USER_ID, created.ID, p.title, p.slug, &body, pageNodeKind())
			}

			get := serveAccept(router, http.MethodGet, tc.path, "text/markdown", tc.headers)
			head := serveAccept(router, http.MethodHead, tc.path, "text/markdown", tc.headers)

			if get.Code != http.StatusOK || get.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
				t.Fatalf("GET = %d %q, want 200 text/markdown", get.Code, get.Header().Get("Content-Type"))
			}
			if got := get.Body.String(); got != tc.want {
				t.Fatalf("body = %q, want %q", got, tc.want)
			}
			for _, leak := range []string{"<!doctype", "<html", "<meta", "og:title", "{{__PAGE_TITLE__}}"} {
				if !strings.Contains(tc.want, leak) && strings.Contains(strings.ToLower(get.Body.String()), strings.ToLower(leak)) {
					t.Errorf("markdown body contains shell markup %q", leak)
				}
			}
			if got := get.Header().Get("Content-Length"); got != strconv.Itoa(len(tc.want)) {
				t.Errorf("Content-Length = %q, want %d", got, len(tc.want))
			}
			if got := get.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept" {
				t.Errorf("Vary = %q, want [Accept]", got)
			}
			if head.Code != get.Code || head.Body.Len() != 0 {
				t.Errorf("HEAD = %d with %d body bytes, want %d and no body", head.Code, head.Body.Len(), get.Code)
			}
			for _, h := range []string{"Content-Type", "Content-Length", "X-Content-Type-Options", "Cache-Control", "Vary"} {
				if got, want := head.Header().Values(h), get.Header().Values(h); strings.Join(got, ",") != strings.Join(want, ",") {
					t.Errorf("%s: HEAD %q, GET %q", h, got, want)
				}
			}
		})
	}
}

// Requests that must get the shell (or no SPA answer at all) even though
// they ask for Markdown; none may be a 5xx or a Markdown body.
func TestWiki_PageMarkdown_NotAPageOrNotPreferred_ServesTheShell(t *testing.T) {
	useEmbeddedFrontend(t)
	tests := []struct {
		name      string
		basePath  string
		public    bool
		path      string
		accept    string
		wantShell bool // 200 text/html; otherwise only "not markdown"
		wantVary  bool
	}{
		{name: "browser Accept on a page", public: true, path: "/wiki/welcome-to-leafwiki", accept: "text/html,application/xhtml+xml,*/*;q=0.8", wantShell: true, wantVary: true},
		{name: "html weighted higher", public: true, path: "/wiki/welcome-to-leafwiki", accept: "text/html,text/markdown;q=0.9", wantShell: true, wantVary: true},
		{name: "no Accept header", public: true, path: "/wiki/welcome-to-leafwiki", wantShell: true, wantVary: true},
		{name: "trailing slash on a page", public: true, path: "/wiki/welcome-to-leafwiki/", accept: "text/markdown", wantShell: true, wantVary: true},
		{name: "double slash", public: true, path: "/wiki//welcome-to-leafwiki", accept: "text/markdown"},
		{name: "dot-dot segment", public: true, path: "/wiki/x/../welcome-to-leafwiki", accept: "text/markdown"},
		{name: "percent-encoded dot-dot", public: true, path: "/wiki/%2e%2e/welcome-to-leafwiki", accept: "text/markdown"},
		{name: "backslash", public: true, path: `/wiki/x\welcome-to-leafwiki`, accept: "text/markdown"},
		{name: "upper-case slug is a different route", public: true, path: "/wiki/Welcome-To-Leafwiki", accept: "text/markdown", wantShell: true, wantVary: true},
		{name: "outside the base path", public: true, path: "/welcome-to-leafwiki", accept: "text/markdown"},
		{name: "base path prefix without separator", public: true, path: "/wikiwelcome-to-leafwiki", accept: "text/markdown"},
		{name: "login route", public: true, path: "/wiki/login", accept: "text/markdown", wantShell: true, wantVary: true},
		{name: "private wiki", public: false, path: "/wiki/welcome-to-leafwiki", accept: "text/markdown", wantShell: true},
		{name: "private wiki, unknown route", public: false, path: "/wiki/missing", accept: "text/markdown", wantShell: true},
	}
	_, public, router := newSEOTestRouter(t, "/wiki")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setPublicAccess(t, public, tc.public)

			rec := serveAccept(router, http.MethodGet, tc.path, tc.accept, nil)

			if rec.Code >= 500 {
				t.Fatalf("status = %d", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "text/markdown") {
				t.Fatalf("got markdown (%d %q), want the shell", rec.Code, rec.Body.String())
			}
			if tc.wantShell && (rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html")) {
				t.Fatalf("got %d %q, want the 200 shell", rec.Code, rec.Header().Get("Content-Type"))
			}
			if !tc.wantShell {
				return
			}
			vary := rec.Header().Values("Vary")
			if tc.wantVary && (len(vary) != 1 || vary[0] != "Accept") {
				t.Errorf("Vary = %q, want [Accept]", vary)
			}
			if !tc.wantVary && len(vary) != 0 {
				t.Errorf("Vary = %q, want none", vary)
			}
		})
	}
}

// An empty wiki: "/" has no redirect target and is no page, so a Markdown
// client gets the shell.
func TestWiki_PageMarkdown_EmptyWikiRoot_ServesTheShell(t *testing.T) {
	useEmbeddedFrontend(t)
	w, public, router := newSEOTestRouter(t, "")
	setPublicAccess(t, public, true)
	deletePageForTest(t, w, SYSTEM_USER_ID, w.tree.GetTree().Children[0].ID, true)

	rec := serveAccept(router, http.MethodGet, "/", "text/markdown", nil)

	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("got %d %q, want the 200 shell", rec.Code, rec.Header().Get("Content-Type"))
	}
}
