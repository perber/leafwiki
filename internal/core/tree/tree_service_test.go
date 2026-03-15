package tree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/markdown"
)

// --- helpers ---

func newLoadedService(t *testing.T) (*TreeService, string) {
	t.Helper()

	tmpDir := t.TempDir()
	svc := NewTreeService(tmpDir)

	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	return svc, tmpDir
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected %q to exist, stat error: %v", path, err)
	}

	return info
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("expected %q to not exist, but it exists", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for %q, got: %v", path, err)
	}
}

func ptrKind(k NodeKind) *NodeKind { return &k }

// --- load basics ---

func TestTreeService_LoadTree_DefaultRootWhenMissing(t *testing.T) {
	svc, _ := newLoadedService(t)

	tree := svc.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Fatalf("expected default root, got: %+v", tree)
	}
	if tree.Kind != NodeKindSection {
		t.Fatalf("expected root to be section, got %q", tree.Kind)
	}
	if tree.Parent != nil {
		t.Fatalf("expected root parent nil")
	}
	if len(tree.Children) != 0 {
		t.Fatalf("expected root to have no children")
	}
}

// --- create / update / delete ---

func TestTreeService_CreateNode_PageAtRoot_CreatesFileAndFrontmatter(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	path := filepath.Join(tmpDir, "root", "welcome.md")
	mustStat(t, path)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	fm, _, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if strings.TrimSpace(fm.LeafWikiID) != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Welcome" {
		t.Fatalf("expected leafwiki_title=Welcome, got %q", fm.LeafWikiTitle)
	}
}

func TestTreeService_CreateChild_UnderPage_AutoConvertsParentToSection(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	parentID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	parentFile := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, parentFile)

	_, err = svc.CreateNode("system", parentID, "Getting Started", "getting-started", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create child failed: %v", err)
	}

	parentDir := filepath.Join(tmpDir, "root", "docs")
	mustStat(t, parentDir)
	mustStat(t, filepath.Join(parentDir, "index.md"))
	mustNotExist(t, parentFile)
	mustStat(t, filepath.Join(parentDir, "getting-started.md"))

	parentNode, err := svc.FindPageByID(*parentID)
	if err != nil {
		t.Fatalf("FindPageByID: %v", err)
	}
	if parentNode.Kind != NodeKindSection {
		t.Fatalf("expected parent kind section, got %q", parentNode.Kind)
	}
}

func TestTreeService_UpdateNode_TitleOnly_UpdatesFrontmatter(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	path := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, path)

	if err := svc.UpdateNode("system", *id, "Documentation", "docs", nil); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	fm, _, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if fm.LeafWikiTitle != "Documentation" {
		t.Fatalf("expected updated title, got %q", fm.LeafWikiTitle)
	}
}

func TestTreeService_UpdateNode_RenameSlug_RenamesOnDisk(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	oldPath := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, oldPath)

	if err := svc.UpdateNode("system", *id, "Docs", "documentation", nil); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	mustStat(t, filepath.Join(tmpDir, "root", "documentation.md"))
	mustNotExist(t, oldPath)
}

func TestTreeService_DeleteNode(t *testing.T) {
	t.Run("non-recursive errors when node has children", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		parentID, _ := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
		_, _ = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))

		err := svc.DeleteNode("system", *parentID, false)
		if !errors.Is(err, ErrPageHasChildren) {
			t.Fatalf("expected ErrPageHasChildren, got %v", err)
		}
	})

	t.Run("recursive delete removes folder and tree node", func(t *testing.T) {
		svc, tmpDir := newLoadedService(t)

		parentID, _ := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
		_, _ = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))

		parentDir := filepath.Join(tmpDir, "root", "parent")
		mustStat(t, parentDir)

		if err := svc.DeleteNode("system", *parentID, true); err != nil {
			t.Fatalf("DeleteNode recursive failed: %v", err)
		}

		mustNotExist(t, parentDir)

		if len(svc.GetTree().Children) != 0 {
			t.Fatalf("expected root to have no children")
		}
	})

	t.Run("invalid id returns not found", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		err := svc.DeleteNode("system", "does-not-exist", false)
		if !errors.Is(err, ErrPageNotFound) {
			t.Fatalf("expected ErrPageNotFound, got %v", err)
		}
	})

	t.Run("drift returns DriftError", func(t *testing.T) {
		svc, tmpDir := newLoadedService(t)

		id, err := svc.CreateNode("system", nil, "Ghost", "ghost", ptrKind(NodeKindPage))
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}

		path := filepath.Join(tmpDir, "root", "ghost.md")
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove file: %v", err)
		}

		err = svc.DeleteNode("system", *id, false)
		if err == nil {
			t.Fatalf("expected drift error")
		}

		var dErr *DriftError
		if !errors.As(err, &dErr) {
			t.Fatalf("expected DriftError, got %T (%v)", err, err)
		}
	})
}

// --- move ---

func TestTreeService_MoveNode(t *testing.T) {
	t.Run("target page auto converts to section", func(t *testing.T) {
		svc, tmpDir := newLoadedService(t)

		aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
		bID, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

		if err := svc.MoveNode("system", *aID, *bID); err != nil {
			t.Fatalf("MoveNode failed: %v", err)
		}

		bDir := filepath.Join(tmpDir, "root", "b")
		mustStat(t, bDir)
		mustStat(t, filepath.Join(bDir, "index.md"))
		mustStat(t, filepath.Join(bDir, "a.md"))
	})

	t.Run("prevents circular reference", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
		bID, _ := svc.CreateNode("system", aID, "B", "b", ptrKind(NodeKindPage))

		err := svc.MoveNode("system", *aID, *bID)
		if !errors.Is(err, ErrMovePageCircularReference) {
			t.Fatalf("expected ErrMovePageCircularReference, got %v", err)
		}
	})

	t.Run("prevents self parent", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))

		err := svc.MoveNode("system", *aID, *aID)
		if !errors.Is(err, ErrPageCannotBeMovedToItself) {
			t.Fatalf("expected ErrPageCannotBeMovedToItself, got %v", err)
		}
	})
}

// --- sorting ---

func TestTreeService_SortPages(t *testing.T) {
	t.Run("valid order", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		idA, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
		idB, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))
		idC, _ := svc.CreateNode("system", nil, "C", "c", ptrKind(NodeKindPage))

		if err := svc.SortPages("root", []string{*idC, *idA, *idB}); err != nil {
			t.Fatalf("SortPages failed: %v", err)
		}

		root := svc.GetTree()
		if root.Children[0].ID != *idC || root.Children[1].ID != *idA || root.Children[2].ID != *idB {
			t.Fatalf("unexpected order after sort")
		}
	})

	t.Run("invalid id returns invalid sort order", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		_, _ = svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
		_, _ = svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

		err := svc.SortPages("root", []string{"only-one"})
		if !errors.Is(err, ErrInvalidSortOrder) {
			t.Fatalf("expected ErrInvalidSortOrder, got %v", err)
		}
	})

	t.Run("duplicate id errors", func(t *testing.T) {
		svc, _ := newLoadedService(t)

		idA, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
		idB, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

		err := svc.SortPages("root", []string{*idA, *idA, *idB})
		if err == nil {
			t.Fatalf("expected error for duplicate ids")
		}
	})
}

// --- routing / lookup / ensure ---

func TestTreeService_FindPageByRoutePath_ReturnsContent(t *testing.T) {
	svc, _ := newLoadedService(t)

	archID, _ := svc.CreateNode("system", nil, "Architecture", "architecture", ptrKind(NodeKindPage))
	projectID, _ := svc.CreateNode("system", archID, "Project A", "project-a", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", projectID, "Specs", "specs", ptrKind(NodeKindPage))

	specsNode := svc.GetTree().Children[0].Children[0].Children[0]
	body := "# Specs\nHello"

	if err := svc.UpdateNode("system", specsNode.ID, "Specs", "specs", &body); err != nil {
		t.Fatalf("UpdateNode content failed: %v", err)
	}

	page, err := svc.FindPageByRoutePath(svc.GetTree().Children, "architecture/project-a/specs")
	if err != nil {
		t.Fatalf("FindPageByRoutePath failed: %v", err)
	}
	if page.Slug != "specs" {
		t.Fatalf("expected slug specs, got %q", page.Slug)
	}
	if !strings.Contains(page.Content, "Hello") {
		t.Fatalf("expected content to include Hello, got %q", page.Content)
	}
}

func TestTreeService_LookupPagePath_Segments(t *testing.T) {
	svc, _ := newLoadedService(t)

	homeID, _ := svc.CreateNode("system", nil, "Home", "home", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", homeID, "About", "about", ptrKind(NodeKindPage))

	lookup, err := svc.LookupPagePath(svc.GetTree().Children, "home/about/team")
	if err != nil {
		t.Fatalf("LookupPagePath failed: %v", err)
	}

	if lookup.Exists {
		t.Fatalf("expected full path to not exist")
	}
	if len(lookup.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(lookup.Segments))
	}
	if !lookup.Segments[0].Exists || lookup.Segments[0].ID == nil {
		t.Fatalf("expected home segment to exist")
	}
	if !lookup.Segments[1].Exists || lookup.Segments[1].ID == nil {
		t.Fatalf("expected about segment to exist")
	}
	if lookup.Segments[2].Exists || lookup.Segments[2].ID != nil {
		t.Fatalf("expected team to not exist")
	}
}

func TestTreeService_EnsurePagePath_CreatesIntermediateSectionsAndFinalPage(t *testing.T) {
	svc, _ := newLoadedService(t)

	res, err := svc.EnsurePagePath("system", "home/about/team/members", "Members", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("EnsurePagePath failed: %v", err)
	}

	if res.Page == nil || res.Page.Slug != "members" {
		t.Fatalf("expected final page 'members'")
	}

	lookup, err := svc.LookupPagePath(svc.GetTree().Children, "home/about/team/members")
	if err != nil {
		t.Fatalf("LookupPagePath failed: %v", err)
	}
	if !lookup.Exists {
		t.Fatalf("expected path to exist after EnsurePagePath")
	}
}

// --- reconstruct from filesystem ---

func TestTreeService_ReconstructTreeFromFS_LoadsProjectionFromDisk(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	mustWriteFile(t, filepath.Join(tmpDir, "root", "intro.md"), `---
leafwiki_id: intro
leafwiki_title: Introduction
---
# Introduction`, 0o644)

	mustMkdir(t, filepath.Join(tmpDir, "root", "docs"))
	mustWriteFile(t, filepath.Join(tmpDir, "root", "docs", "index.md"), `---
leafwiki_id: docs-section
leafwiki_title: Documentation
---
# Documentation`, 0o644)

	mustWriteFile(t, filepath.Join(tmpDir, "root", "docs", "getting-started.md"), `---
leafwiki_id: getting-started
leafwiki_title: Getting Started
---
# Getting Started`, 0o644)

	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	tree := svc.GetTree()

	intro := findChildBySlug(t, tree, "intro")
	if intro.Kind != NodeKindPage {
		t.Fatalf("expected intro to be page, got %q", intro.Kind)
	}

	docs := findChildBySlug(t, tree, "docs")
	if docs.Kind != NodeKindSection {
		t.Fatalf("expected docs to be section, got %q", docs.Kind)
	}
	if docs.ID != "docs-section" {
		t.Fatalf("expected docs ID docs-section, got %q", docs.ID)
	}

	gettingStarted := findChildBySlug(t, docs, "getting-started")
	if gettingStarted.Kind != NodeKindPage {
		t.Fatalf("expected getting-started to be page, got %q", gettingStarted.Kind)
	}
}

func TestTreeService_ReconstructTreeFromFS_EmptyDirectoryReturnsRoot(t *testing.T) {
	svc, _ := newLoadedService(t)

	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	tree := svc.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Fatalf("expected root node, got %+v", tree)
	}
	if len(tree.Children) != 0 {
		t.Fatalf("expected empty root")
	}
}
