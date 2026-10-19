package http_test

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	wikihttp "github.com/perber/wiki/internal/http"
)

// shellTestTemplate mirrors the placeholders and the relative asset reference
// of the built ui/leafwiki-ui/index.html, so these tests don't need dist/.
const shellTestTemplate = `<head><link rel="icon" href="{{__FAVICON_HREF__}}"><meta name="base-path" content="{{__BASE_PATH__}}"><title>{{__SITE_NAME__}}</title><script src="./static/a.js"></script></head><body><div id="root"></div></body>`

func siteName(name string) func() string { return func() string { return name } }

func TestRenderShell_Characterization(t *testing.T) {
	tests := []struct {
		name     string
		template string
		cfg      wikihttp.FrontendConfig
		opts     wikihttp.RouterOptions
		css      string
		want     string
	}{
		{
			name:     "defaults without config",
			template: shellTestTemplate,
			want:     `<head><link rel="icon" href="/favicon.svg"><meta name="base-path" content=""><title>LeafWiki</title><script src="/static/a.js"></script></head><body><div id="root"></div></body>`,
		},
		{
			name:     "empty site name falls back to LeafWiki",
			template: shellTestTemplate,
			cfg:      wikihttp.FrontendConfig{GetSiteName: siteName("")},
			want:     `<head><link rel="icon" href="/favicon.svg"><meta name="base-path" content=""><title>LeafWiki</title><script src="/static/a.js"></script></head><body><div id="root"></div></body>`,
		},
		{
			name:     "site name is HTML-escaped",
			template: shellTestTemplate,
			cfg:      wikihttp.FrontendConfig{GetSiteName: siteName(`A <b> & "c"`)},
			want:     `<head><link rel="icon" href="/favicon.svg"><meta name="base-path" content=""><title>A &lt;b&gt; &amp; &#34;c&#34;</title><script src="/static/a.js"></script></head><body><div id="root"></div></body>`,
		},
		{
			name:     "base path and branding favicon",
			template: shellTestTemplate,
			cfg:      wikihttp.FrontendConfig{GetFaviconFile: func() string { return "f.png" }},
			opts:     wikihttp.RouterOptions{BasePath: "/wiki"},
			want:     `<head><link rel="icon" href="/wiki/branding/f.png"><meta name="base-path" content="/wiki"><title>LeafWiki</title><script src="/wiki/static/a.js"></script></head><body><div id="root"></div></body>`,
		},
		{
			// The stylesheet tag inserts the base path unescaped; known and
			// tracked separately, pinned here so this refactor doesn't change it.
			name:     "base path is escaped except in the custom stylesheet tag",
			template: shellTestTemplate,
			opts:     wikihttp.RouterOptions{BasePath: `/w"x`},
			css:      "/data/custom.css",
			want:     `<head><link rel="icon" href="/w&#34;x/favicon.svg"><meta name="base-path" content="/w&#34;x"><title>LeafWiki</title><script src="/w&#34;x/static/a.js"></script>  <link rel="stylesheet" href="/w"x/custom.css">` + "\n" + `  </head><body><div id="root"></div></body>`,
		},
		{
			name:     "custom stylesheet before injected header code",
			template: shellTestTemplate,
			opts:     wikihttp.RouterOptions{BasePath: "/wiki", InjectCodeInHeader: `<script>x()</script>`},
			css:      "/data/custom.css",
			want:     `<head><link rel="icon" href="/wiki/favicon.svg"><meta name="base-path" content="/wiki"><title>LeafWiki</title><script src="/wiki/static/a.js"></script>  <link rel="stylesheet" href="/wiki/custom.css">` + "\n" + `    <script>x()</script>` + "\n" + `  </head><body><div id="root"></div></body>`,
		},
		{
			name:     "injected header code without stylesheet",
			template: shellTestTemplate,
			opts:     wikihttp.RouterOptions{InjectCodeInHeader: `<meta name="x">`},
			want:     `<head><link rel="icon" href="/favicon.svg"><meta name="base-path" content=""><title>LeafWiki</title><script src="/static/a.js"></script>  <meta name="x">` + "\n" + `  </head><body><div id="root"></div></body>`,
		},
		{
			name:     "whitespace-only header code is ignored",
			template: shellTestTemplate,
			opts:     wikihttp.RouterOptions{InjectCodeInHeader: " \n "},
			want:     `<head><link rel="icon" href="/favicon.svg"><meta name="base-path" content=""><title>LeafWiki</title><script src="/static/a.js"></script></head><body><div id="root"></div></body>`,
		},
		{
			name:     "template without head end tag gets no injection",
			template: `<title>{{__SITE_NAME__}}</title>`,
			opts:     wikihttp.RouterOptions{InjectCodeInHeader: `<meta name="x">`},
			css:      "/data/custom.css",
			want:     `<title>LeafWiki</title>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wikihttp.RenderShell(tt.template, tt.cfg, tt.opts, tt.css, "/deep/page")
			if got != tt.want {
				t.Fatalf("render mismatch\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func shellRouter(t *testing.T, embed bool, cfg wikihttp.FrontendConfig, basePath string) *gin.Engine {
	t.Helper()
	old := wikihttp.EmbedFrontend
	wikihttp.EmbedFrontend = "false"
	if embed {
		wikihttp.EmbedFrontend = "true"
	}
	t.Cleanup(func() { wikihttp.EmbedFrontend = old })
	return wikihttp.NewRouter(nil, cfg, wikihttp.RouterOptions{BasePath: basePath, DisableRequestLog: true})
}

// Requests the SPA fallback never answers with the shell; independent of
// whether dist/index.html was built.
func TestSPAFallback_NonShellRequests(t *testing.T) {
	tests := []struct {
		name     string
		embed    bool
		method   string
		path     string
		wantBody string
	}{
		{name: "api path", embed: true, method: http.MethodGet, path: "/wiki/api/unknown", wantBody: "Page not found"},
		{name: "assets path", embed: true, method: http.MethodGet, path: "/wiki/assets/x.png", wantBody: "Page not found"},
		{name: "branding path", embed: true, method: http.MethodGet, path: "/wiki/branding/x.png", wantBody: "Page not found"},
		{name: "POST", embed: true, method: http.MethodPost, path: "/wiki/page", wantBody: "Page not found"},
		{name: "HEAD", embed: true, method: http.MethodHead, path: "/wiki/page", wantBody: "Page not found"},
		{name: "outside base path", embed: true, method: http.MethodGet, path: "/other/page", wantBody: "Page not found"},
		{name: "frontend not embedded", embed: false, method: http.MethodGet, path: "/wiki/page", wantBody: "404 page not found"},
		{name: "frontend not embedded, outside base path", embed: false, method: http.MethodGet, path: "/other", wantBody: "404 page not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := shellRouter(t, tt.embed, wikihttp.FrontendConfig{}, "/wiki")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", rec.Code)
			}
			if rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

const seamTestTemplate = `<head><title>{{__PAGE_TITLE__}}</title></head><body><div id="root">{{__BODY__}}</div></body>`

func parts(p wikihttp.ShellParts) wikihttp.ShellContributor {
	return wikihttp.ShellContributorFunc(func(context.Context, wikihttp.ShellRequest) wikihttp.ShellParts { return p })
}

func TestRenderShell_Contributors(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		siteName     string
		contributors []wikihttp.ShellContributor
		opts         wikihttp.RouterOptions
		css          string
		want         string
	}{
		{
			name:     "no contributors: site name title, empty root",
			template: seamTestTemplate,
			siteName: "My Wiki",
			want:     `<head><title>My Wiki</title></head><body><div id="root"></div></body>`,
		},
		{
			name:         "empty title falls back to site name",
			template:     seamTestTemplate,
			siteName:     "My Wiki",
			contributors: []wikihttp.ShellContributor{parts(wikihttp.ShellParts{Body: "<p>x</p>"})},
			want:         `<head><title>My Wiki</title></head><body><div id="root"><p>x</p></div></body>`,
		},
		{
			name:     "first non-empty title wins",
			template: seamTestTemplate,
			contributors: []wikihttp.ShellContributor{
				parts(wikihttp.ShellParts{}),
				parts(wikihttp.ShellParts{Title: "First"}),
				parts(wikihttp.ShellParts{Title: "Second"}),
			},
			want: `<head><title>First</title></head><body><div id="root"></div></body>`,
		},
		{
			name:         "title is escaped and placeholders in it are not expanded",
			template:     `<head><title>{{__PAGE_TITLE__}}</title><meta content="{{__BASE_PATH__}}"></head><body><div id="root">{{__BODY__}}</div></body>`,
			opts:         wikihttp.RouterOptions{BasePath: "/wiki"},
			contributors: []wikihttp.ShellContributor{parts(wikihttp.ShellParts{Title: `<b>{{__BASE_PATH__}}"./static/`})},
			want:         `<head><title>&lt;b&gt;{{__BASE_PATH__}}&#34;./static/</title><meta content="/wiki"></head><body><div id="root"></div></body>`,
		},
		{
			name:     "body is concatenated in order and not re-scanned",
			template: seamTestTemplate,
			contributors: []wikihttp.ShellContributor{
				parts(wikihttp.ShellParts{Body: `<p>{{__PAGE_TITLE__}}</p>`}),
				parts(wikihttp.ShellParts{Body: `<a href="./static/x">x</a>`}),
			},
			want: `<head><title>LeafWiki</title></head><body><div id="root"><p>{{__PAGE_TITLE__}}</p><a href="./static/x">x</a></div></body>`,
		},
		{
			name:     "head in registration order, before stylesheet and header code",
			template: seamTestTemplate,
			opts:     wikihttp.RouterOptions{InjectCodeInHeader: `<script>x()</script>`},
			css:      "/data/custom.css",
			contributors: []wikihttp.ShellContributor{
				parts(wikihttp.ShellParts{Head: `<meta name="a">`}),
				parts(wikihttp.ShellParts{Head: `<meta name="b">`}),
			},
			want: `<head><title>LeafWiki</title>  <meta name="a">` + "\n" +
				`    <meta name="b">` + "\n" +
				`    <link rel="stylesheet" href="/custom.css">` + "\n" +
				`    <script>x()</script>` + "\n" +
				`  </head><body><div id="root"></div></body>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := wikihttp.FrontendConfig{GetSiteName: siteName(tt.siteName), ShellContributors: tt.contributors}
			got := wikihttp.RenderShell(tt.template, cfg, tt.opts, tt.css, "/deep/page")
			if got != tt.want {
				t.Fatalf("render mismatch\n got: %s\nwant: %s", got, tt.want)
			}
		})
	}
}

func TestRenderShell_ContributorsSeeStrippedRoutePath(t *testing.T) {
	var got wikihttp.ShellRequest
	cfg := wikihttp.FrontendConfig{ShellContributors: []wikihttp.ShellContributor{
		wikihttp.ShellContributorFunc(func(_ context.Context, req wikihttp.ShellRequest) wikihttp.ShellParts {
			got = req
			return wikihttp.ShellParts{}
		}),
	}}
	wikihttp.RenderShell(seamTestTemplate, cfg, wikihttp.RouterOptions{BasePath: "/wiki"}, "", "/wiki/deep/page")
	if got.RoutePath != "/deep/page" || got.BasePath != "/wiki" || got.Request == nil || got.Request.URL.Path != "/wiki/deep/page" {
		t.Fatalf("request = %+v", got)
	}
}

// recordingResponder records calls and always adds a Vary header (like a
// responder whose decision depends on a request header); it writes a response
// and reports handled only when handle is set.
type recordingResponder struct {
	handle bool
	header string
	calls  []wikihttp.ShellRequest
}

func (r *recordingResponder) RespondShell(w http.ResponseWriter, req wikihttp.ShellRequest) bool {
	r.calls = append(r.calls, req)
	w.Header().Add("Vary", r.header)
	if r.handle {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("handled by " + r.header))
	}
	return r.handle
}

func TestShellResponders_FirstHandledWins(t *testing.T) {
	for _, embed := range []bool{true, false} {
		t.Run(fmt.Sprintf("embed=%v", embed), func(t *testing.T) {
			declining := &recordingResponder{header: "X-A"}
			handling := &recordingResponder{handle: true, header: "X-B"}
			later := &recordingResponder{handle: true, header: "X-C"}
			cfg := wikihttp.FrontendConfig{ShellResponders: []wikihttp.ShellResponder{declining, handling, later}}
			router := shellRouter(t, embed, cfg, "/wiki")

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/wiki/deep/page", nil))

			if rec.Code != http.StatusTeapot || rec.Body.String() != "handled by X-B" {
				t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
			}
			if vary := rec.Header().Values("Vary"); len(vary) != 2 || vary[0] != "X-A" || vary[1] != "X-B" {
				t.Fatalf("Vary = %v, want the declining responder's header kept", vary)
			}
			if len(later.calls) != 0 {
				t.Fatalf("responder after the handling one was called %d times", len(later.calls))
			}
			if req := handling.calls[0]; req.RoutePath != "/deep/page" || req.BasePath != "/wiki" || req.Request.URL.Path != "/wiki/deep/page" {
				t.Fatalf("request = %+v", req)
			}
		})
	}
}

func TestShellResponders_WriteWithoutWriteHeaderIs200(t *testing.T) {
	responder := wikihttp.ShellResponderFunc(func(w http.ResponseWriter, _ wikihttp.ShellRequest) bool {
		_, _ = w.Write([]byte("ok"))
		return true
	})
	router := shellRouter(t, false, wikihttp.FrontendConfig{ShellResponders: []wikihttp.ShellResponder{responder}}, "")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/page", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestShellResponders_BasePathRootIsSlash(t *testing.T) {
	for _, path := range []string{"/wiki", "/wiki/"} {
		t.Run(path, func(t *testing.T) {
			responder := &recordingResponder{handle: true, header: "X"}
			router := shellRouter(t, true, wikihttp.FrontendConfig{ShellResponders: []wikihttp.ShellResponder{responder}}, "/wiki")
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
			if len(responder.calls) != 1 || responder.calls[0].RoutePath != "/" {
				t.Fatalf("calls = %+v", responder.calls)
			}
		})
	}
}

func TestShellResponders_NotCalledForNonShellRequests(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "api", method: http.MethodGet, path: "/wiki/api/unknown"},
		{name: "static", method: http.MethodGet, path: "/wiki/static/missing.js"},
		{name: "assets", method: http.MethodGet, path: "/wiki/assets/x.png"},
		{name: "branding", method: http.MethodGet, path: "/wiki/branding/x.png"},
		{name: "POST", method: http.MethodPost, path: "/wiki/page"},
		{name: "HEAD", method: http.MethodHead, path: "/wiki/page"},
		{name: "outside base path", method: http.MethodGet, path: "/other/page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responder := &recordingResponder{handle: true, header: "X"}
			router := shellRouter(t, true, wikihttp.FrontendConfig{ShellResponders: []wikihttp.ShellResponder{responder}}, "/wiki")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if len(responder.calls) != 0 {
				t.Fatalf("responder called for %s %s", tt.method, tt.path)
			}
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d", rec.Code)
			}
		})
	}
}

func TestShellTagHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  template.HTML
		want template.HTML
	}{
		{name: "meta tag", got: wikihttp.MetaTag("description", "plain"), want: `<meta name="description" content="plain">`},
		{name: "meta tag escapes", got: wikihttp.MetaTag(`a"b`, `"><script>x</script>&`), want: `<meta name="a&#34;b" content="&#34;&gt;&lt;script&gt;x&lt;/script&gt;&amp;">`},
		{name: "meta property", got: wikihttp.MetaProperty("og:title", `T & "q"`), want: `<meta property="og:title" content="T &amp; &#34;q&#34;">`},
		{name: "link tag", got: wikihttp.LinkTag("canonical", "https://x.test/a?b=1&c=2"), want: `<link rel="canonical" href="https://x.test/a?b=1&amp;c=2">`},
		{name: "link tag escapes", got: wikihttp.LinkTag(`x"`, `"><b>`), want: `<link rel="x&#34;" href="&#34;&gt;&lt;b&gt;">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %s, want %s", tt.got, tt.want)
			}
		})
	}
}
