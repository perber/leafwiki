package links

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

func TestResolveURLPath_PageFolderDefault(t *testing.T) {
	if activeRelativeLinkMode() != LinkModePageFolder {
		t.Fatalf("expected default mode page-folder, got %s", activeRelativeLinkMode())
	}

	cases := []struct{ current, href, want string }{
		{"/docs/guide", "setup", "/docs/guide/setup"},
		{"/docs/guide", "../other", "/docs/other"},
		{"/docs/guide", "./child/page", "/docs/guide/child/page"},
		{"/docs/guide", "/root-page", "/root-page"},
	}
	for _, c := range cases {
		got, err := resolveURLPath(c.current, c.href)
		if err != nil {
			t.Fatalf("resolveURLPath(%q, %q): %v", c.current, c.href, err)
		}
		if got != c.want {
			t.Errorf("resolveURLPath(%q, %q) = %q, want %q", c.current, c.href, got, c.want)
		}
	}
}

// resolveURLPath is the tree-less variant (refactor engine): in filesystem
// mode it always strips to the parent directory. Section-aware resolution
// (what the link index uses) is covered by TestLinkBaseDir below.
func TestResolveURLPath_FilesystemMode(t *testing.T) {
	SetRelativeLinkMode(LinkModeFilesystem)
	t.Cleanup(func() { SetRelativeLinkMode(LinkModePageFolder) })

	cases := []struct{ current, href, want string }{
		// a link to a sibling page/file resolves against the parent dir
		{"/docs/guide", "sibling", "/docs/sibling"},
		{"/docs/guide", "sibling/page", "/docs/sibling/page"},
		// parent-directory targets need one ../ less than page-folder semantics
		{"/docs/nested/guide", "../other", "/docs/other"},
		{"/docs/guide", "./same-dir", "/docs/same-dir"},
		// top-level pages resolve against the root
		{"/top", "x", "/x"},
		{"/top", "../x", "/x"},
		// absolute wiki paths are untouched by the mode
		{"/docs/guide", "/root-page", "/root-page"},
	}
	for _, c := range cases {
		got, err := resolveURLPath(c.current, c.href)
		if err != nil {
			t.Fatalf("resolveURLPath(%q, %q): %v", c.current, c.href, err)
		}
		if got != c.want {
			t.Errorf("resolveURLPath(%q, %q) = %q, want %q", c.current, c.href, got, c.want)
		}
	}
}

// The link index resolves against the tree, which knows section-ness:
// a section (index.md) page's directory is the section itself; leaf pages
// use their parent directory.
func TestLinkBaseDir_FilesystemSectionAware(t *testing.T) {
	SetRelativeLinkMode(LinkModeFilesystem)
	t.Cleanup(func() { SetRelativeLinkMode(LinkModePageFolder) })

	ts := tree.NewTreeService(t.TempDir())
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	// "docs" auto-converts to a section when it gains a child; "readme"
	// stays a standalone leaf page.
	docsIDPtr, err := ts.CreateNode("system", nil, "Docs", "docs", nil)
	if err != nil {
		t.Fatalf("CreateNode docs failed: %v", err)
	}
	docsID := *docsIDPtr
	if _, err := ts.CreateNode("system", &docsID, "Guide", "guide", nil); err != nil {
		t.Fatalf("CreateNode guide failed: %v", err)
	}
	if _, err := ts.CreateNode("system", nil, "Readme", "readme", nil); err != nil {
		t.Fatalf("CreateNode readme failed: %v", err)
	}

	cases := map[string]string{
		"/docs":       "/docs", // section page: its directory is the section itself
		"/docs/guide": "/docs", // leaf page: parent directory
		"/readme":     "/",     // top-level leaf: root
		"/missing":    "/",     // unknown page falls back to parent semantics
	}
	for current, want := range cases {
		if got := linkBaseDir(ts, current); got != want {
			t.Errorf("linkBaseDir(%q) = %q, want %q", current, got, want)
		}
	}
}

func TestLinkBaseDir_PageFolderIgnoresSectionNess(t *testing.T) {
	ts := tree.NewTreeService(t.TempDir())
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	docsIDPtr, err := ts.CreateNode("system", nil, "Docs", "docs", nil)
	if err != nil {
		t.Fatalf("CreateNode docs failed: %v", err)
	}
	docsID := *docsIDPtr
	if _, err := ts.CreateNode("system", &docsID, "Guide", "guide", nil); err != nil {
		t.Fatalf("CreateNode guide failed: %v", err)
	}

	// page-folder mode: the page path is the base, sections or not
	if got := linkBaseDir(ts, "/docs"); got != "/docs" {
		t.Errorf("linkBaseDir(/docs) = %q, want /docs", got)
	}
	if got := linkBaseDir(ts, "/docs/guide"); got != "/docs/guide" {
		t.Errorf("linkBaseDir(/docs/guide) = %q, want /docs/guide", got)
	}
}

func TestCanonicalPagePath(t *testing.T) {
	cases := map[string]string{
		// ".md" tails are ignored for lookup
		"docs/guide.md": "docs/guide",
		"docs/guide":    "docs/guide",
		// a trailing index segment addresses the section landing page
		"docs/accounts/index.md": "docs/accounts",
		"docs/accounts/index":    "docs/accounts",
		"docs/accounts":          "docs/accounts",
		// root index addresses the root page (not addressable by path)
		"index":    "",
		"index.md": "",
		// nested lookups keep deeper structure
		"a/b/c/index.md": "a/b/c",
	}
	for in, want := range cases {
		if got := canonicalPagePath(in); got != want {
			t.Errorf("canonicalPagePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSetRelativeLinkMode_UnknownFallsBack(t *testing.T) {
	SetRelativeLinkMode("bogus")
	t.Cleanup(func() { SetRelativeLinkMode(LinkModePageFolder) })

	if activeRelativeLinkMode() != LinkModePageFolder {
		t.Fatalf("expected unknown mode to fall back to page-folder, got %s", activeRelativeLinkMode())
	}
}
