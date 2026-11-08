package seo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki/seo"
)

// fakePageReader returns pages by route path (missing = seo.ErrPageNotFound),
// nothing at all while notLoaded (like a tree that is not loaded yet), or a
// fixed error, and counts its reads.
type fakePageReader struct {
	pages     map[string]*seo.Page
	notLoaded bool
	err       error
	reads     int
	asked     []string
}

func (f *fakePageReader) PageByRoutePath(_ context.Context, routePath string) (*seo.Page, error) {
	f.reads++
	f.asked = append(f.asked, routePath)
	if f.err != nil {
		return nil, f.err
	}
	if f.notLoaded {
		return nil, nil
	}
	page, ok := f.pages[routePath]
	if !ok {
		return nil, seo.ErrPageNotFound
	}
	return page, nil
}

func pageAt(routePath string, page seo.Page) *fakePageReader {
	return &fakePageReader{pages: map[string]*seo.Page{routePath: &page}}
}

func staticSiteName(name string) func() string { return func() string { return name } }

// fakePermalinkResolver resolves page IDs to route paths (missing =
// seo.ErrPageNotFound), resolves nothing at all while notLoaded, or returns a
// fixed error, and records the IDs it was asked for.
type fakePermalinkResolver struct {
	targets   map[string]string
	notLoaded bool
	err       error
	asked     []string
}

func (f *fakePermalinkResolver) ResolvePermalink(_ context.Context, id string) (*seo.PermalinkTarget, error) {
	f.asked = append(f.asked, id)
	if f.err != nil {
		return nil, f.err
	}
	if f.notLoaded {
		return nil, nil
	}
	routePath, ok := f.targets[id]
	if !ok {
		return nil, seo.ErrPageNotFound
	}
	return &seo.PermalinkTarget{RoutePath: routePath}, nil
}

// noPermalinks resolves no ID, for tests of page routes.
func noPermalinks() *fakePermalinkResolver { return &fakePermalinkResolver{} }

func TestGetPageMetaUseCase_Page_BuildsMeta(t *testing.T) {
	longBody := strings.Repeat("word ", 100)
	tests := []struct {
		name      string
		page      seo.Page
		siteName  string
		publicURL string
		basePath  string
		routePath string
		want      seo.GetPageMetaOutput
	}{
		{
			name:      "title is page title, dash, site name",
			page:      seo.Page{Title: "Setup", Body: "Install it."},
			siteName:  "Team Wiki",
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - Team Wiki", PageTitle: "Setup", SiteName: "Team Wiki", Description: "Install it."},
		},
		{
			name:      "empty site name falls back to the default",
			page:      seo.Page{Title: "Setup", Body: "Install it."},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: "Install it."},
		},
		{
			name:      "description property wins over the body",
			page:      seo.Page{Title: "Setup", Body: "Install it.", Properties: map[string]string{"description": "How to set it up."}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: "How to set it up."},
		},
		{
			name:      "description property under the cap is used in full, whitespace collapsed",
			page:      seo.Page{Title: "Setup", Properties: map[string]string{"description": "  " + strings.Repeat("word ", 60) + "\n\t end  "}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: strings.Repeat("word ", 60) + "end"},
		},
		{
			// Capped at 2 x excerpt.MaxRunes, cut like an excerpt: at a word
			// boundary, then "...".
			name:      "long description property is capped at a word boundary",
			page:      seo.Page{Title: "Setup", Properties: map[string]string{"description": longBody}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: strings.TrimSpace(strings.Repeat("word ", 71)) + "..."},
		},
		{
			name:      "description property cap counts runes, not bytes",
			page:      seo.Page{Title: "Setup", Properties: map[string]string{"description": strings.Repeat("ö", 400)}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: strings.Repeat("ö", 360) + "..."},
		},
		{
			name:      "blank description property falls back to the excerpt",
			page:      seo.Page{Title: "Setup", Body: "# Setup\n\nInstall **it**.", Properties: map[string]string{"description": " \n "}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: "Setup Install it."},
		},
		{
			name:      "only the exact key description counts",
			page:      seo.Page{Title: "Setup", Body: "Install it.", Properties: map[string]string{"Description": "x", "summary": "y"}},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: "Install it."},
		},
		{
			name:      "excerpt is truncated like list excerpts",
			page:      seo.Page{Title: "Setup", Body: longBody},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: strings.TrimSpace(strings.Repeat("word ", 35)) + "..."},
		},
		{
			// The excerpt helper returns HTML-escaped text; the description is
			// plain text, escaped once where it becomes markup.
			name:      "excerpt is plain text, not HTML-escaped",
			page:      seo.Page{Title: "Setup", Body: "Fish & chips <3 \"quoted\" it's"},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: `Fish & chips <3 "quoted" it's`},
		},
		{
			name:      "empty body gives no description",
			page:      seo.Page{Title: "Setup"},
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki"},
		},
		{
			name:      "canonical url from public url and route path",
			page:      seo.Page{Title: "Setup"},
			publicURL: "https://wiki.example.com",
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", CanonicalURL: "https://wiki.example.com/docs/setup"},
		},
		{
			name:      "canonical url includes the base path once",
			page:      seo.Page{Title: "Setup"},
			publicURL: "https://wiki.example.com/wiki/",
			basePath:  "/wiki",
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", CanonicalURL: "https://wiki.example.com/wiki/docs/setup"},
		},
		{
			name:      "canonical url takes only the origin of the public url",
			page:      seo.Page{Title: "Setup"},
			publicURL: "http://wiki.example.com:8080/other?x=1#f",
			basePath:  "/wiki",
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", CanonicalURL: "http://wiki.example.com:8080/wiki/docs/setup"},
		},
		{
			name:      "canonical url escapes the route path",
			page:      seo.Page{Title: "Größe"},
			publicURL: "https://wiki.example.com",
			routePath: `/a b/größe?"<x>`,
			want:      seo.GetPageMetaOutput{Title: "Größe - LeafWiki", PageTitle: "Größe", SiteName: "LeafWiki", CanonicalURL: "https://wiki.example.com/a%20b/gr%C3%B6%C3%9Fe%3F%22%3Cx%3E"},
		},
		{
			name:      "unusable public url gives no canonical url",
			page:      seo.Page{Title: "Setup"},
			publicURL: "/relative",
			routePath: "/docs/setup",
			want:      seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := pageAt(strings.TrimPrefix(tc.routePath, "/"), tc.page)
			uc := seo.NewGetPageMetaUseCase(reader, noPermalinks(), staticSiteName(tc.siteName), tc.publicURL)

			out, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath, BasePath: tc.basePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if out == nil {
				t.Fatal("Execute returned no meta for an existing page")
			}
			if *out != tc.want {
				t.Errorf("meta = %+v\nwant   %+v", *out, tc.want)
			}
		})
	}
}

// A route without a page gets only NoIndex: the SPA shows an app screen or
// "not found" there. Without an answer (root, tree not loaded) there is no meta.
func TestGetPageMetaUseCase_NoPage_MarksNoindexOrNothing(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakePageReader
		routePath string
		want      *seo.GetPageMetaOutput
		wantReads int
	}{
		{name: "root is not a page route", reader: pageAt("", seo.Page{Title: "x"}), routePath: "/", wantReads: 0},
		{name: "tree not loaded: no meta", reader: &fakePageReader{notLoaded: true}, routePath: "/missing", wantReads: 1},
		{name: "unknown page: noindex", reader: &fakePageReader{}, routePath: "/missing", want: &seo.GetPageMetaOutput{NoIndex: true}, wantReads: 1},
		{name: "SPA route: noindex", reader: &fakePageReader{}, routePath: "/login", want: &seo.GetPageMetaOutput{NoIndex: true}, wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := seo.NewGetPageMetaUseCase(tc.reader, noPermalinks(), staticSiteName(""), "https://wiki.example.com")

			out, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if (out == nil) != (tc.want == nil) || (out != nil && *out != *tc.want) {
				t.Fatalf("meta = %+v, want %+v", out, tc.want)
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}

// The page read drops the leading slash and every trailing slash, like the
// SPA's page read; the root stays the root, empty segments inside stay.
func TestGetPageMetaUseCase_RoutePath_ReadsItNormalized(t *testing.T) {
	tests := []struct {
		name      string
		routePath string
		wantReads []string
	}{
		{name: "page", routePath: "/docs/setup", wantReads: []string{"docs/setup"}},
		{name: "trailing slash", routePath: "/docs/setup/", wantReads: []string{"docs/setup"}},
		{name: "several trailing slashes", routePath: "/docs/setup///", wantReads: []string{"docs/setup"}},
		{name: "empty segment inside stays", routePath: "/docs//setup", wantReads: []string{"docs//setup"}},
		{name: "root is not read", routePath: "/"},
		{name: "empty route is not read", routePath: ""},
		{name: "slashes only are the root", routePath: "//"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakePageReader{}
			uc := seo.NewGetPageMetaUseCase(reader, noPermalinks(), staticSiteName(""), "")

			if _, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath}); err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if strings.Join(reader.asked, ",") != strings.Join(tc.wantReads, ",") {
				t.Errorf("reader asked %q, want %q", reader.asked, tc.wantReads)
			}
		})
	}
}

// A permalink route ("/p/<id>" or "/p/<id>/<slug>", the SPA route
// "/p/:id/:slug?") gets the meta of the page the ID points at, with the page's
// route path as canonical URL; the slug is decorative and may be stale.
func TestGetPageMetaUseCase_Permalink_UsesTargetPage(t *testing.T) {
	setup := seo.Page{Title: "Setup", Body: "Install it."}
	tests := []struct {
		name      string
		routePath string
		basePath  string
		wantURL   string
	}{
		{name: "id alone", routePath: "/p/abc123", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "id and current slug", routePath: "/p/abc123/setup", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "id and wrong slug", routePath: "/p/abc123/old-name", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "trailing slash", routePath: "/p/abc123/", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "with base path", routePath: "/p/abc123/setup", basePath: "/wiki", wantURL: "https://wiki.example.com/wiki/docs/setup"},
		// The SPA ignores any number of trailing slashes and takes ".." as a
		// slug (react-router matchRoutes).
		{name: "several trailing slashes", routePath: "/p/abc123//", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "slug with trailing slash", routePath: "/p/abc123/setup/", wantURL: "https://wiki.example.com/docs/setup"},
		{name: "dot-dot slug is decorative", routePath: "/p/abc123/..", wantURL: "https://wiki.example.com/docs/setup"},
		// react-router matches path segments case-insensitively.
		{name: "upper-case P", routePath: "/P/abc123", wantURL: "https://wiki.example.com/docs/setup"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A page at the literal path "p/abc123" is not what the SPA shows there.
			reader := &fakePageReader{pages: map[string]*seo.Page{"docs/setup": &setup, "p/abc123": {Title: "Shadowed"}}}
			permalinks := &fakePermalinkResolver{targets: map[string]string{"abc123": "docs/setup"}}
			uc := seo.NewGetPageMetaUseCase(reader, permalinks, staticSiteName(""), "https://wiki.example.com")

			out, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath, BasePath: tc.basePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			want := seo.GetPageMetaOutput{Title: "Setup - LeafWiki", PageTitle: "Setup", SiteName: "LeafWiki", Description: "Install it.", CanonicalURL: tc.wantURL}
			if out == nil || *out != want {
				t.Fatalf("meta = %+v\nwant   %+v", out, want)
			}
			if len(permalinks.asked) != 1 || permalinks.asked[0] != "abc123" {
				t.Errorf("resolver asked %q, want [abc123]", permalinks.asked)
			}
			if len(reader.asked) != 1 || reader.asked[0] != "docs/setup" {
				t.Errorf("reader asked %q, want [docs/setup]", reader.asked)
			}
		})
	}
}

// Routes that are no permalink in the SPA ("/p/" alone, an empty ID, extra
// segments) stay ordinary page routes. A permalink or page route without a
// page gets only NoIndex; the root's ID and a tree that is not loaded get no meta.
func TestGetPageMetaUseCase_PermalinkNoPage_MarksNoindexOrNothing(t *testing.T) {
	tests := []struct {
		name          string
		routePath     string
		permalinks    *fakePermalinkResolver
		wantNoIndex   bool
		wantResolved  []string
		wantPageReads []string
	}{
		{name: "unknown id", routePath: "/p/unknown", wantNoIndex: true, wantResolved: []string{"unknown"}},
		{name: "id of the root", routePath: "/p/root", permalinks: &fakePermalinkResolver{targets: map[string]string{"root": ""}}, wantResolved: []string{"root"}},
		{name: "tree not loaded", routePath: "/p/abc123", permalinks: &fakePermalinkResolver{notLoaded: true}, wantResolved: []string{"abc123"}},
		{name: "target gone before the page read", routePath: "/p/abc123", permalinks: &fakePermalinkResolver{targets: map[string]string{"abc123": "gone"}}, wantNoIndex: true, wantResolved: []string{"abc123"}, wantPageReads: []string{"gone"}},
		{name: "p alone is a page route", routePath: "/p", wantNoIndex: true, wantPageReads: []string{"p"}},
		{name: "p with slash is the page route p", routePath: "/p/", wantNoIndex: true, wantPageReads: []string{"p"}},
		{name: "empty id is a page route", routePath: "/p//setup", wantNoIndex: true, wantPageReads: []string{"p//setup"}},
		{name: "extra segments are a page route", routePath: "/p/abc123/setup/more", wantNoIndex: true, wantPageReads: []string{"p/abc123/setup/more"}},
		{name: "prefix without separator is a page route", routePath: "/pabc123", wantNoIndex: true, wantPageReads: []string{"pabc123"}},
		// The ID reaches the resolver as is; it is a map key there, never a path.
		{name: "unicode id", routePath: "/p/größe", wantNoIndex: true, wantResolved: []string{"größe"}},
		{name: "dot-dot id", routePath: "/p/..", wantNoIndex: true, wantResolved: []string{".."}},
		{name: "id with markup characters", routePath: `/p/a b"<x>&`, wantNoIndex: true, wantResolved: []string{`a b"<x>&`}},
		{name: "very long id", routePath: "/p/" + strings.Repeat("a", 10000), wantNoIndex: true, wantResolved: []string{strings.Repeat("a", 10000)}},
		{name: "double slash before the id is a page route", routePath: "/p//abc123", wantNoIndex: true, wantPageReads: []string{"p//abc123"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakePageReader{}
			permalinks := tc.permalinks
			if permalinks == nil {
				permalinks = noPermalinks()
			}
			uc := seo.NewGetPageMetaUseCase(reader, permalinks, staticSiteName(""), "https://wiki.example.com")

			out, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if tc.wantNoIndex {
				if out == nil || *out != (seo.GetPageMetaOutput{NoIndex: true}) {
					t.Fatalf("meta = %+v, want only NoIndex", out)
				}
			} else if out != nil {
				t.Fatalf("meta = %+v, want nil", out)
			}
			if strings.Join(permalinks.asked, ",") != strings.Join(tc.wantResolved, ",") {
				t.Errorf("resolver asked %q, want %q", permalinks.asked, tc.wantResolved)
			}
			if strings.Join(reader.asked, ",") != strings.Join(tc.wantPageReads, ",") {
				t.Errorf("reader asked %q, want %q", reader.asked, tc.wantPageReads)
			}
		})
	}
}

func TestGetPageMetaUseCase_ReadError_IsReturned(t *testing.T) {
	readErr := errors.New("disk on fire")
	tests := []struct {
		name       string
		reader     *fakePageReader
		permalinks *fakePermalinkResolver
		routePath  string
		wantReads  int
	}{
		{name: "page reader error", reader: &fakePageReader{err: readErr}, permalinks: noPermalinks(), routePath: "/docs", wantReads: 1},
		{name: "permalink resolver error, no page read", reader: &fakePageReader{}, permalinks: &fakePermalinkResolver{err: readErr}, routePath: "/p/abc123", wantReads: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := seo.NewGetPageMetaUseCase(tc.reader, tc.permalinks, staticSiteName(""), "")

			if _, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath}); !errors.Is(err, readErr) {
				t.Fatalf("err = %v, want %v", err, readErr)
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}

func TestDocumentTitle_Format_MatchesTheSPA(t *testing.T) {
	tests := []struct {
		name      string
		pageTitle string
		siteName  string
		want      string
	}{
		{name: "page title, dash, site name", pageTitle: "Setup", siteName: "Team Wiki", want: "Setup - Team Wiki"},
		{name: "site name alone without a page title", pageTitle: "", siteName: "Team Wiki", want: "Team Wiki"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := seo.DocumentTitle(tc.pageTitle, tc.siteName); got != tc.want {
				t.Errorf("DocumentTitle(%q, %q) = %q, want %q", tc.pageTitle, tc.siteName, got, tc.want)
			}
		})
	}
}
