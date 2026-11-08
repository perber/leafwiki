package seo_test

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/wiki/seo"
)

// fakePageRouteLister returns fixed routes (cut at the limit like the tree
// does; truncated reports more even below it) or an error, and records the
// limit it was asked for.
type fakePageRouteLister struct {
	routes    []seo.PageRoute
	truncated bool
	err       error
	reads     int
	lastLimit int
}

func (f *fakePageRouteLister) ListPageRoutes(_ context.Context, limit int) ([]seo.PageRoute, bool, error) {
	f.reads++
	f.lastLimit = limit
	if f.err != nil {
		return nil, false, f.err
	}
	if limit > 0 && len(f.routes) > limit {
		return f.routes[:limit], true, nil
	}
	return f.routes, f.truncated, nil
}

func routesAt(routePaths ...string) *fakePageRouteLister {
	f := &fakePageRouteLister{}
	for _, p := range routePaths {
		f.routes = append(f.routes, seo.PageRoute{RoutePath: p})
	}
	return f
}

// sitemapXML builds the sitemap and returns the written document.
func sitemapXML(t *testing.T, lister seo.PageRouteLister, publicURL, basePath string) string {
	t.Helper()
	out, err := seo.NewBuildSitemapUseCase(lister, publicURL).Execute(context.Background(), seo.BuildSitemapInput{BasePath: basePath})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if out == nil {
		t.Fatal("Execute returned no sitemap")
	}
	var b strings.Builder
	if err := out.WriteXML(&b); err != nil {
		t.Fatalf("WriteXML failed: %v", err)
	}
	return b.String()
}

// sitemapLocs parses the document and returns its <loc> values, so the test
// also proves the output is well-formed XML.
func sitemapLocs(t *testing.T, doc string) []string {
	t.Helper()
	var set struct {
		XMLName xml.Name `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
		URLs    []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(doc), &set); err != nil {
		t.Fatalf("sitemap is not valid XML: %v\n%s", err, doc)
	}
	locs := make([]string, len(set.URLs))
	for i, u := range set.URLs {
		locs[i] = u.Loc
	}
	return locs
}

func TestBuildSitemapUseCase_ListsEveryRoute(t *testing.T) {
	tests := []struct {
		name      string
		routes    []string
		publicURL string
		basePath  string
		want      []string
	}{
		{
			name:      "pages and sections in navigation order",
			routes:    []string{"docs", "docs/intro", "docs/advanced", "about"},
			publicURL: "https://wiki.example.com",
			want: []string{
				"https://wiki.example.com/docs",
				"https://wiki.example.com/docs/intro",
				"https://wiki.example.com/docs/advanced",
				"https://wiki.example.com/about",
			},
		},
		{name: "trailing slash on public url", routes: []string{"docs"}, publicURL: "https://wiki.example.com/", want: []string{"https://wiki.example.com/docs"}},
		{
			// The path of --public-url is only checked against --base-path at
			// startup; the base path is what the router serves under.
			name:      "base path inside the public url is not doubled",
			routes:    []string{"docs"},
			publicURL: "https://wiki.example.com/wiki",
			basePath:  "/wiki",
			want:      []string{"https://wiki.example.com/wiki/docs"},
		},
		{name: "public url port is kept", routes: []string{"docs"}, publicURL: "http://localhost:8080", want: []string{"http://localhost:8080/docs"}},
		{name: "unicode segment is percent-escaped", routes: []string{"größe"}, publicURL: "https://wiki.example.com", want: []string{"https://wiki.example.com/gr%C3%B6%C3%9Fe"}},
		{
			name:      "ampersand, angle brackets and quotes",
			routes:    []string{`a&b`, `c<d>"e`},
			publicURL: "https://wiki.example.com",
			want:      []string{"https://wiki.example.com/a&b", "https://wiki.example.com/c%3Cd%3E%22e"},
		},
		{name: "unusable route paths are left out", routes: []string{"", "docs//intro", "/docs", "about"}, publicURL: "https://wiki.example.com", want: []string{"https://wiki.example.com/about"}},
		{name: "empty wiki", publicURL: "https://wiki.example.com", want: []string{}},
		{name: "percent sign is escaped", routes: []string{"100%"}, publicURL: "https://wiki.example.com", want: []string{"https://wiki.example.com/100%25"}},
		{name: "apostrophe survives the xml round trip", routes: []string{"it's"}, publicURL: "https://wiki.example.com", want: []string{"https://wiki.example.com/it%27s"}},
		{
			name:      "backslash, question mark, hash and space are escaped",
			routes:    []string{`a\b`, "c?d#e f"},
			publicURL: "https://wiki.example.com",
			want:      []string{"https://wiki.example.com/a%5Cb", "https://wiki.example.com/c%3Fd%23e%20f"},
		},
		{
			// Characters XML 1.0 cannot carry must not break the document.
			name:      "control characters and invalid utf-8 are escaped",
			routes:    []string{"a\x01b", "c\xffd", "e\r\nf"},
			publicURL: "https://wiki.example.com",
			want:      []string{"https://wiki.example.com/a%01b", "https://wiki.example.com/c%FFd", "https://wiki.example.com/e%0D%0Af"},
		},
		{name: "query and fragment of the public url are dropped", routes: []string{"docs"}, publicURL: "https://wiki.example.com/?utm=1#top", want: []string{"https://wiki.example.com/docs"}},
		{name: "credentials in the public url never appear", routes: []string{"docs"}, publicURL: "https://admin:s3cret@wiki.example.com", want: []string{"https://wiki.example.com/docs"}},
		{name: "upper-case scheme is normalised", routes: []string{"docs"}, publicURL: "HTTPS://wiki.example.com", want: []string{"https://wiki.example.com/docs"}},
		{name: "public url with surrounding spaces", routes: []string{"docs"}, publicURL: "  https://wiki.example.com  ", want: []string{"https://wiki.example.com/docs"}},
		{name: "base path with trailing slash", routes: []string{"docs"}, publicURL: "https://wiki.example.com/wiki/", basePath: "/wiki/", want: []string{"https://wiki.example.com/wiki/docs"}},
		{name: "base path of a single slash", routes: []string{"docs"}, publicURL: "https://wiki.example.com", basePath: "/", want: []string{"https://wiki.example.com/docs"}},
		{
			name:      "very long route path",
			routes:    []string{strings.Repeat("a", 2000) + "/" + strings.Repeat("b", 2000)},
			publicURL: "https://wiki.example.com",
			want:      []string{"https://wiki.example.com/" + strings.Repeat("a", 2000) + "/" + strings.Repeat("b", 2000)},
		},
		{name: "ipv6 host with port", routes: []string{"docs"}, publicURL: "http://[::1]:8080", want: []string{"http://[::1]:8080/docs"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := sitemapXML(t, routesAt(tc.routes...), tc.publicURL, tc.basePath)

			got := sitemapLocs(t, doc)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("locs =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestBuildSitemapUseCase_WritesSitemapDocument(t *testing.T) {
	lister := &fakePageRouteLister{routes: []seo.PageRoute{
		{RoutePath: "a&b", UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600))},
		{RoutePath: "never-saved"},
	}}

	got := sitemapXML(t, lister, "https://wiki.example.com", "")

	want := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n" +
		`  <url>` + "\n" +
		`    <loc>https://wiki.example.com/a&amp;b</loc>` + "\n" +
		`    <lastmod>2026-01-02T02:04:05Z</lastmod>` + "\n" +
		`  </url>` + "\n" +
		`  <url>` + "\n" +
		`    <loc>https://wiki.example.com/never-saved</loc>` + "\n" +
		`  </url>` + "\n" +
		`</urlset>`
	if got != want {
		t.Errorf("sitemap =\n%s\nwant\n%s", got, want)
	}
}

func TestBuildSitemapOutput_WriteXML_ReturnsWriteError(t *testing.T) {
	out := &seo.BuildSitemapOutput{URLs: []seo.SitemapURL{{Loc: "https://wiki.example.com/docs"}}}

	if err := out.WriteXML(failingWriter{}); !errors.Is(err, errWriteFailed) {
		t.Fatalf("err = %v, want %v", err, errWriteFailed)
	}
}

var errWriteFailed = errors.New("connection reset")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

func TestBuildSitemapUseCase_TooManyRoutes_Truncates(t *testing.T) {
	lister := &fakePageRouteLister{}
	for i := range seo.MaxSitemapURLs + 1 {
		lister.routes = append(lister.routes, seo.PageRoute{RoutePath: fmt.Sprintf("p%d", i)})
	}

	out, err := seo.NewBuildSitemapUseCase(lister, "https://wiki.example.com").Execute(context.Background(), seo.BuildSitemapInput{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if lister.lastLimit != seo.MaxSitemapURLs {
		t.Errorf("limit = %d, want %d", lister.lastLimit, seo.MaxSitemapURLs)
	}
	if !out.Truncated || len(out.URLs) != seo.MaxSitemapURLs {
		t.Errorf("Truncated = %v with %d URLs, want true with %d", out.Truncated, len(out.URLs), seo.MaxSitemapURLs)
	}
}

func TestBuildSitemapUseCase_NoSitemap(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
	}{
		{name: "without public url", publicURL: ""},
		{name: "unusable public url", publicURL: "wiki.example.com"},
		{name: "public url with another scheme", publicURL: "ftp://wiki.example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lister := routesAt("docs")

			out, err := seo.NewBuildSitemapUseCase(lister, tc.publicURL).Execute(context.Background(), seo.BuildSitemapInput{})
			if err != nil || out != nil {
				t.Fatalf("Execute = %+v, %v; want nil, nil", out, err)
			}
			if lister.reads != 0 {
				t.Errorf("reads = %d, want 0", lister.reads)
			}
		})
	}
}

func TestBuildSitemapUseCase_ReadError_IsReturned(t *testing.T) {
	readErr := errors.New("disk on fire")

	out, err := seo.NewBuildSitemapUseCase(&fakePageRouteLister{err: readErr}, "https://wiki.example.com").Execute(context.Background(), seo.BuildSitemapInput{})
	if !errors.Is(err, readErr) || out != nil {
		t.Fatalf("Execute = %+v, %v; want nil, %v", out, err, readErr)
	}
}

// BenchmarkSitemap measures building and writing the sitemap of a large wiki
// (fake lister, so without the tree walk), the cost of one crawler fetch on
// small hardware.
func BenchmarkSitemap(b *testing.B) {
	for _, n := range []int{10_000, seo.MaxSitemapURLs} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			lister := &fakePageRouteLister{}
			updated := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			for i := range n {
				lister.routes = append(lister.routes, seo.PageRoute{RoutePath: fmt.Sprintf("section-%d/page-%d", i/100, i), UpdatedAt: updated})
			}
			uc := seo.NewBuildSitemapUseCase(lister, "https://wiki.example.com")
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				out, err := uc.Execute(ctx, seo.BuildSitemapInput{BasePath: "/wiki"})
				if err != nil {
					b.Fatal(err)
				}
				if err := out.WriteXML(io.Discard); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
