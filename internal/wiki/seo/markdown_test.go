package seo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/wiki/seo"
)

func TestGetPageMarkdownUseCase_Accept_DecidesWhetherToServeMarkdown(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{name: "text/markdown", accept: "text/markdown", want: true},
		{name: "text/markdown with charset parameter", accept: "text/markdown; charset=utf-8", want: true},
		{name: "text/markdown with q=0 is a refusal", accept: "text/markdown;q=0", want: false},
		{name: "text/markdown with q=0.000 is a refusal", accept: "text/markdown;q=0.000", want: false},
		{name: "tie with text/html prefers markdown", accept: "text/html,text/markdown", want: true},
		{name: "text/html weighted lower", accept: "text/html;q=0.9,text/markdown", want: true},
		{name: "text/html weighted higher", accept: "text/html,text/markdown;q=0.9", want: false},
		{name: "text/* weighted higher counts for text/html", accept: "text/*,text/markdown;q=0.5", want: false},
		{name: "*/* weighted higher counts for text/html", accept: "text/markdown;q=0.5,*/*", want: false},
		{name: "*/* weighted lower", accept: "text/markdown,*/*;q=0.8", want: true},
		{name: "text/html more specific than text/* wins for html", accept: "text/html;q=0.1,text/*,text/markdown;q=0.5", want: true},
		{name: "typical agent header", accept: "text/markdown, text/html;q=0.9, */*;q=0.8", want: true},
		{name: "typical browser header", accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", want: false},
		{name: "*/* alone", accept: "*/*", want: false},
		{name: "text/* alone", accept: "text/*", want: false},
		{name: "empty", accept: "", want: false},
		{name: "upper case and spaces", accept: "  Text/Markdown ; Q=1.0 ,  TEXT/HTML ; q=0.5 ", want: true},
		{name: "malformed q value is ignored with its range", accept: "text/markdown;q=abc", want: false},
		{name: "NaN q value is malformed", accept: "text/markdown;q=NaN", want: false},
		{name: "q value above 1 is malformed", accept: "text/markdown;q=2", want: false},
		{name: "q value with too many digits is malformed", accept: "text/markdown;q=0.5555", want: false},
		{name: "negative q value is malformed", accept: "text/markdown;q=-1", want: false},
		{name: "q value 1.5 is malformed", accept: "text/markdown;q=1.5", want: false},
		{name: "q value without dot is malformed", accept: "text/markdown;q=05", want: false},
		{name: "q value with a letter is malformed", accept: "text/markdown;q=0.5a", want: false},
		{name: "exponent q value is malformed", accept: "text/markdown;q=1e0", want: false},
		{name: "q value 1. is valid", accept: "text/markdown;q=1.", want: true},
		{name: "parameters after q are ignored", accept: "text/markdown;q=0.5;ext=1,text/html;q=0.4", want: true},
		{name: "malformed html range does not block markdown", accept: "text/html;q=x,text/markdown;q=0.5", want: true},
		{name: "media range without subtype", accept: "text,text/markdown", want: true},
		{name: "garbage", accept: ";;,,/;q=", want: false},
		{name: "other markdown-like type", accept: "text/x-markdown", want: false},
		{name: "duplicate ranges use the highest q", accept: "text/markdown;q=0,text/markdown;q=0.5,text/html;q=0.4", want: true},
		{name: "variant parameter", accept: "text/markdown;variant=GFM", want: true},
		{name: "charset and variant before a lower q", accept: "text/markdown;charset=utf-8;variant=CommonMark;q=0.5,text/html;q=0.6", want: false},
		{name: "charset before q, html lower", accept: "text/markdown; charset=UTF-8; q=0.7, text/html; q=0.6", want: true},
		{name: "smallest positive q is still asked for", accept: "text/markdown;q=0.001", want: true},
		{name: "html refused explicitly", accept: "text/html;q=0,text/markdown;q=0.1", want: true},
		{name: "*/* refused explicitly", accept: "text/markdown;q=0.001,*/*;q=0", want: true},
		{name: "q=1.000 is valid", accept: "text/markdown;q=1.000", want: true},
		{name: "q=1.001 is malformed", accept: "text/markdown;q=1.001", want: false},
		{name: "q=0. is a refusal", accept: "text/markdown;q=0.", want: false},
		{name: "empty q value is malformed", accept: "text/markdown;q=", want: false},
		{name: "q=0 markdown with text/* is a refusal", accept: "text/markdown;q=0,text/*", want: false},
		{name: "first q of two wins", accept: "text/markdown;q=0.5;q=0.1,text/html;q=0.4", want: true},
		{name: "most specific html match wins over a higher */*", accept: "text/*;q=0.1,*/*,text/markdown;q=0.5", want: true},
		{name: "duplicate html ranges use the highest q", accept: "text/html;q=0.1,text/html;q=0.9,text/markdown;q=0.5", want: false},
		{name: "all upper case", accept: "TEXT/MARKDOWN", want: true},
		{name: "application/markdown is not text/markdown", accept: "application/markdown", want: false},
		{name: "non-ASCII type is not text/markdown", accept: "tëxt/markdown", want: false},
		{name: "tab whitespace", accept: "\ttext/markdown\t;\tq=0.9 ,\ttext/html;q=0.8", want: true},
		// Quoted values are not parsed (documented on prefersMarkdown): the
		// quoted comma splits the list, so the hidden text/markdown counts.
		{name: "quoted comma splits the list", accept: `text/html;q=0.5;ext="x,text/markdown;y=z"`, want: true},
		{name: "quoted comma before a quote-ended range", accept: `text/html;q=0.5;ext="x,text/markdown"`, want: false},
		{name: "quoted comma inside a markdown parameter still selects markdown", accept: `text/markdown;foo="a,b",text/html;q=0.5`, want: true},
		{name: "many ranges, markdown last", accept: strings.Repeat("application/x-foo;q=0.1,", 5000) + "text/markdown", want: true},
		{name: "many ranges, html last and higher", accept: "text/markdown;q=0.5," + strings.Repeat("image/png,", 5000) + "text/html", want: false},
		{name: "very long parameter value", accept: "text/markdown;x=" + strings.Repeat("a", 100_000), want: true},
		{name: "only commas", accept: strings.Repeat(",", 1000), want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := pageAt("docs/setup", seo.Page{Title: "Setup", Body: "# Setup\n\nInstall it."})

			out, err := seo.NewGetPageMarkdownUseCase(reader).Execute(context.Background(), seo.GetPageMarkdownInput{
				RoutePath: "/docs/setup",
				Accept:    tc.accept,
			})
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if got := out != nil; got != tc.want {
				t.Fatalf("served markdown = %v, want %v", got, tc.want)
			}
			if !tc.want && reader.reads != 0 {
				t.Errorf("reads = %d, want 0 when markdown is not asked for", reader.reads)
			}
			if tc.want && out.Markdown != "# Setup\n\nInstall it." {
				t.Errorf("Markdown = %q, want the page body", out.Markdown)
			}
		})
	}
}

func TestGetPageMarkdownUseCase_NoPage_ReturnsNil(t *testing.T) {
	tests := []struct {
		name      string
		reader    *fakePageReader
		routePath string
		wantReads int
		wantErr   bool
	}{
		{name: "root", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "/", wantReads: 0},
		{name: "empty route path", reader: pageAt("docs", seo.Page{Title: "Docs"}), routePath: "", wantReads: 0},
		{name: "unknown page", reader: &fakePageReader{}, routePath: "/missing", wantReads: 1},
		// The SPA shows the permalink's target there, not the page at this path.
		{name: "permalink route", reader: pageAt("p/abc123", seo.Page{Title: "Shadowed"}), routePath: "/p/abc123", wantReads: 0},
		{name: "permalink route with slug", reader: pageAt("p/abc123/setup", seo.Page{Title: "Shadowed"}), routePath: "/p/abc123/setup", wantReads: 0},
		{name: "permalink route with upper-case P", reader: pageAt("P/abc123", seo.Page{Title: "Shadowed"}), routePath: "/P/abc123", wantReads: 0},
		{name: "read error", reader: &fakePageReader{err: errors.New("boom")}, routePath: "/docs", wantReads: 1, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := seo.NewGetPageMarkdownUseCase(tc.reader).Execute(context.Background(), seo.GetPageMarkdownInput{
				RoutePath: tc.routePath,
				Accept:    "text/markdown",
			})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if out != nil {
				t.Errorf("out = %+v, want nil", out)
			}
			if tc.reader.reads != tc.wantReads {
				t.Errorf("reads = %d, want %d", tc.reader.reads, tc.wantReads)
			}
		})
	}
}

func TestGetPageMarkdownUseCase_RoutePathWithLeadingSlash_ReadsItWithout(t *testing.T) {
	reader := pageAt("docs/setup", seo.Page{Title: "Setup", Body: "x"})

	if _, err := seo.NewGetPageMarkdownUseCase(reader).Execute(context.Background(), seo.GetPageMarkdownInput{
		RoutePath: "/docs/setup",
		Accept:    "text/markdown",
	}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if len(reader.asked) != 1 || reader.asked[0] != "docs/setup" {
		t.Fatalf("asked = %q, want [docs/setup]", reader.asked)
	}
}
