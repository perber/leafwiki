package seo_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	wikihttp "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/wiki/seo"
)

// routesFixture is the seo routes on a bare gin engine.
type routesFixture struct {
	engine *gin.Engine
	lister *fakePageRouteLister
	logs   *bytes.Buffer
}

func newRoutesFixture(t *testing.T, public bool, publicURL, basePath string, lister *fakePageRouteLister) routesFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if lister == nil {
		lister = routesAt("docs")
	}
	logs := &bytes.Buffer{}
	routes := seo.NewRoutes(seo.RoutesConfig{
		Public:  seo.NewPublicOnly(publicaccess.Fixed(public)),
		Robots:  seo.NewGetRobotsUseCase(publicURL),
		Sitemap: seo.NewBuildSitemapUseCase(lister, publicURL),
		Log:     slog.New(slog.NewTextHandler(logs, nil)),
	})
	engine := gin.New()
	routes.RegisterRoutes(wikihttp.RouterContext{
		Engine: engine,
		Base:   engine.Group(basePath),
		Opts:   wikihttp.RouterOptions{BasePath: basePath},
	})
	return routesFixture{engine: engine, lister: lister, logs: logs}
}

func (f routesFixture) serve(method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.engine.ServeHTTP(rec, req)
	return rec
}

func TestRoutes_Robots(t *testing.T) {
	tests := []struct {
		name      string
		public    bool
		publicURL string
		basePath  string
		headers   map[string]string
		want      string
	}{
		{name: "private wiki disallows everything", publicURL: "https://wiki.example.com", want: "User-agent: *\nDisallow: /\n"},
		{name: "public wiki without public url", public: true, want: "User-agent: *\nAllow: /\n"},
		{
			name:      "public wiki with public url",
			public:    true,
			publicURL: "https://wiki.example.com",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/sitemap.xml\n",
		},
		{
			name:      "under a base path",
			public:    true,
			publicURL: "https://wiki.example.com/wiki",
			basePath:  "/wiki",
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/wiki/sitemap.xml\n",
		},
		{
			name:      "forwarded host and proto have no effect",
			public:    true,
			publicURL: "https://wiki.example.com",
			headers:   map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http", "Host": "evil.example"},
			want:      "User-agent: *\nAllow: /\n\nSitemap: https://wiki.example.com/sitemap.xml\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoutesFixture(t, tc.public, tc.publicURL, tc.basePath, nil)

			rec := f.serve(http.MethodGet, tc.basePath+"/robots.txt", tc.headers)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
			assertHeader(t, rec, "Content-Type", "text/plain; charset=utf-8")
			assertHeader(t, rec, "Cache-Control", "no-cache")
			if f.lister.reads != 0 {
				t.Errorf("reads = %d, want 0", f.lister.reads)
			}
		})
	}
}

func TestRoutes_Sitemap_PublicWiki_ServesSitemap(t *testing.T) {
	f := newRoutesFixture(t, true, "https://wiki.example.com/wiki", "/wiki", routesAt("docs", "docs/setup"))

	rec := f.serve(http.MethodGet, "/wiki/sitemap.xml", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertHeader(t, rec, "Content-Type", "application/xml; charset=utf-8")
	assertHeader(t, rec, "Cache-Control", "no-cache")
	got := sitemapLocs(t, rec.Body.String())
	if want := "https://wiki.example.com/wiki/docs https://wiki.example.com/wiki/docs/setup"; strings.Join(got, " ") != want {
		t.Errorf("locs = %v, want %s", got, want)
	}
}

func TestRoutes_Sitemap_NotAvailable_Returns404WithoutReads(t *testing.T) {
	tests := []struct {
		name      string
		public    bool
		publicURL string
	}{
		{name: "private wiki", publicURL: "https://wiki.example.com"},
		{name: "public wiki without public url", public: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoutesFixture(t, tc.public, tc.publicURL, "", nil)

			rec := f.serve(http.MethodGet, "/sitemap.xml", nil)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			assertHeader(t, rec, "Cache-Control", "no-cache")
			if f.lister.reads != 0 {
				t.Errorf("reads = %d, want 0", f.lister.reads)
			}
		})
	}
}

func TestRoutes_Sitemap_ReadError_Returns500WithoutDetails(t *testing.T) {
	f := newRoutesFixture(t, true, "https://wiki.example.com", "", &fakePageRouteLister{err: errors.New("disk on fire")})

	rec := f.serve(http.MethodGet, "/sitemap.xml", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "disk on fire") {
		t.Errorf("body leaks the error: %q", rec.Body.String())
	}
	if !strings.Contains(f.logs.String(), "disk on fire") {
		t.Errorf("error not logged; logs = %q", f.logs.String())
	}
}

// A crawler that hangs up mid-sitemap is not an operator problem.
func TestRoutes_Sitemap_ClientGone_LogsNoError(t *testing.T) {
	f := newRoutesFixture(t, true, "https://wiki.example.com", "", nil)

	f.engine.ServeHTTP(failingResponseWriter{httptest.NewRecorder()}, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))

	if strings.Contains(f.logs.String(), "level=ERROR") {
		t.Errorf("logs = %q, want no error", f.logs.String())
	}
}

type failingResponseWriter struct{ *httptest.ResponseRecorder }

func (failingResponseWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

func TestRoutes_Sitemap_Truncated_WarnsOnce(t *testing.T) {
	lister := routesAt("docs")
	lister.truncated = true
	f := newRoutesFixture(t, true, "https://wiki.example.com", "", lister)

	for range 2 {
		if rec := f.serve(http.MethodGet, "/sitemap.xml", nil); rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	}

	if n := strings.Count(f.logs.String(), "level=WARN"); n != 1 {
		t.Errorf("warnings = %d, want 1; logs = %q", n, f.logs.String())
	}
}

// HEAD answers like GET without a body, through a real server so net/http's
// own HEAD handling (Content-Length, dropped body) is part of the test.
func TestRoutes_Head_SameHeadersAsGet(t *testing.T) {
	tests := []struct {
		name      string
		public    bool
		publicURL string
		path      string
	}{
		{name: "robots.txt, public", public: true, publicURL: "https://wiki.example.com", path: "/robots.txt"},
		{name: "robots.txt, private", path: "/robots.txt"},
		{name: "sitemap.xml, public", public: true, publicURL: "https://wiki.example.com", path: "/sitemap.xml"},
		{name: "sitemap.xml, private", publicURL: "https://wiki.example.com", path: "/sitemap.xml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(newRoutesFixture(t, tc.public, tc.publicURL, "", nil).engine)
			t.Cleanup(server.Close)

			get := fetch(t, http.MethodGet, server.URL+tc.path)
			head := fetch(t, http.MethodHead, server.URL+tc.path)

			if head.status != get.status {
				t.Errorf("status HEAD=%d GET=%d", head.status, get.status)
			}
			for _, h := range []string{"Content-Type", "Content-Length", "Cache-Control"} {
				if got, want := head.header.Get(h), get.header.Get(h); got != want {
					t.Errorf("%s: HEAD %q, GET %q", h, got, want)
				}
			}
			if head.body != "" {
				t.Errorf("HEAD body = %q, want empty", head.body)
			}
			if get.body == "" {
				t.Error("GET body is empty")
			}
		})
	}
}

type fetched struct {
	status int
	header http.Header
	body   string
}

func fetch(t *testing.T, method, url string) fetched {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return fetched{status: resp.StatusCode, header: resp.Header, body: string(body)}
}

func assertHeader(t *testing.T, rec *httptest.ResponseRecorder, name, want string) {
	t.Helper()
	if got := rec.Header().Get(name); got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

// Absolute URLs come from --public-url only: Host and X-Forwarded-* headers
// must not reach the sitemap either.
func TestRoutes_Sitemap_IgnoresRequestHostHeaders(t *testing.T) {
	f := newRoutesFixture(t, true, "https://wiki.example.com", "", routesAt("docs"))

	rec := f.serve(http.MethodGet, "/sitemap.xml", map[string]string{
		"Host": "evil.example", "X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http", "Forwarded": "host=evil.example",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "evil.example") {
		t.Errorf("sitemap uses a request header:\n%s", rec.Body.String())
	}
	if got := sitemapLocs(t, rec.Body.String()); strings.Join(got, " ") != "https://wiki.example.com/docs" {
		t.Errorf("locs = %v", got)
	}
}

// Look-alike paths are not the seo routes: they must not answer with robots
// or sitemap content (and must not read pages), whatever public access is.
func TestRoutes_LookalikePaths_AreNotServed(t *testing.T) {
	paths := []string{"/Robots.txt", "/ROBOTS.TXT", "/Sitemap.xml", "/sitemap.XML", "/sitemap.xml.gz", "/sitemap.xml/x", "/robots.txt/x", "/wiki/sitemap.xml", "/./sitemap.xml/.."}
	for _, public := range []bool{false, true} {
		f := newRoutesFixture(t, public, "https://wiki.example.com", "", nil)
		for _, p := range paths {
			rec := f.serve(http.MethodGet, p, nil)
			if rec.Code >= 500 {
				t.Errorf("public=%v %s: status %d", public, p, rec.Code)
			}
			if body := rec.Body.String(); strings.Contains(body, "<urlset") || strings.Contains(body, "User-agent") {
				t.Errorf("public=%v %s: answered with seo content (%d):\n%s", public, p, rec.Code, body)
			}
		}
		if f.lister.reads != 0 {
			t.Errorf("public=%v: reads = %d, want 0", public, f.lister.reads)
		}
	}
}

// A trailing slash or a query string must not bypass the public-access gate.
func TestRoutes_PrivateWiki_SlashAndQueryVariants_LeakNoRoutes(t *testing.T) {
	f := newRoutesFixture(t, false, "https://wiki.example.com", "", nil)
	for _, p := range []string{"/sitemap.xml/", "/sitemap.xml?x=1", "/sitemap.xml?", "/robots.txt/", "/robots.txt?x=1"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			rec := f.serve(method, p, nil)
			if strings.Contains(rec.Body.String(), "<loc>") || strings.Contains(rec.Body.String(), "Allow: /\n") {
				t.Errorf("%s %s leaks (%d):\n%s", method, p, rec.Code, rec.Body.String())
			}
		}
	}
	if f.lister.reads != 0 {
		t.Errorf("reads = %d, want 0", f.lister.reads)
	}
}

// Methods other than GET and HEAD are not served.
func TestRoutes_OtherMethods_AreNotServed(t *testing.T) {
	f := newRoutesFixture(t, true, "https://wiki.example.com", "", nil)
	for _, p := range []string{"/robots.txt", "/sitemap.xml"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
			rec := f.serve(method, p, nil)
			if rec.Code == http.StatusOK {
				t.Errorf("%s %s = 200, want not served", method, p)
			}
		}
	}
	if f.lister.reads != 0 {
		t.Errorf("reads = %d, want 0", f.lister.reads)
	}
}

// The truncation warning is logged once even under concurrent fetches.
func TestRoutes_Sitemap_Truncated_ConcurrentFetches_WarnOnce(t *testing.T) {
	lister := &concurrentLister{truncated: true}
	logs := &lockedBuffer{}
	gin.SetMode(gin.TestMode)
	routes := seo.NewRoutes(seo.RoutesConfig{
		Public:  seo.NewPublicOnly(publicaccess.Fixed(true)),
		Robots:  seo.NewGetRobotsUseCase("https://wiki.example.com"),
		Sitemap: seo.NewBuildSitemapUseCase(lister, "https://wiki.example.com"),
		Log:     slog.New(slog.NewTextHandler(logs, nil)),
	})
	engine := gin.New()
	routes.RegisterRoutes(wikihttp.RouterContext{Engine: engine, Base: engine.Group(""), Opts: wikihttp.RouterOptions{}})

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d", rec.Code)
			}
		}()
	}
	wg.Wait()

	if n := strings.Count(logs.String(), "level=WARN"); n != 1 {
		t.Errorf("warnings = %d, want 1", n)
	}
}

type concurrentLister struct{ truncated bool }

func (l *concurrentLister) ListPageRoutes(context.Context, int) ([]seo.PageRoute, bool, error) {
	return []seo.PageRoute{{RoutePath: "docs"}}, l.truncated, nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
