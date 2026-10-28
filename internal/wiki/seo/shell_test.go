package seo_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
	return seo.NewPageMeta(seo.NewGetPageMetaUseCase(reader, staticSiteName("Team Wiki"), publicURL), discardLogger())
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
