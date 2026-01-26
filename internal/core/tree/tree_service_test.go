package tree

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/frontmatter"
)

// --- helpers ---

func newLoadedService(t *testing.T) (*TreeService, string) {
	t.Helper()
	tmpDir := t.TempDir()

	// Ensure schema is current so LoadTree doesn't try to migrate unless a test wants it.
	if err := saveSchema(tmpDir, CurrentSchemaVersion); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

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

// --- A) Load/Save basics ---

func TestTreeService_LoadTree_DefaultRootWhenMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// schema current to prevent migration from failing due to missing schema file
	if err := saveSchema(tmpDir, CurrentSchemaVersion); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	tree := svc.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Fatalf("expected default root, got: %+v", tree)
	}
	if tree.Kind != NodeKindSection {
		t.Fatalf("expected root to be section, got %q", tree.Kind)
	}
}

func TestTreeService_SaveAndLoad_RoundtripParents(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create a small tree through public API (exercises disk + tree)
	idA, err := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	_, err = svc.CreateNode("system", idA, "B", "b", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}

	if err := svc.SaveTree(); err != nil {
		t.Fatalf("SaveTree failed: %v", err)
	}

	// Reload in a new service instance
	if err := saveSchema(tmpDir, CurrentSchemaVersion); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}
	loaded := NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	root := loaded.GetTree()
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child at root, got %d", len(root.Children))
	}
	a := root.Children[0]
	if a.Parent == nil || a.Parent.ID != "root" {
		t.Fatalf("expected parent pointer on A")
	}
	if len(a.Children) != 1 {
		t.Fatalf("expected A to have 1 child, got %d", len(a.Children))
	}
	b := a.Children[0]
	if b.Parent == nil || b.Parent.ID != a.ID {
		t.Fatalf("expected parent pointer on B")
	}
}

// --- B) Create/Update/Delete disk sync ---

func TestTreeService_CreateNode_Page_Root_CreatesFileAndFrontmatter(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// file path: <tmp>/root/welcome.md (based on your existing tests + GeneratePath convention)
	p := filepath.Join(tmpDir, "root", "welcome.md")
	mustStat(t, p)

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	fm, _, has, err := frontmatter.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter to exist")
	}
	if strings.TrimSpace(fm.LeafWikiID) != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
}

func TestTreeService_CreateChild_UnderPage_AutoConvertsParentToSection(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create parent as page
	parentID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}

	// Should exist as file initially
	parentFile := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, parentFile)

	// Create child under parent: must convert parent to section
	_, err = svc.CreateNode("system", parentID, "Getting Started", "getting-started", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create child failed: %v", err)
	}

	// Parent should now be a folder with index.md (converted from docs.md)
	parentDir := filepath.Join(tmpDir, "root", "docs")
	mustStat(t, parentDir)
	index := filepath.Join(parentDir, "index.md")
	mustStat(t, index)

	// Old file should be gone
	mustNotExist(t, parentFile)

	// Child file should be inside folder
	childFile := filepath.Join(parentDir, "getting-started.md")
	mustStat(t, childFile)

	// Tree kind updated
	parentNode, err := svc.FindPageByID(svc.GetTree().Children, *parentID)
	if err != nil {
		t.Fatalf("FindPageByID: %v", err)
	}
	if parentNode.Kind != NodeKindSection {
		t.Fatalf("expected parent kind section, got %q", parentNode.Kind)
	}
}

func TestTreeService_UpdateNode_TitleOnly_SyncsFrontmatterIfFileExists(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	p := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, p)

	// Update title only: content=nil, slug unchanged
	if err := svc.UpdateNode("system", *id, "Documentation", "docs", nil); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	fm, _, has, err := frontmatter.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter")
	}
	if fm.LeafWikiTitle != "Documentation" {
		t.Fatalf("expected leafwiki_title to be updated, got %q", fm.LeafWikiTitle)
	}
}

func TestTreeService_UpdateNode_SlugRename_RenamesOnDisk(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	oldPath := filepath.Join(tmpDir, "root", "docs.md")
	mustStat(t, oldPath)

	newSlug := "documentation"
	if err := svc.UpdateNode("system", *id, "Docs", newSlug, nil); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}

	newPath := filepath.Join(tmpDir, "root", newSlug+".md")
	mustStat(t, newPath)
	mustNotExist(t, oldPath)
}

/*
Disable this test for now as we are not enforcing to pass the kinds yet.
func TestTreeService_UpdateNode_SectionToPage_DisallowedWithChildren(t *testing.T) {
	svc, _ := newLoadedService(t)

	// Create parent page, then child to force parent to section
	parentID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create parent failed: %v", err)
	}
	_, err = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("Create child failed: %v", err)
	}

	// Now parent is section with children, attempt to convert back to page
	err = svc.UpdateNode("system", *parentID, "Docs", "docs", nil)
	if err == nil {
		t.Fatalf("expected error converting section->page with children")
	}
	if !errors.Is(err, ErrPageHasChildren) {
		t.Fatalf("expected ErrPageHasChildren, got: %v", err)
	}
}
*/

func TestTreeService_DeleteNode_NonRecursiveErrorsWhenHasChildren(t *testing.T) {
	svc, _ := newLoadedService(t)

	parentID, _ := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))

	err := svc.DeleteNode("system", *parentID, false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrPageHasChildren) {
		t.Fatalf("expected ErrPageHasChildren, got: %v", err)
	}
}

func TestTreeService_DeleteNode_RecursiveDeletesDiskAndTree(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	parentID, _ := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))

	// Parent should now be a folder
	parentDir := filepath.Join(tmpDir, "root", "parent")
	mustStat(t, parentDir)

	err := svc.DeleteNode("system", *parentID, true)
	if err != nil {
		t.Fatalf("DeleteNode recursive failed: %v", err)
	}

	// Folder should be gone
	mustNotExist(t, parentDir)

	// Tree should have no children at root
	if len(svc.GetTree().Children) != 0 {
		t.Fatalf("expected root to have no children")
	}
}

func TestTreeService_DeletePage_Leaf_Success_RemovesFileAndTreeAndReindexes(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create 3 leaf pages
	idA, err := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode A: %v", err)
	}
	idB, err := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode B: %v", err)
	}
	idC, err := svc.CreateNode("system", nil, "C", "c", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode C: %v", err)
	}

	// Verify files exist
	pathA := filepath.Join(tmpDir, "root", "a.md")
	pathB := filepath.Join(tmpDir, "root", "b.md")
	pathC := filepath.Join(tmpDir, "root", "c.md")
	if _, err := os.Stat(pathB); err != nil {
		t.Fatalf("expected %s exists: %v", pathB, err)
	}

	// Delete middle page (B)
	if err := svc.DeleteNode("system", *idB, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	// Disk: B gone; A/C still there
	if _, err := os.Stat(pathB); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected %s to be deleted, got err=%v", pathB, err)
	}
	if _, err := os.Stat(pathA); err != nil {
		t.Fatalf("expected %s exists: %v", pathA, err)
	}
	if _, err := os.Stat(pathC); err != nil {
		t.Fatalf("expected %s exists: %v", pathC, err)
	}

	// Tree: only 2 children remain
	root := svc.GetTree()
	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children after delete, got %d", len(root.Children))
	}

	// Ensure deleted ID not present
	for _, ch := range root.Children {
		if ch.ID == *idB {
			t.Fatalf("deleted node still present in tree")
		}
	}

	// Reindex: positions must be 0..1 (order depends on previous positions; we just assert contiguous)
	if root.Children[0].Position != 0 || root.Children[1].Position != 1 {
		t.Fatalf("expected positions reindexed to 0..1, got %d,%d",
			root.Children[0].Position, root.Children[1].Position)
	}

	// Optional: ensure remaining IDs are the ones we expect
	_ = idA
	_ = idC
}

func TestTreeService_DeletePage_WithChildren_NonRecursive_ReturnsErrPageHasChildren(t *testing.T) {
	svc, _ := newLoadedService(t)

	parentID, err := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode parent: %v", err)
	}

	_, err = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode child: %v", err)
	}

	err = svc.DeleteNode("system", *parentID, false)
	if err == nil {
		t.Fatalf("expected error deleting page with children without recursive")
	}
	if !errors.Is(err, ErrPageHasChildren) {
		t.Fatalf("expected ErrPageHasChildren, got: %v", err)
	}
}

func TestTreeService_DeletePage_WithChildren_Recursive_DeletesFolder(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	parentID, err := svc.CreateNode("system", nil, "Parent", "parent", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode parent: %v", err)
	}
	_, err = svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode child: %v", err)
	}

	// Parent was auto-converted to section -> folder should exist
	parentDir := filepath.Join(tmpDir, "root", "parent")
	if _, err := os.Stat(parentDir); err != nil {
		t.Fatalf("expected parent dir exists (after auto-convert): %v", err)
	}

	// Recursive delete should remove the folder
	if err := svc.DeleteNode("system", *parentID, true); err != nil {
		t.Fatalf("DeleteNode recursive failed: %v", err)
	}

	if _, err := os.Stat(parentDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected parent folder deleted, got err=%v", err)
	}

	// Tree should no longer contain parent
	if len(svc.GetTree().Children) != 0 {
		t.Fatalf("expected root to have no children after delete, got %d", len(svc.GetTree().Children))
	}
}

func TestTreeService_DeletePage_InvalidID_ReturnsErrPageNotFound(t *testing.T) {
	svc, _ := newLoadedService(t)

	err := svc.DeleteNode("system", "does-not-exist", false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, ErrPageNotFound) {
		t.Fatalf("expected ErrPageNotFound, got: %v", err)
	}
}

func TestTreeService_DeletePage_Drift_FileMissing_ReturnsError(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create a leaf page normally (creates file)
	id, err := svc.CreateNode("system", nil, "Ghost", "ghost", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Delete the file manually to simulate drift
	p := filepath.Join(tmpDir, "root", "ghost.md")
	if err := os.Remove(p); err != nil {
		t.Fatalf("failed to remove file to simulate drift: %v", err)
	}

	// Now delete node - should error (drift)
	err = svc.DeleteNode("system", *id, false)
	if err == nil {
		t.Fatalf("expected drift error")
	}
	// If you have a concrete DriftError type, you can assert with errors.As.
	var dErr *DriftError
	if !errors.As(err, &dErr) {
		t.Fatalf("expected DriftError, got: %T (%v)", err, err)
	}
}

// --- C) Move semantics ---

func TestTreeService_MoveNode_TargetPageAutoConvertsToSection(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	bID, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

	// Move A under B (B is a page => should auto-convert to section)
	if err := svc.MoveNode("system", *aID, *bID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}

	// B should now be folder with index.md
	bDir := filepath.Join(tmpDir, "root", "b")
	mustStat(t, bDir)
	mustStat(t, filepath.Join(bDir, "index.md"))

	// A should now be inside B folder
	aPath := filepath.Join(bDir, "a.md")
	mustStat(t, aPath)
}

func TestTreeService_MoveNode_PreventsCircularReference(t *testing.T) {
	svc, _ := newLoadedService(t)

	aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	// create child under A so A becomes section and has child
	bID, _ := svc.CreateNode("system", aID, "B", "b", ptrKind(NodeKindPage))

	// Try move A under B (A -> ... -> B). Should error with circular reference.
	err := svc.MoveNode("system", *aID, *bID)
	if err == nil {
		t.Fatalf("expected error moving node under its descendant")
	}
	if !errors.Is(err, ErrMovePageCircularReference) {
		t.Fatalf("expected ErrMovePageCircularReference, got: %v", err)
	}
}

func TestTreeService_MoveNode_PreventsSelfParent(t *testing.T) {
	svc, _ := newLoadedService(t)

	aID, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))

	err := svc.MoveNode("system", *aID, *aID)
	if err == nil {
		t.Fatalf("expected error moving node into itself")
	}
	if !errors.Is(err, ErrPageCannotBeMovedToItself) {
		t.Fatalf("expected ErrPageCannotBeMovedToItself, got: %v", err)
	}
}

// --- D) SortPages ---

func TestTreeService_SortPages_ValidOrder(t *testing.T) {
	svc, _ := newLoadedService(t)

	idA, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	idB, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))
	idC, _ := svc.CreateNode("system", nil, "C", "c", ptrKind(NodeKindPage))

	err := svc.SortPages("root", []string{*idC, *idA, *idB})
	if err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}

	root := svc.GetTree()
	if root.Children[0].ID != *idC || root.Children[1].ID != *idA || root.Children[2].ID != *idB {
		t.Fatalf("unexpected order after sort")
	}
	if root.Children[0].Position != 0 || root.Children[1].Position != 1 || root.Children[2].Position != 2 {
		t.Fatalf("expected positions to be reindexed")
	}
}

func TestTreeService_SortPages_InvalidLength(t *testing.T) {
	svc, _ := newLoadedService(t)

	_, _ = svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

	err := svc.SortPages("root", []string{"only-one"})
	if err == nil {
		t.Fatalf("expected error for invalid length")
	}
	if !errors.Is(err, ErrInvalidSortOrder) {
		t.Fatalf("expected ErrInvalidSortOrder, got: %v", err)
	}
}

func TestTreeService_SortPages_DuplicateID(t *testing.T) {
	svc, _ := newLoadedService(t)

	idA, _ := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	idB, _ := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))

	err := svc.SortPages("root", []string{*idA, *idA, *idB})
	if err == nil {
		t.Fatalf("expected error for duplicate IDs")
	}
}

// --- E) Routing, Lookup, Ensure ---

func TestTreeService_FindPageByRoutePath_ReturnsContent(t *testing.T) {
	svc, _ := newLoadedService(t)

	archID, _ := svc.CreateNode("system", nil, "Architecture", "architecture", ptrKind(NodeKindPage))
	// create child -> converts arch to section
	projectID, _ := svc.CreateNode("system", archID, "Project A", "project-a", ptrKind(NodeKindPage))
	_, _ = svc.CreateNode("system", projectID, "Specs", "specs", ptrKind(NodeKindPage))

	// Update specs content
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
		t.Fatalf("expected content to include Hello, got: %q", page.Content)
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
		t.Fatalf("expected home segment to exist with ID")
	}
	if !lookup.Segments[1].Exists || lookup.Segments[1].ID == nil {
		t.Fatalf("expected about segment to exist with ID")
	}
	if lookup.Segments[2].Exists || lookup.Segments[2].ID != nil {
		t.Fatalf("expected team to not exist")
	}
}

func TestTreeService_EnsurePagePath_CreatesIntermediateSectionsAndFinalPage(t *testing.T) {
	svc, _ := newLoadedService(t)

	// Ensure a deep path; intermediate nodes should become sections
	res, err := svc.EnsurePagePath("system", "home/about/team/members", "Members", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("EnsurePagePath failed: %v", err)
	}
	if res.Page == nil || res.Page.Slug != "members" {
		t.Fatalf("expected final page 'members'")
	}

	// home/about/team should exist as path now
	lookup, err := svc.LookupPagePath(svc.GetTree().Children, "home/about/team/members")
	if err != nil {
		t.Fatalf("LookupPagePath failed: %v", err)
	}
	if !lookup.Exists {
		t.Fatalf("expected path to exist after EnsurePagePath")
	}
}

// --- F) Migration V2 (frontmatter backfill) ---
func TestTreeService_LoadTree_MigratesToV2_AddsFrontmatterAndPreservesBody(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()

	// start on v1 (or generally: current-1)
	if err := saveSchema(tmpDir, CurrentSchemaVersion-1); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Page1", "page1", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// IMPORTANT: persist tree so the next service instance sees the node
	if err := svc.SaveTree(); err != nil {
		t.Fatalf("SaveTree failed: %v", err)
	}

	// overwrite file without FM
	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	body := "# Page 1 Content\nHello World\n"
	if err := os.WriteFile(pagePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write old content failed: %v", err)
	}

	// force schema old again
	if err := saveSchema(tmpDir, CurrentSchemaVersion-1); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	loaded := NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("read migrated file: %v", err)
	}

	fm, migratedBody, has, err := frontmatter.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration, got:\n%s", string(raw))
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if strings.TrimSpace(fm.LeafWikiTitle) == "" {
		t.Fatalf("expected leafwiki_title to be set")
	}
	if migratedBody != body {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, body)
	}
}

// --- small util ---

func ptrKind(k NodeKind) *NodeKind { return &k }
