package seo_test

import (
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/seo"
	"github.com/perber/wiki/internal/core/tree"
)

func buildTestTree() *tree.PageNode {
	root := &tree.PageNode{ID: "root", Slug: "root", Kind: tree.NodeKindSection}

	docs := &tree.PageNode{ID: "docs", Slug: "docs", Kind: tree.NodeKindSection, Parent: root}
	root.Children = []*tree.PageNode{docs}

	intro := &tree.PageNode{
		ID: "intro", Slug: "intro", Kind: tree.NodeKindPage, Parent: docs,
		Metadata: tree.PageMetadata{UpdatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
	}
	advanced := &tree.PageNode{ID: "advanced", Slug: "advanced", Kind: tree.NodeKindPage, Parent: docs}
	docs.Children = []*tree.PageNode{intro, advanced}

	return root
}

func TestBuildSitemap_ListsPagesNotSections(t *testing.T) {
	xmlBytes, err := seo.BuildSitemap(buildTestTree(), "https://wiki.example.com")
	if err != nil {
		t.Fatalf("BuildSitemap returned error: %v", err)
	}
	out := string(xmlBytes)

	if !strings.Contains(out, "<loc>https://wiki.example.com/docs/intro</loc>") {
		t.Fatalf("expected intro page URL, got %q", out)
	}
	if !strings.Contains(out, "<loc>https://wiki.example.com/docs/advanced</loc>") {
		t.Fatalf("expected advanced page URL, got %q", out)
	}
	if strings.Contains(out, "/docs</loc>") {
		t.Fatalf("expected section node to be excluded from sitemap, got %q", out)
	}
}

func TestBuildSitemap_IncludesLastModWhenSet(t *testing.T) {
	xmlBytes, err := seo.BuildSitemap(buildTestTree(), "https://wiki.example.com")
	if err != nil {
		t.Fatalf("BuildSitemap returned error: %v", err)
	}
	out := string(xmlBytes)

	if !strings.Contains(out, "<lastmod>2026-01-02T03:04:05Z</lastmod>") {
		t.Fatalf("expected lastmod for intro page, got %q", out)
	}
}

// buildHomeFirstTree returns a tree whose first child is a page, making it the
// home page shown at the site root.
func buildHomeFirstTree() *tree.PageNode {
	root := &tree.PageNode{ID: "root", Slug: "root", Kind: tree.NodeKindSection}
	home := &tree.PageNode{
		ID: "home", Slug: "home", Kind: tree.NodeKindPage, Parent: root,
		Metadata: tree.PageMetadata{UpdatedAt: time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)},
	}
	other := &tree.PageNode{ID: "other", Slug: "other", Kind: tree.NodeKindPage, Parent: root}
	root.Children = []*tree.PageNode{home, other}
	return root
}

func TestHomeRoutePath_FirstChildPage(t *testing.T) {
	if got := seo.HomeRoutePath(buildHomeFirstTree()); got != "home" {
		t.Fatalf("expected home route path %q, got %q", "home", got)
	}
}

func TestHomeRoutePath_FirstChildSectionHasNoHomePage(t *testing.T) {
	// buildTestTree starts with a section, so the root shows no page of its own.
	if got := seo.HomeRoutePath(buildTestTree()); got != "" {
		t.Fatalf("expected no home route path, got %q", got)
	}
}

func TestHomeRoutePath_EmptyTree(t *testing.T) {
	if got := seo.HomeRoutePath(nil); got != "" {
		t.Fatalf("expected no home route path for a nil tree, got %q", got)
	}
	empty := &tree.PageNode{ID: "root", Slug: "root", Kind: tree.NodeKindSection}
	if got := seo.HomeRoutePath(empty); got != "" {
		t.Fatalf("expected no home route path for an empty tree, got %q", got)
	}
}

// The home page must be advertised at the root URL only - listing it under its
// own path too would contradict the canonical tag and split indexing signals.
func TestBuildSitemap_ListsHomePageAtRootURLOnly(t *testing.T) {
	xmlBytes, err := seo.BuildSitemap(buildHomeFirstTree(), "https://wiki.example.com")
	if err != nil {
		t.Fatalf("BuildSitemap returned error: %v", err)
	}
	out := string(xmlBytes)

	if !strings.Contains(out, "<loc>https://wiki.example.com/</loc>") {
		t.Fatalf("expected home page listed at the root URL, got %q", out)
	}
	if strings.Contains(out, "<loc>https://wiki.example.com/home</loc>") {
		t.Fatalf("expected home page not listed under its own path, got %q", out)
	}
	if !strings.Contains(out, "<loc>https://wiki.example.com/other</loc>") {
		t.Fatalf("expected non-home pages listed under their own paths, got %q", out)
	}
	if !strings.Contains(out, "<lastmod>2026-05-06T07:08:09Z</lastmod>") {
		t.Fatalf("expected the home page's lastmod on the root URL, got %q", out)
	}
}

func TestBuildSitemap_TrimsTrailingSlashFromBaseURL(t *testing.T) {
	xmlBytes, err := seo.BuildSitemap(buildTestTree(), "https://wiki.example.com/")
	if err != nil {
		t.Fatalf("BuildSitemap returned error: %v", err)
	}
	out := string(xmlBytes)

	if strings.Contains(out, "example.com//docs") {
		t.Fatalf("expected no double slash in URL, got %q", out)
	}
}
