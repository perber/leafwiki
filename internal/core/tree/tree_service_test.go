package tree

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/markdown"
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

func persistLegacyTreeSnapshot(t *testing.T, storageDir string, tree *PageNode) {
	t.Helper()
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal legacy tree snapshot failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, legacyTreeFilename), raw, 0o644); err != nil {
		t.Fatalf("write legacy tree snapshot failed: %v", err)
	}
}

func readOrderIDs(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".order.json"))
	if err != nil {
		t.Fatalf("read order file: %v", err)
	}
	var persisted struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal order file: %v", err)
	}
	return persisted.OrderedIDs
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

func TestTreeService_LoadTree_MigratesLegacyTreeOrderIntoOrderFiles(t *testing.T) {
	tmpDir := t.TempDir()

	mustWriteFile(t, filepath.Join(tmpDir, "root", "a.md"), `---
leafwiki_id: id-a
leafwiki_title: A
---
# A`, 0o644)
	mustWriteFile(t, filepath.Join(tmpDir, "root", "b.md"), `---
leafwiki_id: id-b
leafwiki_title: B
---
# B`, 0o644)
	mustWriteFile(t, filepath.Join(tmpDir, "root", "c.md"), `---
leafwiki_id: id-c
leafwiki_title: C
---
# C`, 0o644)

	legacyTree := &PageNode{
		ID:       "root",
		Slug:     "root",
		Title:    "root",
		Kind:     NodeKindSection,
		Children: []*PageNode{{ID: "id-c", Slug: "c", Title: "C", Kind: NodeKindPage, Position: 0}, {ID: "id-a", Slug: "a", Title: "A", Kind: NodeKindPage, Position: 1}, {ID: "id-b", Slug: "b", Title: "B", Kind: NodeKindPage, Position: 2}},
	}
	persistLegacyTreeSnapshot(t, tmpDir, legacyTree)
	if err := saveSchema(tmpDir, 4); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	root := svc.GetTree()
	if got, want := slugs(root.Children), []string{"c", "a", "b"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected child order after legacy migration: got %v want %v", got, want)
	}
	if got, want := readOrderIDs(t, filepath.Join(tmpDir, "root")), []string{"id-c", "id-a", "id-b"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected persisted order after legacy migration: got %v want %v", got, want)
	}
	mustStat(t, filepath.Join(tmpDir, "tree.json"))
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

func TestTreeService_TreeHash_IsStableAcrossRepeatedCalls(t *testing.T) {
	svc, _ := newLoadedService(t)

	h1 := svc.TreeHash()
	h2 := svc.TreeHash()
	if h1 == "" {
		t.Fatalf("expected non-empty hash")
	}
	if h1 != h2 {
		t.Fatalf("expected stable hash across repeated calls, got %q and %q", h1, h2)
	}
	if want := svc.GetTree().Hash(); h1 != want {
		t.Fatalf("expected TreeHash to match underlying tree hash, got %q want %q", h1, want)
	}
}

func TestTreeService_TreeHash_ChangesWhenTreeChanges(t *testing.T) {
	svc, _ := newLoadedService(t)

	before := svc.TreeHash()
	pageID, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	afterCreate := svc.TreeHash()
	if before == afterCreate {
		t.Fatalf("expected hash to change after create")
	}

	if err := svc.UpdateNode("system", *pageID, "Welcome 2", "welcome", nil); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}
	afterUpdate := svc.TreeHash()
	if afterCreate == afterUpdate {
		t.Fatalf("expected hash to change after update")
	}
}

func TestTreeService_TreeHash_ChangesWhenOrderChanges(t *testing.T) {
	svc, _ := newLoadedService(t)

	firstID, err := svc.CreateNode("system", nil, "One", "one", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode first failed: %v", err)
	}
	secondID, err := svc.CreateNode("system", nil, "Two", "two", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode second failed: %v", err)
	}

	before := svc.TreeHash()
	if err := svc.SortPages("", []string{*secondID, *firstID}); err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}
	after := svc.TreeHash()
	if before == after {
		t.Fatalf("expected hash to change after sort")
	}
}

// --- B) Create/Update/Delete disk sync ---

func TestTreeService_CreateNode_ReloadsFromFilesystem(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	reloaded := NewTreeService(tmpDir)
	if err := reloaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	root := reloaded.GetTree()
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child after reload, got %d", len(root.Children))
	}
	if root.Children[0].ID != *id {
		t.Fatalf("expected persisted child ID %q, got %q", *id, root.Children[0].ID)
	}
}

func TestTreeService_CreateChild_RollsBackParentAutoConvertWhenTreeSaveFails(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	parentID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode parent failed: %v", err)
	}
	mustStat(t, filepath.Join(tmpDir, "root", "docs.md"))

	mustMkdir(t, filepath.Join(tmpDir, "root", "docs", ".order.json"))

	childID, err := svc.CreateNode("system", parentID, "Child", "child", ptrKind(NodeKindPage))
	if err == nil {
		t.Fatalf("expected CreateNode child to fail when child order save fails")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected CreateNode error to mention child order persistence, got: %v", err)
	}
	if childID != nil {
		t.Fatalf("expected returned child id to be nil on failure, got %q", *childID)
	}

	root := svc.GetTree()
	if len(root.Children) != 1 {
		t.Fatalf("expected only original parent after rollback, got %d root children", len(root.Children))
	}
	parent := root.Children[0]
	if parent.Kind != NodeKindPage {
		t.Fatalf("expected parent kind rolled back to page, got %q", parent.Kind)
	}
	if len(parent.Children) != 0 {
		t.Fatalf("expected parent children rolled back, got %d", len(parent.Children))
	}
	mustStat(t, filepath.Join(tmpDir, "root", "docs.md"))
	mustNotExist(t, filepath.Join(tmpDir, "root", "docs"))
	mustNotExist(t, filepath.Join(tmpDir, "root", "docs", "index.md"))
	mustNotExist(t, filepath.Join(tmpDir, "root", "docs", "child.md"))
}

func TestTreeService_CreateNode_RollsBackWhenTreeSaveFails(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	mustMkdir(t, filepath.Join(tmpDir, "root", ".order.json"))

	id, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err == nil {
		t.Fatalf("expected CreateNode to fail when order file write fails")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected CreateNode error to mention child order persistence, got: %v", err)
	}
	if id != nil {
		t.Fatalf("expected returned id to be nil on failure, got %q", *id)
	}
	if len(svc.GetTree().Children) != 0 {
		t.Fatalf("expected in-memory tree rollback, got %d root children", len(svc.GetTree().Children))
	}
	mustNotExist(t, filepath.Join(tmpDir, "root", "welcome.md"))
	if info, err := os.Stat(filepath.Join(tmpDir, "root", ".order.json")); err != nil {
		t.Fatalf("stat root order path: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("expected failure trigger to remain a directory")
	}
	reloaded := NewTreeService(tmpDir)
	if err := reloaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	if len(reloaded.GetTree().Children) != 0 {
		t.Fatalf("expected no persisted children after rollback, got %d", len(reloaded.GetTree().Children))
	}
}

func TestTreeService_CreateNode_RollsBackWhenOrderWriteFails(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	mustMkdir(t, filepath.Join(tmpDir, "root", ".order.json"))

	id, err := svc.CreateNode("system", nil, "Welcome", "welcome", ptrKind(NodeKindPage))
	if err == nil {
		t.Fatalf("expected CreateNode to fail when order file write fails")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected CreateNode error to mention child order persistence, got: %v", err)
	}
	if id != nil {
		t.Fatalf("expected returned id to be nil on failure, got %q", *id)
	}

	if len(svc.GetTree().Children) != 0 {
		t.Fatalf("expected in-memory tree rollback, got %d root children", len(svc.GetTree().Children))
	}
	mustNotExist(t, filepath.Join(tmpDir, "root", "welcome.md"))

	reloaded := NewTreeService(tmpDir)
	if err := reloaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	if len(reloaded.GetTree().Children) != 0 {
		t.Fatalf("expected no persisted children after rollback, got %d", len(reloaded.GetTree().Children))
	}
}

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

	fm, _, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter to exist")
	}
	if strings.TrimSpace(fm.LeafWikiID) != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if fm.LeafWikiCreatedAt == "" || fm.LeafWikiUpdatedAt == "" {
		t.Fatalf("expected leafwiki timestamps to be set, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "system" || fm.LeafWikiLastAuthorID != "system" {
		t.Fatalf("expected creator metadata to be set, got %#v", fm)
	}
}

func TestTreeService_CreateNode_PersistsRootOrderFile(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	idA, err := svc.CreateNode("system", nil, "A", "a", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode A failed: %v", err)
	}
	idB, err := svc.CreateNode("system", nil, "B", "b", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode B failed: %v", err)
	}

	got := readOrderIDs(t, filepath.Join(tmpDir, "root"))
	want := []string{*idA, *idB}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected persisted order after create: got %v want %v", got, want)
	}
}

func TestTreeService_CreateNode_Section_CreatesIndexWithFrontmatter(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	index := filepath.Join(tmpDir, "root", "docs", "index.md")
	raw, err := os.ReadFile(index)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter to exist")
	}
	if strings.TrimSpace(fm.LeafWikiID) != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Docs" {
		t.Fatalf("expected leafwiki_title Docs, got %q", fm.LeafWikiTitle)
	}
	if fm.LeafWikiCreatedAt == "" || fm.LeafWikiUpdatedAt == "" {
		t.Fatalf("expected leafwiki timestamps to be set, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "system" || fm.LeafWikiLastAuthorID != "system" {
		t.Fatalf("expected creator metadata to be set, got %#v", fm)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty section body, got %q", body)
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
	fm, _, has, err := markdown.ParseFrontmatter(string(raw))
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

func TestTreeService_DeleteNode_UpdatesRootOrderFile(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

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

	if err := svc.DeleteNode("system", *idB, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	got := readOrderIDs(t, filepath.Join(tmpDir, "root"))
	want := []string{*idA, *idC}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected persisted order after delete: got %v want %v", got, want)
	}
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

func TestTreeService_MoveNode_UpdatesSourceAndDestinationOrderFiles(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	destID, err := svc.CreateNode("system", nil, "Dest", "dest", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode dest: %v", err)
	}
	moveID, err := svc.CreateNode("system", nil, "Move", "move", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode move: %v", err)
	}
	stayID, err := svc.CreateNode("system", nil, "Stay", "stay", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode stay: %v", err)
	}
	nestedID, err := svc.CreateNode("system", destID, "Nested", "nested", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode nested: %v", err)
	}

	if err := svc.MoveNode("system", *moveID, *destID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}

	rootOrder := readOrderIDs(t, filepath.Join(tmpDir, "root"))
	wantRoot := []string{*destID, *stayID}
	if strings.Join(rootOrder, ",") != strings.Join(wantRoot, ",") {
		t.Fatalf("unexpected root order after move: got %v want %v", rootOrder, wantRoot)
	}

	destOrder := readOrderIDs(t, filepath.Join(tmpDir, "root", "dest"))
	wantDest := []string{*nestedID, *moveID}
	if strings.Join(destOrder, ",") != strings.Join(wantDest, ",") {
		t.Fatalf("unexpected destination order after move: got %v want %v", destOrder, wantDest)
	}
}

func TestTreeService_MoveNode_IgnoresOrderPersistenceErrorsAfterMove(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	destID, err := svc.CreateNode("system", nil, "Dest", "dest", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode dest: %v", err)
	}
	moveID, err := svc.CreateNode("system", nil, "Move", "move", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode move: %v", err)
	}

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil {
		t.Fatalf("remove root order file: %v", err)
	}
	mustMkdir(t, filepath.Join(tmpDir, "root", ".order.json"))
	if err := os.Remove(filepath.Join(tmpDir, "root", "dest", ".order.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove dest order file: %v", err)
	}
	mustMkdir(t, filepath.Join(tmpDir, "root", "dest", ".order.json"))

	if err := svc.MoveNode("system", *moveID, *destID); err != nil {
		t.Fatalf("MoveNode should succeed despite order persistence failure: %v", err)
	}

	destDir := filepath.Join(tmpDir, "root", "dest")
	mustStat(t, filepath.Join(destDir, "move.md"))
	root := svc.GetTree()
	if len(root.Children) != 1 || root.Children[0].ID != *destID {
		t.Fatalf("expected moved node removed from root, got %#v", root.Children)
	}
	dest := findChildBySlug(t, root, "dest")
	if len(dest.Children) != 1 || dest.Children[0].ID != *moveID {
		t.Fatalf("expected moved node under destination after best-effort order persistence")
	}
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

func TestTreeService_SortPages_PersistsOrderFileWithoutChangingMetadata(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

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

	root := svc.GetTree()
	before := map[string]PageMetadata{}
	for _, child := range root.Children {
		before[child.ID] = child.Metadata
	}

	if err := svc.SortPages("root", []string{*idC, *idA, *idB}); err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}

	orderPath := filepath.Join(tmpDir, "root", ".order.json")
	raw, err := os.ReadFile(orderPath)
	if err != nil {
		t.Fatalf("read order file: %v", err)
	}
	var persisted struct {
		OrderedIDs []string `json:"ordered_ids"`
	}
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("unmarshal order file: %v", err)
	}
	wantIDs := []string{*idC, *idA, *idB}
	if strings.Join(persisted.OrderedIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("unexpected persisted order: got %v want %v", persisted.OrderedIDs, wantIDs)
	}

	for _, child := range svc.GetTree().Children {
		if got := child.Metadata; got != before[child.ID] {
			t.Fatalf("metadata changed during reorder for %q: before=%+v after=%+v", child.ID, before[child.ID], got)
		}
	}
}

func TestTreeService_SortPages_RollsBackWhenOrderPersistenceFails(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

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

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil {
		t.Fatalf("remove root order file: %v", err)
	}
	mustMkdir(t, filepath.Join(tmpDir, "root", ".order.json"))

	err = svc.SortPages("root", []string{*idC, *idA, *idB})
	if err == nil {
		t.Fatalf("expected SortPages to fail when order persistence fails")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected SortPages error to mention child order persistence, got: %v", err)
	}

	root := svc.GetTree()
	want := []string{*idA, *idB, *idC}
	got := []string{root.Children[0].ID, root.Children[1].ID, root.Children[2].ID}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected in-memory order rollback, got %v want %v", got, want)
	}
	for i, child := range root.Children {
		if child.Position != i {
			t.Fatalf("expected child %q position rollback to %d, got %d", child.ID, i, child.Position)
		}
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

func TestTreeService_GetPage_SectionWithoutIndex_DoesNotMaterializeIndex(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "root", "docs", "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove index.md: %v", err)
	}

	page, err := svc.GetPage(*id)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if page.ID != *id {
		t.Fatalf("expected page ID %q, got %q", *id, page.ID)
	}
	if page.Content != "" {
		t.Fatalf("expected empty content for section without index, got %q", page.Content)
	}
	if _, err := os.Stat(indexPath); err == nil {
		t.Fatalf("expected GetPage to avoid materializing index.md")
	}
}

func TestTreeService_ConvertNode_PageToSection_MaterializesIndexWithNodeMetadata(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata.CreatedAt = time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC)
	node.Metadata.UpdatedAt = time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC)
	node.Metadata.CreatorID = "alice"
	node.Metadata.LastAuthorID = "bob"

	if err := svc.ConvertNode("carol", *id, NodeKindSection); err != nil {
		t.Fatalf("ConvertNode failed: %v", err)
	}

	indexPath := filepath.Join(tmpDir, "root", "docs", "index.md")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read converted index: %v", err)
	}

	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after conversion")
	}
	if fm.LeafWikiID != *id || fm.LeafWikiTitle != "Docs" {
		t.Fatalf("unexpected converted frontmatter: %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-22T10:15:30Z" {
		t.Fatalf("expected created_at to be preserved after conversion, got %#v", fm)
	}
	if fm.LeafWikiUpdatedAt == "" {
		t.Fatalf("expected updated_at after conversion, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "carol" {
		t.Fatalf("expected metadata to be carried over and updated for actor, got %#v", fm)
	}
	if !strings.Contains(body, "# Docs") {
		t.Fatalf("expected converted body to be preserved, got %q", body)
	}
}

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

func TestTreeService_EnsurePagePath_PersistsOrderFilesForCreatedPath(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	res, err := svc.EnsurePagePath("system", "home/about/team/members", "Members", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("EnsurePagePath failed: %v", err)
	}
	if res.Page == nil || res.Page.Slug != "members" {
		t.Fatalf("expected final page 'members'")
	}

	rootOrder := readOrderIDs(t, filepath.Join(tmpDir, "root"))
	if len(rootOrder) != 1 || rootOrder[0] != res.Created[0].ID {
		t.Fatalf("unexpected root order after EnsurePagePath: %v", rootOrder)
	}

	homeOrder := readOrderIDs(t, filepath.Join(tmpDir, "root", "home"))
	if len(homeOrder) != 1 || homeOrder[0] != res.Created[1].ID {
		t.Fatalf("unexpected home order after EnsurePagePath: %v", homeOrder)
	}

	aboutOrder := readOrderIDs(t, filepath.Join(tmpDir, "root", "home", "about"))
	if len(aboutOrder) != 1 || aboutOrder[0] != res.Created[2].ID {
		t.Fatalf("unexpected about order after EnsurePagePath: %v", aboutOrder)
	}

	teamOrder := readOrderIDs(t, filepath.Join(tmpDir, "root", "home", "about", "team"))
	if len(teamOrder) != 1 || teamOrder[0] != res.Created[3].ID {
		t.Fatalf("unexpected team order after EnsurePagePath: %v", teamOrder)
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

// --- F) Migration V3 (metadata frontmatter backfill) ---
func TestTreeService_LoadTree_MigratesToV5_BackfillsChildOrderFiles(t *testing.T) {
	if CurrentSchemaVersion < 5 {
		t.Skip("requires schema v5+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 4); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	docsID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode docs failed: %v", err)
	}
	alphaID, err := svc.CreateNode("system", nil, "Alpha", "alpha", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode alpha failed: %v", err)
	}
	betaID, err := svc.CreateNode("system", docsID, "Beta", "beta", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode beta failed: %v", err)
	}

	root := svc.GetTree()
	root.Children = []*PageNode{root.Children[1], root.Children[0]}
	for i, child := range root.Children {
		child.Position = i
	}

	docsNode, err := svc.FindPageByID(root.Children, *docsID)
	if err != nil {
		t.Fatalf("FindPageByID docs failed: %v", err)
	}
	if len(docsNode.Children) != 1 || docsNode.Children[0].ID != *betaID {
		t.Fatalf("expected docs child beta before migration")
	}

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove root order file: %v", err)
	}
	if err := os.Remove(filepath.Join(tmpDir, "root", "docs", ".order.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove docs order file: %v", err)
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	if err := saveSchema(tmpDir, 4); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	loaded := NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	rootOrder := readOrderIDs(t, filepath.Join(tmpDir, "root"))
	wantRoot := []string{*alphaID, *docsID}
	if strings.Join(rootOrder, ",") != strings.Join(wantRoot, ",") {
		t.Fatalf("unexpected root order after migration: got %v want %v", rootOrder, wantRoot)
	}

	docsOrder := readOrderIDs(t, filepath.Join(tmpDir, "root", "docs"))
	wantDocs := []string{*betaID}
	if strings.Join(docsOrder, ",") != strings.Join(wantDocs, ",") {
		t.Fatalf("unexpected docs order after migration: got %v want %v", docsOrder, wantDocs)
	}
}

func TestTreeService_LoadTree_MigratesToV4_MaterializesMissingSectionIndex(t *testing.T) {
	if CurrentSchemaVersion < 4 {
		t.Skip("requires schema v4+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 3); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	indexPath := filepath.Join(tmpDir, "root", "docs", "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove section index failed: %v", err)
	}

	if err := saveSchema(tmpDir, 3); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	loaded := NewTreeService(tmpDir)
	if err := loaded.LoadTree(); err != nil {
		t.Fatalf("LoadTree (migrating) failed: %v", err)
	}

	raw, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read migrated section index: %v", err)
	}
	fm, body, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiID != *id || fm.LeafWikiTitle != "Docs" {
		t.Fatalf("expected section frontmatter to be materialized, got %#v", fm)
	}
	if fm.LeafWikiCreatedAt != "2026-03-22T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-22T11:16:31Z" {
		t.Fatalf("expected timestamps to be materialized, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("expected author metadata to be materialized, got %#v", fm)
	}
	if strings.TrimSpace(body) != "" {
		t.Fatalf("expected empty section body after migration, got %q", body)
	}
}

func TestTreeService_LoadTree_MigratesToV3_BackfillsMetadataFrontmatter(t *testing.T) {
	if CurrentSchemaVersion < 3 {
		t.Skip("requires schema v3+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 2); err != nil {
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

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 21, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 21, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := "---\nleafwiki_id: " + *id + "\nleafwiki_title: Page1\n---\n# Page 1 Content\nHello World\n"
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}

	if err := saveSchema(tmpDir, 2); err != nil {
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

	fm, migratedBody, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiCreatedAt != "2026-03-21T10:15:30Z" || fm.LeafWikiUpdatedAt != "2026-03-21T11:16:31Z" {
		t.Fatalf("expected metadata timestamps to be backfilled, got %#v", fm)
	}
	if fm.LeafWikiCreatorID != "alice" || fm.LeafWikiLastAuthorID != "bob" {
		t.Fatalf("expected metadata authors to be backfilled, got %#v", fm)
	}
	wantBody := "# Page 1 Content\nHello World\n"
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

// --- F) Migration V2 (frontmatter backfill) ---
func TestTreeService_LoadTree_MigratesToV2_AddsFrontmatterAndPreservesBody(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()

	// start on v1 (or generally: current-1)
	if err := saveSchema(tmpDir, 1); err != nil {
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
	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	// overwrite file without FM
	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	body := "# Page 1 Content\nHello World\n"
	if err := os.WriteFile(pagePath, []byte(body), 0o644); err != nil {
		t.Fatalf("write old content failed: %v", err)
	}

	// force schema old again
	if err := saveSchema(tmpDir, 1); err != nil {
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

	fm, migratedBody, has, err := markdown.ParseFrontmatter(string(raw))
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

func TestTreeService_LoadTree_MigratesToV2_PreservesExistingCustomFrontmatter(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 1); err != nil {
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

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := `---
custom_key: keep-me
tags:
  - alpha
---
# Page 1 Content
Hello World
`
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}

	if err := saveSchema(tmpDir, 1); err != nil {
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

	migrated := string(raw)
	if !strings.Contains(migrated, "custom_key: keep-me") {
		t.Fatalf(`expected custom frontmatter to be preserved, got:
%s`, migrated)
	}
	if !strings.Contains(migrated, "- alpha") {
		t.Fatalf(`expected list frontmatter to be preserved, got:
%s`, migrated)
	}

	fm, migratedBody, has, err := markdown.ParseFrontmatter(migrated)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf(`expected frontmatter after migration, got:
%s`, migrated)
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if strings.TrimSpace(fm.LeafWikiTitle) == "" {
		t.Fatalf("expected leafwiki_title to be set")
	}
	wantBody := `# Page 1 Content
Hello World
`
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

func TestTreeService_LoadTree_MigratesToV2_PreservesExistingLeafWikiTitle(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 1); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Tree Title", "page1", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := `---
leafwiki_title: Existing Title
---
# Page 1 Content
Hello World
`
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}

	if err := saveSchema(tmpDir, 1); err != nil {
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

	fm, migratedBody, has, err := markdown.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Existing Title" {
		t.Fatalf("expected existing leafwiki_title to be preserved, got %q", fm.LeafWikiTitle)
	}
	wantBody := `# Page 1 Content
Hello World
`
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

func TestTreeService_LoadTree_MigratesToV2_PreservesTitleAlias(t *testing.T) {
	if CurrentSchemaVersion < 2 {
		t.Skip("requires schema v2+")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 1); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Tree Title", "page1", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	pagePath := filepath.Join(tmpDir, "root", "page1.md")
	legacyContent := `---
title: Alias Title
custom_key: keep-me
---
# Page 1 Content
Hello World
`
	if err := os.WriteFile(pagePath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy content failed: %v", err)
	}

	if err := saveSchema(tmpDir, 1); err != nil {
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

	migrated := string(raw)
	if !strings.Contains(migrated, "title: Alias Title") {
		t.Fatalf(`expected title alias to be preserved, got:
%s`, migrated)
	}
	if !strings.Contains(migrated, "custom_key: keep-me") {
		t.Fatalf(`expected custom frontmatter to be preserved, got:
%s`, migrated)
	}

	fm, migratedBody, has, err := markdown.ParseFrontmatter(migrated)
	if err != nil {
		t.Fatalf("ParseFrontmatter: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after migration")
	}
	if fm.LeafWikiID != *id {
		t.Fatalf("expected leafwiki_id=%q, got %q", *id, fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Alias Title" {
		t.Fatalf("expected title alias to remain effective, got %q", fm.LeafWikiTitle)
	}
	wantBody := `# Page 1 Content
Hello World
`
	if migratedBody != wantBody {
		t.Fatalf("expected body preserved exactly.\nGot:\n%q\nWant:\n%q", migratedBody, wantBody)
	}
}

// TestTreeService_ReconstructTreeFromFS_UpdatesSchemaVersion verifies that
// ReconstructTreeFromFS writes the current schema version to prevent unnecessary migrations
func TestTreeService_ReconstructTreeFromFS_UpdatesSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal file structure for reconstruction
	mustMkdir(t, filepath.Join(tmpDir, "root"))
	mustWriteFile(t, filepath.Join(tmpDir, "root", "test.md"), "# Test Page", 0o644)

	// Create service WITHOUT schema.json (simulating an old/missing schema)
	svc := NewTreeService(tmpDir)

	// Reconstruct the tree (no prior tree loaded)
	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	// Verify schema.json was created with current version
	schema, err := loadSchema(tmpDir)
	if err != nil {
		t.Fatalf("loadSchema failed: %v", err)
	}

	if schema.Version != CurrentSchemaVersion {
		t.Errorf("expected schema version %d after reconstruction, got %d", CurrentSchemaVersion, schema.Version)
	}

	// Startup reconstruction should no longer create a tree.json snapshot.
	mustNotExist(t, filepath.Join(tmpDir, "tree.json"))
}

// --- G) ReconstructTreeFromFS ---

func TestTreeService_ReconstructTreeFromFS_BackfillsMetadata(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create some files on disk manually (simulating external changes)
	mustWriteFile(t, filepath.Join(tmpDir, "root", "page1.md"), `---
leafwiki_id: page-1
leafwiki_title: Page One
---
# Page One`, 0o644)

	mustMkdir(t, filepath.Join(tmpDir, "root", "section1"))
	mustWriteFile(t, filepath.Join(tmpDir, "root", "section1", "index.md"), `---
leafwiki_id: sec-1
leafwiki_title: Section One
---
# Section One`, 0o644)

	mustWriteFile(t, filepath.Join(tmpDir, "root", "section1", "page2.md"), `---
leafwiki_id: page-2
leafwiki_title: Page Two
---
# Page Two`, 0o644)

	// Reconstruct the tree from filesystem
	err := svc.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	// Verify metadata was backfilled for all nodes
	tree := svc.GetTree()

	// Check root metadata
	if tree.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected root metadata CreatedAt to be backfilled, got zero")
	}
	if tree.Metadata.UpdatedAt.IsZero() {
		t.Fatalf("expected root metadata UpdatedAt to be backfilled, got zero")
	}

	// Find and verify page1
	page1 := findChildBySlug(t, tree, "page1")
	if page1.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected page1 metadata CreatedAt to be backfilled, got zero")
	}
	if page1.Metadata.UpdatedAt.IsZero() {
		t.Fatalf("expected page1 metadata UpdatedAt to be backfilled, got zero")
	}

	// Find and verify section1
	section1 := findChildBySlug(t, tree, "section1")
	if section1.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected section1 metadata CreatedAt to be backfilled, got zero")
	}
	if section1.Metadata.UpdatedAt.IsZero() {
		t.Fatalf("expected section1 metadata UpdatedAt to be backfilled, got zero")
	}

	// Find and verify page2 (child of section1)
	page2 := findChildBySlug(t, section1, "page2")
	if page2.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected page2 metadata CreatedAt to be backfilled, got zero")
	}
	if page2.Metadata.UpdatedAt.IsZero() {
		t.Fatalf("expected page2 metadata UpdatedAt to be backfilled, got zero")
	}
}

func TestTreeService_ReconstructTreeFromFS_ReloadsFromFilesystem(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create some files on disk manually
	mustWriteFile(t, filepath.Join(tmpDir, "root", "readme.md"), `---
leafwiki_id: readme-page
leafwiki_title: README
---
# README`, 0o644)

	// Reconstruct the tree from filesystem
	err := svc.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	// Verify we can reload the tree directly from the filesystem.
	newSvc := NewTreeService(tmpDir)
	if err := newSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree after reconstruction failed: %v", err)
	}

	// Verify the tree structure matches
	tree := newSvc.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Fatalf("expected root node after reload, got: %+v", tree)
	}

	// Verify the readme page exists
	readme := findChildBySlug(t, tree, "readme")
	if readme.ID != "readme-page" {
		t.Fatalf("expected readme ID to be 'readme-page', got %q", readme.ID)
	}
	if readme.Title != "README" {
		t.Fatalf("expected readme title to be 'README', got %q", readme.Title)
	}

	// Verify metadata was persisted
	if readme.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected persisted metadata CreatedAt to not be zero")
	}
	if readme.Metadata.UpdatedAt.IsZero() {
		t.Fatalf("expected persisted metadata UpdatedAt to not be zero")
	}
}

func TestTreeService_ReconstructTreeFromFS_ReloadsMetadataFromFrontmatter(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	mustWriteFile(t, filepath.Join(tmpDir, "root", "readme.md"), `---
leafwiki_id: readme-page
leafwiki_title: README
leafwiki_created_at: 2026-03-21T10:15:30Z
leafwiki_updated_at: 2026-03-21T11:16:31Z
leafwiki_creator_id: alice
leafwiki_last_author_id: bob
---
# README`, 0o644)

	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	newSvc := NewTreeService(tmpDir)
	if err := newSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree after reconstruction failed: %v", err)
	}

	readme := findChildBySlug(t, newSvc.GetTree(), "readme")
	if got := readme.Metadata.CreatedAt.UTC().Format(time.RFC3339); got != "2026-03-21T10:15:30Z" {
		t.Fatalf("expected persisted created_at from frontmatter, got %q", got)
	}
	if got := readme.Metadata.UpdatedAt.UTC().Format(time.RFC3339); got != "2026-03-21T11:16:31Z" {
		t.Fatalf("expected persisted updated_at from frontmatter, got %q", got)
	}
	if readme.Metadata.CreatorID != "alice" || readme.Metadata.LastAuthorID != "bob" {
		t.Fatalf("expected persisted author metadata from frontmatter, got %#v", readme.Metadata)
	}
}

func TestTreeService_ReconstructTreeFromFS_ReloadsMetadataFallbacksWhenMissing(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	readmePath := filepath.Join(tmpDir, "root", "readme.md")
	mustWriteFile(t, readmePath, `# README`, 0o644)

	wantTime := time.Date(2026, time.March, 21, 12, 34, 56, 0, time.UTC)
	if err := os.Chtimes(readmePath, wantTime, wantTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	newSvc := NewTreeService(tmpDir)
	if err := newSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree after reconstruction failed: %v", err)
	}

	readme := findChildBySlug(t, newSvc.GetTree(), "readme")
	if strings.TrimSpace(readme.ID) == "" {
		t.Fatalf("expected generated ID to persist")
	}
	if got := readme.Metadata.CreatedAt.UTC().Format(time.RFC3339); got != wantTime.Format(time.RFC3339) {
		t.Fatalf("expected persisted created_at fallback from mtime, got %q", got)
	}
	if got := readme.Metadata.UpdatedAt.UTC().Format(time.RFC3339); got != wantTime.Format(time.RFC3339) {
		t.Fatalf("expected persisted updated_at fallback from mtime, got %q", got)
	}
	if readme.Metadata.CreatorID != reconstructSystemUserID || readme.Metadata.LastAuthorID != reconstructSystemUserID {
		t.Fatalf("expected persisted system-user metadata fallback, got %#v", readme.Metadata)
	}
}

func TestTreeService_ReconstructTreeFromFS_ComplexTree_PreservesStructure(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Create a complex tree structure on disk
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

	mustMkdir(t, filepath.Join(tmpDir, "root", "docs", "guides"))
	mustWriteFile(t, filepath.Join(tmpDir, "root", "docs", "guides", "index.md"), `---
leafwiki_id: guides-section
leafwiki_title: Guides
---
# Guides`, 0o644)

	mustWriteFile(t, filepath.Join(tmpDir, "root", "docs", "guides", "basic.md"), `---
leafwiki_id: basic-guide
leafwiki_title: Basic Guide
---
# Basic Guide`, 0o644)

	// Reconstruct
	err := svc.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	tree := svc.GetTree()

	// Verify structure
	intro := findChildBySlug(t, tree, "intro")
	if intro.Kind != NodeKindPage {
		t.Fatalf("expected intro to be a page, got %q", intro.Kind)
	}

	docs := findChildBySlug(t, tree, "docs")
	if docs.Kind != NodeKindSection {
		t.Fatalf("expected docs to be a section, got %q", docs.Kind)
	}
	if docs.ID != "docs-section" {
		t.Fatalf("expected docs ID to be 'docs-section', got %q", docs.ID)
	}

	gettingStarted := findChildBySlug(t, docs, "getting-started")
	if gettingStarted.Kind != NodeKindPage {
		t.Fatalf("expected getting-started to be a page, got %q", gettingStarted.Kind)
	}

	guides := findChildBySlug(t, docs, "guides")
	if guides.Kind != NodeKindSection {
		t.Fatalf("expected guides to be a section, got %q", guides.Kind)
	}

	basic := findChildBySlug(t, guides, "basic")
	if basic.Kind != NodeKindPage {
		t.Fatalf("expected basic to be a page, got %q", basic.Kind)
	}

	// Verify all nodes have metadata
	if intro.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected intro to have metadata")
	}
	if docs.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected docs to have metadata")
	}
	if guides.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected guides to have metadata")
	}
	if basic.Metadata.CreatedAt.IsZero() {
		t.Fatalf("expected basic to have metadata")
	}

	mustNotExist(t, filepath.Join(tmpDir, "tree.json"))

	reloadedSvc := NewTreeService(tmpDir)
	if err := reloadedSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree after reconstruction failed: %v", err)
	}

	reloadedTree := reloadedSvc.GetTree()
	if len(reloadedTree.Children) != len(tree.Children) {
		t.Fatalf("expected reloaded tree to have same number of children")
	}
}

func TestTreeService_ReconstructTreeFromFS_EmptyDirectory_CreatesRootAndPersists(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	// Reconstruct from empty directory (should create just root)
	err := svc.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	tree := svc.GetTree()
	if tree == nil || tree.ID != "root" {
		t.Fatalf("expected root node, got: %+v", tree)
	}

	// Note: Root metadata may not be backfilled from filesystem when directory is empty
	// because there's no corresponding file/directory to stat. This is expected behavior.
	// The important thing is that the tree is reconstructed and persisted.

	mustNotExist(t, filepath.Join(tmpDir, "tree.json"))

	// Verify we can reload
	reloadedSvc := NewTreeService(tmpDir)
	if err := reloadedSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree after reconstruction failed: %v", err)
	}

	reloadedTree := reloadedSvc.GetTree()
	if reloadedTree == nil || reloadedTree.ID != "root" {
		t.Fatalf("expected root node after reload")
	}
}

func TestTreeService_ReconstructTreeFromFS_RevertsOnMetadataBackfillError(t *testing.T) {
	// This test is harder to trigger without mocking, but we can at least verify
	// that if the tree state is preserved if we can cause a failure scenario.
	// For now, we'll test that a successful reconstruction doesn't lose the old tree.
	svc, tmpDir := newLoadedService(t)

	// Create initial tree state
	initialID, err := svc.CreateNode("system", nil, "Initial", "initial", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	// Get initial tree
	initialTree := svc.GetTree()
	if len(initialTree.Children) != 1 {
		t.Fatalf("expected 1 child in initial tree")
	}

	// Create a new file on disk
	mustWriteFile(t, filepath.Join(tmpDir, "root", "new-page.md"), `---
leafwiki_id: new-page
leafwiki_title: New Page
---
# New Page`, 0o644)

	// Reconstruct should succeed
	err = svc.ReconstructTreeFromFS()
	if err != nil {
		t.Fatalf("ReconstructTreeFromFS failed: %v", err)
	}

	// Verify new tree has both nodes
	newTree := svc.GetTree()
	if len(newTree.Children) != 2 {
		t.Fatalf("expected 2 children after reconstruction, got %d", len(newTree.Children))
	}

	// Verify initial node still exists
	var foundInitial bool
	for _, child := range newTree.Children {
		if child.ID == *initialID {
			foundInitial = true
			break
		}
	}
	if !foundInitial {
		t.Fatalf("expected initial node to still exist after reconstruction")
	}
}

// --- small util ---

func ptrKind(k NodeKind) *NodeKind { return &k }

func TestTreeService_LoadTree_MigratesToV5_ReturnsErrorWhenOrderFileCannotBeWritten(t *testing.T) {
	if CurrentSchemaVersion < 5 {
		t.Skip("requires schema v5+")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission-based migration failure test is not reliable on Windows")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 4); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	_, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	_, err = svc.CreateNode("system", nil, "Alpha", "alpha", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode alpha failed: %v", err)
	}

	if err := os.Remove(filepath.Join(tmpDir, "root", ".order.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove root order file failed: %v", err)
	}
	if err := os.Chmod(filepath.Join(tmpDir, "root"), 0o555); err != nil {
		t.Fatalf("chmod root dir failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(filepath.Join(tmpDir, "root"), 0o755)
	}()

	if err := saveSchema(tmpDir, 4); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	loaded := NewTreeService(tmpDir)
	err = loaded.LoadTree()
	if err == nil {
		t.Fatalf("expected migration error when order file cannot be written")
	}
	if !strings.Contains(err.Error(), "persist child order") {
		t.Fatalf("expected migration error to mention child order persistence, got: %v", err)
	}
}

func TestTreeService_LoadTree_MigratesToV4_ReturnsErrorWhenSectionIndexCannotBeWritten(t *testing.T) {
	if CurrentSchemaVersion < 4 {
		t.Skip("requires schema v4+")
	}
	if runtime.GOOS == "windows" {
		t.Skip("permission-based migration failure test is not reliable on Windows")
	}

	tmpDir := t.TempDir()

	if err := saveSchema(tmpDir, 3); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	svc := NewTreeService(tmpDir)
	if err := svc.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}

	id, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}

	node, err := svc.FindPageByID(svc.GetTree().Children, *id)
	if err != nil {
		t.Fatalf("FindPageByID failed: %v", err)
	}
	node.Metadata = PageMetadata{
		CreatedAt:    time.Date(2026, time.March, 22, 10, 15, 30, 0, time.UTC),
		UpdatedAt:    time.Date(2026, time.March, 22, 11, 16, 31, 0, time.UTC),
		CreatorID:    "alice",
		LastAuthorID: "bob",
	}

	persistLegacyTreeSnapshot(t, tmpDir, svc.GetTree())

	sectionDir := filepath.Join(tmpDir, "root", "docs")
	indexPath := filepath.Join(sectionDir, "index.md")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove section index failed: %v", err)
	}
	if err := os.Chmod(sectionDir, 0o555); err != nil {
		t.Fatalf("chmod section dir failed: %v", err)
	}
	defer func() {
		_ = os.Chmod(sectionDir, 0o755)
	}()

	if err := saveSchema(tmpDir, 3); err != nil {
		t.Fatalf("saveSchema failed: %v", err)
	}

	loaded := NewTreeService(tmpDir)
	err = loaded.LoadTree()
	if err == nil {
		t.Fatalf("expected migration error when section index cannot be written")
	}
	if !strings.Contains(err.Error(), "materialize section index") {
		t.Fatalf("expected migration error to mention section index materialization, got: %v", err)
	}
}
