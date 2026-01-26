package tree

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func findChildBySlug(t *testing.T, parent *PageNode, slug string) *PageNode {
	t.Helper()
	for _, ch := range parent.Children {
		if ch.Slug == slug {
			return ch
		}
	}
	t.Fatalf("child with slug %q not found under %q", slug, parent.Slug)
	return nil
}

func slugs(children []*PageNode) []string {
	out := make([]string, 0, len(children))
	for _, c := range children {
		out = append(out, c.Slug)
	}
	return out
}

// --- tests ---

func TestNodeStore_ReconstructTreeFromFS_EmptyStorage_ReturnsRoot(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	if tree == nil || tree.ID != "root" || tree.Kind != NodeKindSection {
		t.Fatalf("unexpected root: %#v", tree)
	}
	if tree.Parent != nil {
		t.Fatalf("expected root parent nil")
	}
	if len(tree.Children) != 0 {
		t.Fatalf("expected root to have no children, got %d", len(tree.Children))
	}
}

func TestNodeStore_ReconstructTreeFromFS_BuildsSectionsAndPages_SkipsIndexMdAsPage(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// FS layout:
	// <tmp>/docs/index.md (section content)
	// <tmp>/docs/intro.md (page)
	// <tmp>/readme.md (page at root)
	mustMkdir(t, filepath.Join(tmp, "root", "docs"))

	secIndex := `---
leafwiki_id: sec-docs
leafwiki_title: Documentation
---
# Section`
	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "index.md"), secIndex, 0o644)

	pageIntro := `---
leafwiki_id: page-intro
leafwiki_title: Introduction
---
# Intro`
	mustWriteFile(t, filepath.Join(tmp, "root", "docs", "intro.md"), pageIntro, 0o644)

	rootPage := `---
leafwiki_id: page-readme
leafwiki_title: Readme
---
# Readme`
	mustWriteFile(t, filepath.Join(tmp, "root", "readme.md"), rootPage, 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// root has: docs(section), readme(page)
	docs := findChildBySlug(t, tree, "docs")
	if docs.Kind != NodeKindSection {
		t.Fatalf("expected docs to be section, got %q", docs.Kind)
	}
	// section title/id from index frontmatter
	if docs.ID != "sec-docs" {
		t.Fatalf("expected docs.ID=sec-docs, got %q", docs.ID)
	}
	if docs.Title != "Documentation" {
		t.Fatalf("expected docs.Title=Documentation, got %q", docs.Title)
	}

	// ensure index.md wasn't turned into a page child
	for _, ch := range docs.Children {
		if ch.Slug == "index" {
			t.Fatalf("index.md must be skipped as page, but found slug index")
		}
	}

	intro := findChildBySlug(t, docs, "intro")
	if intro.Kind != NodeKindPage {
		t.Fatalf("expected intro to be page, got %q", intro.Kind)
	}
	// page title/id from frontmatter
	if intro.ID != "page-intro" {
		t.Fatalf("expected intro.ID=page-intro, got %q (BUG: your current code sets ID=\"\")", intro.ID)
	}
	if intro.Title != "Introduction" {
		t.Fatalf("expected intro.Title=Introduction, got %q", intro.Title)
	}

	readme := findChildBySlug(t, tree, "readme")
	if readme.Kind != NodeKindPage {
		t.Fatalf("expected readme to be page, got %q", readme.Kind)
	}
	if readme.ID != "page-readme" {
		t.Fatalf("expected readme.ID=page-readme, got %q", readme.ID)
	}
	if readme.Title != "Readme" {
		t.Fatalf("expected readme.Title=Readme, got %q", readme.Title)
	}

	// parent pointers
	if docs.Parent == nil || docs.Parent.ID != "root" {
		t.Fatalf("expected docs parent root, got %#v", docs.Parent)
	}
	if intro.Parent == nil || intro.Parent.ID != docs.ID {
		t.Fatalf("expected intro parent docs, got %#v", intro.Parent)
	}
}

func TestNodeStore_ReconstructTreeFromFS_SectionWithoutIndex_UsesDirNameAsTitle(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// FS: <tmp>/emptysec/ (no index.md)
	mustMkdir(t, filepath.Join(tmp, "root", "emptysec"))

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	sec := findChildBySlug(t, tree, "emptysec")
	if sec.Kind != NodeKindSection {
		t.Fatalf("expected section, got %q", sec.Kind)
	}
	// title defaults to folder name (per your code)
	if sec.Title != "emptysec" {
		t.Fatalf("expected title=emptysec, got %q", sec.Title)
	}
	if strings.TrimSpace(sec.ID) == "" {
		t.Fatalf("expected some generated id, got empty")
	}
}

func TestNodeStore_ReconstructTreeFromFS_PageWithoutFrontmatter_FallsBackToHeadlineTitle(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// FS: <tmp>/plain.md (no fm)
	mustWriteFile(t, filepath.Join(tmp, "root", "plain.md"), "# hello\n", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	p := findChildBySlug(t, tree, "plain")
	if p.Kind != NodeKindPage {
		t.Fatalf("expected page, got %q", p.Kind)
	}

	// title fallback should be headline
	if p.Title != "hello" {
		t.Fatalf("expected title fallback to slug 'plain', got %q", p.Title)
	}
	if strings.TrimSpace(p.ID) == "" {
		// should still have generated id (unless you later decide to keep empty)
		t.Fatalf("expected generated id, got empty")
	}
}

func TestNodeStore_ReconstructTreeFromFS_PositionsAreContiguous(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// Create several files/dirs
	mustWriteFile(t, filepath.Join(tmp, "root", "b.md"), "# b", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "a.md"), "# a", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "zsec"))

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// Positions should be 0..n-1 regardless of order
	seen := make([]int, 0, len(tree.Children))
	for _, ch := range tree.Children {
		seen = append(seen, ch.Position)
	}
	sort.Ints(seen)
	for i := range seen {
		if seen[i] != i {
			t.Fatalf("expected contiguous positions 0..%d, got %v (slugs=%v)", len(seen)-1, seen, slugs(tree.Children))
		}
	}
}

func TestNodeStore_ReconstructTreeFromFS_SkipsInvalidSlugs(t *testing.T) {
	tmp := t.TempDir()
	store := NewNodeStore(tmp)

	// Create directories and files with invalid slugs
	// Invalid directory names
	mustMkdir(t, filepath.Join(tmp, "root", "Invalid-Dir")) // uppercase
	mustWriteFile(t, filepath.Join(tmp, "root", "Invalid-Dir", "index.md"), "# Invalid Dir", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "dir_with_underscores")) // underscores
	mustWriteFile(t, filepath.Join(tmp, "root", "dir_with_underscores", "index.md"), "# Dir With Underscores", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "dir with spaces")) // spaces
	mustWriteFile(t, filepath.Join(tmp, "root", "dir with spaces", "index.md"), "# Dir With Spaces", 0o644)
	mustMkdir(t, filepath.Join(tmp, "root", "api")) // reserved slug
	mustWriteFile(t, filepath.Join(tmp, "root", "api", "index.md"), "# API", 0o644)
	
	// Invalid file names
	mustWriteFile(t, filepath.Join(tmp, "root", "Invalid-File.md"), "# Invalid File", 0o644) // uppercase
	mustWriteFile(t, filepath.Join(tmp, "root", "file_with_underscores.md"), "# File With Underscores", 0o644) // underscores
	mustWriteFile(t, filepath.Join(tmp, "root", "file with spaces.md"), "# File With Spaces", 0o644) // spaces
	mustWriteFile(t, filepath.Join(tmp, "root", "edit.md"), "# Edit", 0o644) // reserved slug
	
	// Valid slugs for comparison
	mustMkdir(t, filepath.Join(tmp, "root", "valid-dir"))
	mustWriteFile(t, filepath.Join(tmp, "root", "valid-dir", "index.md"), "# Valid Dir", 0o644)
	mustWriteFile(t, filepath.Join(tmp, "root", "valid-file.md"), "# Valid File", 0o644)

	tree, err := store.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	// Should only have the valid directory and file
	if len(tree.Children) != 2 {
		t.Fatalf("expected 2 children (valid-dir and valid-file), got %d: %v", len(tree.Children), slugs(tree.Children))
	}

	// Verify valid-dir exists
	validDir := findChildBySlug(t, tree, "valid-dir")
	if validDir.Kind != NodeKindSection {
		t.Fatalf("expected valid-dir to be section, got %q", validDir.Kind)
	}
	if validDir.Title != "Valid Dir" {
		t.Fatalf("expected valid-dir title 'Valid Dir', got %q", validDir.Title)
	}

	// Verify valid-file exists
	validFile := findChildBySlug(t, tree, "valid-file")
	if validFile.Kind != NodeKindPage {
		t.Fatalf("expected valid-file to be page, got %q", validFile.Kind)
	}
	if validFile.Title != "Valid File" {
		t.Fatalf("expected valid-file title 'Valid File', got %q", validFile.Title)
	}

	// Verify invalid slugs are not present
	invalidSlugs := []string{
		"Invalid-Dir", "dir_with_underscores", "dir with spaces", "api",
		"Invalid-File", "file_with_underscores", "file with spaces", "edit",
	}
	for _, invalidSlug := range invalidSlugs {
		for _, child := range tree.Children {
			if child.Slug == invalidSlug {
				t.Fatalf("found invalid slug %q in tree, should have been skipped", invalidSlug)
			}
		}
	}
}
