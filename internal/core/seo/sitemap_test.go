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
