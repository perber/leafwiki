package seo_test

import (
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/seo"
)

func TestPageMeta_DocumentTitle(t *testing.T) {
	cases := []struct {
		name      string
		meta      seo.PageMeta
		wantTitle string
	}{
		{"page and site name", seo.PageMeta{SiteName: "LeafWiki", PageTitle: "Intro"}, "Intro · LeafWiki"},
		{"no page title", seo.PageMeta{SiteName: "LeafWiki"}, "LeafWiki"},
		{"no site name", seo.PageMeta{PageTitle: "Intro"}, "Intro"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.meta.DocumentTitle(); got != tc.wantTitle {
				t.Fatalf("DocumentTitle() = %q, want %q", got, tc.wantTitle)
			}
		})
	}
}

func TestPageMeta_HeadTags_EscapesAndIncludesCanonical(t *testing.T) {
	meta := seo.BuildPageMeta("LeafWiki", `Title <with> "quotes"`, "Some body content.", "https://wiki.example.com/docs/intro")

	head := meta.HeadTags()

	if !strings.Contains(head, `<link rel="canonical" href="https://wiki.example.com/docs/intro">`) {
		t.Fatalf("expected canonical link, got %q", head)
	}
	if !strings.Contains(head, `<meta property="og:url" content="https://wiki.example.com/docs/intro">`) {
		t.Fatalf("expected og:url meta tag, got %q", head)
	}
	if strings.Contains(head, "Title <with>") {
		t.Fatalf("expected page title to be HTML-escaped, got %q", head)
	}
	if !strings.Contains(head, "&lt;with&gt;") {
		t.Fatalf("expected escaped title in og:title, got %q", head)
	}
	if !strings.Contains(head, `<meta name="description" content="Some body content.">`) {
		t.Fatalf("expected description meta tag, got %q", head)
	}
}

func TestPageMeta_HeadTags_NoCanonical_OmitsCanonicalAndOGURL(t *testing.T) {
	meta := seo.BuildPageMeta("LeafWiki", "Intro", "Body.", "")

	head := meta.HeadTags()

	if strings.Contains(head, "rel=\"canonical\"") || strings.Contains(head, "og:url") {
		t.Fatalf("expected no canonical/og:url tags without a canonical URL, got %q", head)
	}
}

func TestBuildPageMeta_DescriptionDerivedFromContent(t *testing.T) {
	meta := seo.BuildPageMeta("LeafWiki", "Intro", "# Intro\n\nThis is the body text.", "")
	if !strings.Contains(meta.Description, "This is the body text.") {
		t.Fatalf("expected description derived from markdown body, got %q", meta.Description)
	}
}
