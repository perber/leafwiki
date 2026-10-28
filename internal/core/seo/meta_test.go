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

func TestBuildPageMeta_DescriptionPrefersFrontmatter(t *testing.T) {
	raw := "---\ndescription: An author-written summary.\n---\n\n# Intro\n\nThis is the body text.\n"

	meta := seo.BuildPageMeta("LeafWiki", "Intro", raw, "")

	if meta.Description != "An author-written summary." {
		t.Fatalf("expected frontmatter description, got %q", meta.Description)
	}
}

// A frontmatter description is deliberate, so it is used in full rather than
// truncated to the excerpt length.
func TestBuildPageMeta_FrontmatterDescriptionIsNotTruncated(t *testing.T) {
	long := strings.Repeat("word ", 80)
	raw := "---\ndescription: " + long + "\n---\n\nBody.\n"

	meta := seo.BuildPageMeta("LeafWiki", "Intro", raw, "")

	if meta.Description != strings.TrimSpace(long) {
		t.Fatalf("expected the full frontmatter description, got %q", meta.Description)
	}
}

func TestBuildPageMeta_FallsBackToExcerptWhenFrontmatterHasNoDescription(t *testing.T) {
	cases := map[string]string{
		"no description key": "---\ntags: [a, b]\n---\n\nThis is the body text.\n",
		"empty description":  "---\ndescription: \"   \"\n---\n\nThis is the body text.\n",
		"non-string value":   "---\ndescription:\n  - a\n  - b\n---\n\nThis is the body text.\n",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			meta := seo.BuildPageMeta("LeafWiki", "Intro", raw, "")
			if !strings.Contains(meta.Description, "This is the body text.") {
				t.Fatalf("expected excerpt fallback, got %q", meta.Description)
			}
		})
	}
}

// Frontmatter must not leak into the excerpt fallback.
func TestBuildPageMeta_ExcerptSkipsFrontmatter(t *testing.T) {
	raw := "---\ntags: [secret-tag]\n---\n\nThis is the body text.\n"

	meta := seo.BuildPageMeta("LeafWiki", "Intro", raw, "")

	if strings.Contains(meta.Description, "secret-tag") {
		t.Fatalf("expected frontmatter to be excluded from the excerpt, got %q", meta.Description)
	}
}
