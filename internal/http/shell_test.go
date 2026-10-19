package http_test

import (
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
