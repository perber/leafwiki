package seo_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	wikihttp "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/wiki/seo"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func shellRequest(basePath, routePath string) wikihttp.ShellRequest {
	return wikihttp.ShellRequest{
		Request:   httptest.NewRequest(http.MethodGet, basePath+routePath, nil),
		RoutePath: routePath,
		BasePath:  basePath,
	}
}

func rootRedirect(reader *fakeFirstPageReader) wikihttp.ShellResponder {
	return seo.NewRootRedirect(seo.NewGetRootTargetUseCase(reader), discardLogger())
}

func TestPublicOnly_Responder_RunsOnlyWithPublicAccess(t *testing.T) {
	tests := []struct {
		name        string
		gate        publicaccess.ReadGate
		wantHandled bool
		wantReads   int
	}{
		{name: "public access off: zero reads", gate: publicaccess.Fixed(false), wantHandled: false, wantReads: 0},
		{name: "no gate counts as off", gate: nil, wantHandled: false, wantReads: 0},
		{name: "public access on", gate: publicaccess.Fixed(true), wantHandled: true, wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := firstPage("docs")
			responder := seo.NewPublicOnly(tc.gate).Responder(rootRedirect(reader))

			rec := httptest.NewRecorder()
			handled := responder.RespondShell(rec, shellRequest("", "/"))

			if handled != tc.wantHandled {
				t.Errorf("handled = %v, want %v", handled, tc.wantHandled)
			}
			if reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", reader.reads, tc.wantReads)
			}
			if !tc.wantHandled && (len(rec.Header()) != 0 || rec.Body.Len() != 0) {
				t.Errorf("declining responder wrote headers %v / body %q", rec.Header(), rec.Body.String())
			}
		})
	}
}

func TestPublicOnly_Handler_PicksHandlerByPublicAccess(t *testing.T) {
	tests := []struct {
		name string
		gate publicaccess.ReadGate
		want string
	}{
		{name: "public access off", gate: publicaccess.Fixed(false), want: "off"},
		{name: "no gate counts as off", gate: nil, want: "off"},
		{name: "public access on", gate: publicaccess.Fixed(true), want: "on"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ran []string
			handler := func(name string) http.HandlerFunc {
				return func(http.ResponseWriter, *http.Request) { ran = append(ran, name) }
			}

			seo.NewPublicOnly(tc.gate).Handler(handler("on"), handler("off")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/robots.txt", nil))

			if strings.Join(ran, ",") != tc.want {
				t.Errorf("ran %v, want only %s", ran, tc.want)
			}
		})
	}
}

func TestRootRedirect_Root_RedirectsToFirstPage(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		headers  map[string]string
		want     string
	}{
		{name: "no base path", want: "/docs"},
		{name: "base path", basePath: "/wiki", want: "/wiki/docs"},
		{
			name:     "forwarded host and proto have no effect",
			basePath: "/wiki",
			headers:  map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "https", "Host": "evil.example"},
			want:     "/wiki/docs",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := shellRequest(tc.basePath, "/")
			for k, v := range tc.headers {
				req.Request.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			if !rootRedirect(firstPage("docs")).RespondShell(rec, req) {
				t.Fatal("root request was not handled")
			}
			if rec.Code != http.StatusFound {
				t.Errorf("status = %d, want 302", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Errorf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRootRedirect_NoTarget_Declines(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakeFirstPageReader
		routePath string
		wantReads int
	}{
		{name: "not the root", reader: firstPage("docs"), routePath: "/docs", wantReads: 0},
		{name: "empty wiki", reader: &fakeFirstPageReader{}, routePath: "/", wantReads: 1},
		{name: "read error", reader: &fakeFirstPageReader{err: errors.New("boom")}, routePath: "/", wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			if rootRedirect(tc.reader).RespondShell(rec, shellRequest("/wiki", tc.routePath)) {
				t.Fatal("responder handled a request without a target")
			}
			if len(rec.Header()) != 0 || rec.Body.Len() != 0 {
				t.Errorf("declining responder wrote headers %v / body %q", rec.Header(), rec.Body.String())
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}

func pageMeta(reader *fakePageReader, publicURL string) wikihttp.ShellContributor {
	return permalinkMeta(reader, noPermalinks(), publicURL)
}

// permalinkMeta is pageMeta with a permalink resolver.
func permalinkMeta(reader *fakePageReader, permalinks *fakePermalinkResolver, publicURL string) wikihttp.ShellContributor {
	return seo.NewPageMeta(seo.NewGetPageMetaUseCase(reader, permalinks, staticSiteName("Team Wiki"), publicURL), discardLogger())
}

func TestPublicOnly_Contributor_RunsOnlyWithPublicAccess(t *testing.T) {
	tests := []struct {
		name      string
		gate      publicaccess.ReadGate
		wantTitle string
		wantReads int
	}{
		{name: "public access off: zero reads", gate: publicaccess.Fixed(false), wantReads: 0},
		{name: "no gate counts as off", gate: nil, wantReads: 0},
		{name: "public access on", gate: publicaccess.Fixed(true), wantTitle: "Setup - Team Wiki", wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := pageAt("docs/setup", seo.Page{Title: "Setup", Body: "Install it."})
			contributor := seo.NewPublicOnly(tc.gate).Contributor(pageMeta(reader, ""))

			got := contributor.ContributeShell(context.Background(), shellRequest("", "/docs/setup"))

			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if tc.wantReads == 0 && got != (wikihttp.ShellParts{}) {
				t.Errorf("declining contributor returned %+v, want empty parts", got)
			}
			if reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", reader.reads, tc.wantReads)
			}
		})
	}
}

func TestPageMeta_Page_AddsTitleMetaAndOpenGraphTags(t *testing.T) {
	tests := []struct {
		name      string
		page      seo.Page
		publicURL string
		basePath  string
		headers   map[string]string
		wantTitle string
		wantHead  string
	}{
		{
			name:      "without public url: no canonical, no og:url",
			page:      seo.Page{Title: "Setup", Body: "Install it."},
			wantTitle: "Setup - Team Wiki",
			wantHead: `<meta name="description" content="Install it.">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="Setup">` +
				`<meta property="og:description" content="Install it.">` +
				`<meta property="og:site_name" content="Team Wiki">`,
		},
		{
			name:      "with public url and base path: canonical and og:url",
			page:      seo.Page{Title: "Setup", Body: "Install it."},
			publicURL: "https://wiki.example.com/wiki",
			basePath:  "/wiki",
			wantTitle: "Setup - Team Wiki",
			wantHead: `<meta name="description" content="Install it.">` +
				`<link rel="canonical" href="https://wiki.example.com/wiki/docs/setup">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="Setup">` +
				`<meta property="og:description" content="Install it.">` +
				`<meta property="og:site_name" content="Team Wiki">` +
				`<meta property="og:url" content="https://wiki.example.com/wiki/docs/setup">`,
		},
		{
			name:      "forwarded host and proto have no effect",
			page:      seo.Page{Title: "Setup"},
			publicURL: "https://wiki.example.com",
			headers:   map[string]string{"X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "http", "Host": "evil.example"},
			wantTitle: "Setup - Team Wiki",
			wantHead: `<link rel="canonical" href="https://wiki.example.com/docs/setup">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="Setup">` +
				`<meta property="og:site_name" content="Team Wiki">` +
				`<meta property="og:url" content="https://wiki.example.com/docs/setup">`,
		},
		{
			// The router escapes Title; the contributor escapes Head.
			name:      "special characters in title and description are escaped",
			page:      seo.Page{Title: `A "<b>" & </head>`, Properties: map[string]string{"description": `x"><script>&</head>`}},
			wantTitle: `A "<b>" & </head> - Team Wiki`,
			wantHead: `<meta name="description" content="x&#34;&gt;&lt;script&gt;&amp;&lt;/head&gt;">` +
				`<meta property="og:type" content="website">` +
				`<meta property="og:title" content="A &#34;&lt;b&gt;&#34; &amp; &lt;/head&gt;">` +
				`<meta property="og:description" content="x&#34;&gt;&lt;script&gt;&amp;&lt;/head&gt;">` +
				`<meta property="og:site_name" content="Team Wiki">`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := shellRequest(tc.basePath, "/docs/setup")
			for k, v := range tc.headers {
				req.Request.Header.Set(k, v)
			}

			got := pageMeta(pageAt("docs/setup", tc.page), tc.publicURL).ContributeShell(context.Background(), req)

			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
			if string(got.Head) != tc.wantHead {
				t.Errorf("Head =\n%s\nwant\n%s", got.Head, tc.wantHead)
			}
			if !got.NoCache {
				t.Error("NoCache = false, want true for a shell with page meta")
			}
			if got.Body != "" {
				t.Errorf("Body = %q, want empty", got.Body)
			}
		})
	}
}

func TestPageMeta_NoPage_ContributesNothing(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakePageReader
		routePath string
	}{
		{name: "root", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "/"},
		{name: "unknown page", reader: &fakePageReader{}, routePath: "/missing"},
		{name: "read error", reader: &fakePageReader{err: errors.New("boom")}, routePath: "/docs"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pageMeta(tc.reader, "https://wiki.example.com").ContributeShell(context.Background(), shellRequest("", tc.routePath))
			if got != (wikihttp.ShellParts{}) {
				t.Errorf("parts = %+v, want empty", got)
			}
		})
	}
}

func TestPageMeta_Permalink_AddsTargetPageMetaWithCanonical(t *testing.T) {
	reader := pageAt("docs/setup", seo.Page{Title: "Setup"})
	permalinks := &fakePermalinkResolver{targets: map[string]string{"abc123": "docs/setup"}}
	contributor := permalinkMeta(reader, permalinks, "https://wiki.example.com")

	got := contributor.ContributeShell(context.Background(), shellRequest("", "/p/abc123/old-slug"))

	wantHead := `<link rel="canonical" href="https://wiki.example.com/docs/setup">` +
		`<meta property="og:type" content="website">` +
		`<meta property="og:title" content="Setup">` +
		`<meta property="og:site_name" content="Team Wiki">` +
		`<meta property="og:url" content="https://wiki.example.com/docs/setup">`
	if got.Title != "Setup - Team Wiki" || string(got.Head) != wantHead || !got.NoCache {
		t.Fatalf("parts = %+v\nwant Title %q, NoCache, Head\n%s", got, "Setup - Team Wiki", wantHead)
	}
}

func TestPublicOnly_PermalinkMetaPublicAccessOff_ReadsNothing(t *testing.T) {
	reader := pageAt("docs/setup", seo.Page{Title: "Setup"})
	permalinks := &fakePermalinkResolver{targets: map[string]string{"abc123": "docs/setup"}}
	contributor := seo.NewPublicOnly(publicaccess.Fixed(false)).Contributor(permalinkMeta(reader, permalinks, "https://wiki.example.com"))

	if got := contributor.ContributeShell(context.Background(), shellRequest("", "/p/abc123")); got != (wikihttp.ShellParts{}) {
		t.Errorf("parts = %+v, want empty", got)
	}
	if len(permalinks.asked) != 0 || reader.reads != 0 {
		t.Errorf("resolver asked %q, page reads %d; want nothing read", permalinks.asked, reader.reads)
	}
}

// BenchmarkPageMeta_LargePage measures the contributor on a 100 KB page body
// (excerpt included), the cost added to every shell request of a public wiki.
func BenchmarkPageMeta_LargePage(b *testing.B) {
	paragraph := "Lorem ipsum dolor sit amet, **consectetur** adipiscing elit, [a link](https://example.com) and `code`.\n\n"
	body := strings.Repeat(paragraph, 100*1024/len(paragraph))
	contributor := pageMeta(pageAt("docs/setup", seo.Page{Title: "Setup", Body: body}), "https://wiki.example.com")
	req := shellRequest("", "/docs/setup")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		contributor.ContributeShell(ctx, req)
	}
}

func pageMarkdown(reader *fakePageReader) wikihttp.ShellResponder {
	return seo.NewPageMarkdown(seo.NewGetPageMarkdownUseCase(reader), discardLogger())
}

func markdownRequest(routePath string, accept ...string) wikihttp.ShellRequest {
	req := shellRequest("/wiki", routePath)
	for _, a := range accept {
		req.Request.Header.Add("Accept", a)
	}
	return req
}

func TestPageMarkdown_MarkdownRequested_ServesPageBody(t *testing.T) {
	tests := []struct {
		name   string
		accept []string
	}{
		{name: "one Accept header", accept: []string{"text/markdown"}},
		{name: "ranges spread over two Accept headers", accept: []string{"text/html;q=0.5", "text/markdown"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := pageAt("docs/setup", seo.Page{Title: "Setup", Body: "# Setup\n\n<b>Install</b> it.", Properties: map[string]string{"description": "d"}})
			rec := httptest.NewRecorder()

			if !pageMarkdown(reader).RespondShell(rec, markdownRequest("/docs/setup", tc.accept...)) {
				t.Fatal("markdown request was not handled")
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != "# Setup\n\n<b>Install</b> it." {
				t.Errorf("body = %q, want the page body", got)
			}
			want := map[string]string{
				"Content-Type":           "text/markdown; charset=utf-8",
				"X-Content-Type-Options": "nosniff",
				"Cache-Control":          "no-cache",
				"Vary":                   "Accept",
				"Content-Length":         "27",
				// Opened directly in a browser, the raw content runs no script
				// and reaches no origin storage.
				"Content-Security-Policy": "sandbox",
				"Content-Disposition":     "",
			}
			for h, v := range want {
				if got := rec.Header().Get(h); got != v {
					t.Errorf("%s = %q, want %q", h, got, v)
				}
			}
		})
	}
}

func TestPageMarkdown_NotServed_DeclinesWithVaryOnly(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakePageReader
		routePath string
		accept    string
	}{
		{name: "html requested", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "/docs", accept: "text/html"},
		{name: "no Accept header", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "/docs"},
		{name: "unknown page", reader: &fakePageReader{}, routePath: "/missing", accept: "text/markdown"},
		{name: "root", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "/", accept: "text/markdown"},
		{name: "read error", reader: &fakePageReader{err: errors.New("boom")}, routePath: "/docs", accept: "text/markdown"},
		{name: "browser Accept header", reader: pageAt("docs", seo.Page{Title: "Docs", Body: "x"}), routePath: "/docs", accept: "text/html,application/xhtml+xml,*/*;q=0.8"},
		{name: "markdown refused with q=0", reader: pageAt("docs", seo.Page{Title: "Docs", Body: "x"}), routePath: "/docs", accept: "text/markdown;q=0"},
		{name: "empty route path", reader: pageAt("docs", seo.Page{Title: "Docs", Body: "x"}), routePath: "", accept: "text/markdown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			if pageMarkdown(tc.reader).RespondShell(rec, markdownRequest(tc.routePath, tc.accept)) {
				t.Fatal("responder handled a request it should decline")
			}
			if got := rec.Header().Values("Vary"); len(got) != 1 || got[0] != "Accept" {
				t.Errorf("Vary = %q, want [Accept]: the shell depends on Accept too", got)
			}
			if len(rec.Header()) != 1 || rec.Body.Len() != 0 {
				t.Errorf("declining responder wrote headers %v / body %q, want Vary only", rec.Header(), rec.Body.String())
			}
		})
	}
}

// Vary is added, not set: values another responder or middleware set before
// survive, and Accept appears once even if it is already listed.
func TestPageMarkdown_ExistingVary_KeptWithAcceptOnce(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		reader   *fakePageReader
		want     []string
	}{
		{name: "declining, other value", existing: []string{"Accept-Encoding"}, reader: &fakePageReader{}, want: []string{"Accept-Encoding", "Accept"}},
		{name: "handling, other value", existing: []string{"Accept-Encoding"}, reader: pageAt("docs", seo.Page{Title: "Docs", Body: "x"}), want: []string{"Accept-Encoding", "Accept"}},
		{name: "declining, Accept already set", existing: []string{"Accept"}, reader: &fakePageReader{}, want: []string{"Accept"}},
		{name: "handling, Accept already set", existing: []string{"Accept"}, reader: pageAt("docs", seo.Page{Title: "Docs", Body: "x"}), want: []string{"Accept"}},
		{name: "declining, Accept in a list", existing: []string{"Accept-Encoding, Accept"}, reader: &fakePageReader{}, want: []string{"Accept-Encoding, Accept"}},
		{name: "declining, Accept in a second line", existing: []string{"Origin", " accept "}, reader: &fakePageReader{}, want: []string{"Origin", " accept "}},
		{name: "declining, Vary * covers Accept", existing: []string{"*"}, reader: &fakePageReader{}, want: []string{"*"}},
		{name: "declining, other case", existing: []string{"accept"}, reader: &fakePageReader{}, want: []string{"accept"}},
		{name: "declining, Accept-Encoding is not Accept", existing: []string{"Accept-Encoding,Accept-Language"}, reader: &fakePageReader{}, want: []string{"Accept-Encoding,Accept-Language", "Accept"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			for _, v := range tc.existing {
				rec.Header().Add("Vary", v)
			}

			pageMarkdown(tc.reader).RespondShell(rec, markdownRequest("/docs", "text/markdown"))

			if got := rec.Header().Values("Vary"); strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("Vary = %q, want %q", got, tc.want)
			}
		})
	}
}

// A private wiki's shell is the same for every Accept value, so it gets no
// Vary: Accept and nothing is read.
func TestPublicOnly_PageMarkdownPublicAccessOff_AddsNoVaryAndReadsNothing(t *testing.T) {
	reader := pageAt("docs", seo.Page{Title: "Docs", Body: "x"})
	rec := httptest.NewRecorder()

	if seo.NewPublicOnly(publicaccess.Fixed(false)).Responder(pageMarkdown(reader)).RespondShell(rec, markdownRequest("/docs", "text/markdown")) {
		t.Fatal("responder handled a request while public access is off")
	}
	if len(rec.Header()) != 0 || rec.Body.Len() != 0 {
		t.Errorf("wrote headers %v / body %q, want nothing", rec.Header(), rec.Body.String())
	}
	if reader.reads != 0 {
		t.Errorf("reads = %d, want 0", reader.reads)
	}
}

// The body is written byte for byte: no trimming, no line-ending rewrite,
// and Content-Length counts bytes, not runes.
func TestPageMarkdown_Body_ServedVerbatim(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "CRLF line endings", body: "# Title\r\n\r\nLine\r\n"},
		{name: "lone CR", body: "a\rb"},
		{name: "unicode", body: "# Grüße 🌿\n\nÄÖÜ – 日本語"},
		{name: "leading and trailing whitespace", body: "\n\n  text  \n\n"},
		{name: "shell placeholders and HTML stay raw", body: "{{__BODY__}} </head><script>alert(1)</script>"},
		{name: "frontmatter-looking text in the body", body: "---\ntitle: x\n---\n"},
		{name: "large body", body: strings.Repeat("0123456789abcdef", 64*1024)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			if !pageMarkdown(pageAt("docs", seo.Page{Title: "Docs", Body: tc.body})).RespondShell(rec, markdownRequest("/docs", "text/markdown")) {
				t.Fatal("markdown request was not handled")
			}
			if rec.Code != http.StatusOK || rec.Body.String() != tc.body {
				t.Fatalf("got %d %q, want 200 with the body verbatim", rec.Code, rec.Body.String())
			}
			if got, want := rec.Header().Get("Content-Length"), strconv.Itoa(len(tc.body)); got != want {
				t.Errorf("Content-Length = %q, want %q", got, want)
			}
		})
	}
}
