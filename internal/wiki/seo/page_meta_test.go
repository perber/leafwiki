package seo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki/seo"
)

// fakePageReader returns pages by route path (missing = no page) or a fixed
// error, and counts its reads.
type fakePageReader struct {
	pages map[string]*seo.Page
	err   error
	reads int
	asked []string
}

func (f *fakePageReader) PageByRoutePath(_ context.Context, routePath string) (*seo.Page, error) {
	f.reads++
	f.asked = append(f.asked, routePath)
	if f.err != nil {
		return nil, f.err
	}
	return f.pages[routePath], nil
}

func pageAt(routePath string, page seo.Page) *fakePageReader {
	return &fakePageReader{pages: map[string]*seo.Page{routePath: &page}}
}

func staticSiteName(name string) func() string { return func() string { return name } }

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
			uc := seo.NewGetPageMetaUseCase(reader, staticSiteName(tc.siteName), tc.publicURL)

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

func TestGetPageMetaUseCase_NoPage_ReturnsNoMeta(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakePageReader
		routePath string
		wantReads int
	}{
		{name: "root is not a page route", reader: pageAt("", seo.Page{Title: "x"}), routePath: "/", wantReads: 0},
		{name: "unknown page", reader: &fakePageReader{}, routePath: "/missing", wantReads: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			uc := seo.NewGetPageMetaUseCase(tc.reader, staticSiteName(""), "https://wiki.example.com")

			out, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: tc.routePath})
			if err != nil || out != nil {
				t.Fatalf("Execute = %+v, %v; want nil, nil", out, err)
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}

func TestGetPageMetaUseCase_ReaderError_IsReturned(t *testing.T) {
	readErr := errors.New("disk on fire")
	uc := seo.NewGetPageMetaUseCase(&fakePageReader{err: readErr}, staticSiteName(""), "")

	if _, err := uc.Execute(context.Background(), seo.GetPageMetaInput{RoutePath: "/docs"}); !errors.Is(err, readErr) {
		t.Fatalf("err = %v, want %v", err, readErr)
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
