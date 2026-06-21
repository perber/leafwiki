package pagesave

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/search"
)

// setupSearchTest creates a temp-dir-backed tree, SQLiteIndex and SearchIndexSideEffect.
func setupSearchTest(t *testing.T) (*tree.TreeService, *search.SQLiteIndex, *SearchIndexSideEffect) {
	t.Helper()
	tmp := t.TempDir()

	treeSvc := tree.NewTreeService(tmp)
	if err := treeSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	index, err := search.NewSQLiteIndex(tmp)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	t.Cleanup(func() {
		if err := index.Close(); err != nil {
			t.Errorf("index.Close: %v", err)
		}
	})

	effect := NewSearchIndexSideEffect(index, treeSvc, nil)
	return treeSvc, index, effect
}

// createPageWithContent creates a page node and writes content to it via UpdateNode.
func createPageWithContent(t *testing.T, treeSvc *tree.TreeService, title, slug, content string) *tree.Page {
	t.Helper()
	kind := tree.NodeKindPage
	id, err := treeSvc.CreateNode("system", nil, title, slug, &kind)
	if err != nil {
		t.Fatalf("CreateNode(%q): %v", title, err)
	}
	page, err := treeSvc.GetPage(*id)
	if err != nil {
		t.Fatalf("GetPage after CreateNode: %v", err)
	}
	if err := treeSvc.UpdateNode("system", *id, title, slug, &content, page.Version(), nil, nil, false); err != nil {
		t.Fatalf("UpdateNode(%q): %v", title, err)
	}
	page, err = treeSvc.GetPage(*id)
	if err != nil {
		t.Fatalf("GetPage after UpdateNode: %v", err)
	}
	return page
}

// ─── IndexAllPages ────────────────────────────────────────────────────────────

func TestSearchIndexSideEffect_IndexAllPages_IndexesExistingPages(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	page := createPageWithContent(t, treeSvc, "Search Test Page", "search-test", "# Search Test Page\nThis is some uniquecontent for indexing.")

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	result, err := index.Search("uniquecontent", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected at least one search hit, got 0")
	}
	if result.Items[0].PageID != page.ID {
		t.Errorf("expected pageID %q, got %q", page.ID, result.Items[0].PageID)
	}
}

func TestSearchIndexSideEffect_IndexAllPages_ClearsStaleEntries(t *testing.T) {
	_, index, effect := setupSearchTest(t)

	// Pre-populate the index with a stale entry not present in the tree.
	if err := index.IndexPage("stale/path", "stale.md", "stale-id", "Stale Page", tree.NodeKindPage, "stale content ghostpage"); err != nil {
		t.Fatalf("IndexPage (stale): %v", err)
	}

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	result, err := index.Search("ghostpage", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("expected stale entry to be cleared, got %d hits", result.Count)
	}
}

func TestSearchIndexSideEffect_IndexAllPages_EmptyTree(t *testing.T) {
	_, index, effect := setupSearchTest(t)

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("expected no error on empty tree, got: %v", err)
	}

	result, err := index.Search("anything", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("expected 0 hits on empty tree, got %d", result.Count)
	}
}

// ─── Apply ───────────────────────────────────────────────────────────────────

func TestSearchIndexSideEffect_Apply_Create_IndexesPage(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	page := createPageWithContent(t, treeSvc, "Created Page", "created", "some uniqueterm_create content")

	effect.Apply(PageSaveEvent{
		Operation: PageOperationCreate,
		After:     page,
	})

	result, err := index.Search("uniqueterm_create", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected page to be indexed after Create event")
	}
	if result.Items[0].PageID != page.ID {
		t.Errorf("expected pageID %q, got %q", page.ID, result.Items[0].PageID)
	}
}

func TestSearchIndexSideEffect_Apply_Update_ReplacesContentAfterBootstrap(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	page := createPageWithContent(t, treeSvc, "My Page", "my-page", "initial uniqueword_before content")

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	before, err := index.Search("uniqueword_before", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search (before): %v", err)
	}
	if before.Count == 0 {
		t.Fatal("expected initial content to be indexed after bootstrap")
	}

	newContent := "updated uniqueword_after content"
	if err := treeSvc.UpdateNode("system", page.ID, page.Title, page.Slug, &newContent, page.Version(), nil, nil, false); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	updated, err := treeSvc.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage after update: %v", err)
	}

	effect.Apply(PageSaveEvent{
		Operation: PageOperationUpdate,
		After:     updated,
	})

	stale, err := index.Search("uniqueword_before", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search (stale): %v", err)
	}
	if stale.Count != 0 {
		t.Errorf("expected old content to be replaced, but 'uniqueword_before' still found")
	}

	fresh, err := index.Search("uniqueword_after", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search (fresh): %v", err)
	}
	if fresh.Count == 0 {
		t.Error("expected new content to be searchable after Update event")
	}
}

func TestSearchIndexSideEffect_Apply_Delete_RemovesFromIndex(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	page := createPageWithContent(t, treeSvc, "Delete Me", "delete-me", "deletable uniqueterm_delete content")

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	before, err := index.Search("uniqueterm_delete", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search (before): %v", err)
	}
	if before.Count == 0 {
		t.Fatal("expected page to be indexed before deletion")
	}

	effect.Apply(PageSaveEvent{
		Operation:     PageOperationDelete,
		AffectedPages: []*tree.Page{page},
	})

	after, err := index.Search("uniqueterm_delete", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search (after): %v", err)
	}
	if after.Count != 0 {
		t.Errorf("expected page to be removed from index after Delete event, got %d hits", after.Count)
	}
}

func TestSearchIndexSideEffect_Apply_Delete_Recursive_RemovesAllPagesFromIndex(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	parent := createPageWithContent(t, treeSvc, "Parent Section", "parent", "parent uniqueterm_parent content")

	kind := tree.NodeKindPage
	child1ID, err := treeSvc.CreateNode("system", &parent.ID, "Child One", "child-one", &kind)
	if err != nil {
		t.Fatalf("CreateNode child1: %v", err)
	}
	child1, err := treeSvc.GetPage(*child1ID)
	if err != nil {
		t.Fatalf("GetPage child1: %v", err)
	}
	content1 := "child one uniqueterm_child1 content"
	if err := treeSvc.UpdateNode("system", child1.ID, child1.Title, child1.Slug, &content1, child1.Version(), nil, nil, false); err != nil {
		t.Fatalf("UpdateNode child1: %v", err)
	}

	child2ID, err := treeSvc.CreateNode("system", &parent.ID, "Child Two", "child-two", &kind)
	if err != nil {
		t.Fatalf("CreateNode child2: %v", err)
	}
	child2, err := treeSvc.GetPage(*child2ID)
	if err != nil {
		t.Fatalf("GetPage child2: %v", err)
	}
	content2 := "child two uniqueterm_child2 content"
	if err := treeSvc.UpdateNode("system", child2.ID, child2.Title, child2.Slug, &content2, child2.Version(), nil, nil, false); err != nil {
		t.Fatalf("UpdateNode child2: %v", err)
	}

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	for _, term := range []string{"uniqueterm_parent", "uniqueterm_child1", "uniqueterm_child2"} {
		r, err := index.Search(term, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) before delete: %v", term, err)
		}
		if r.Count == 0 {
			t.Fatalf("expected %q to be indexed before delete", term)
		}
	}

	// Simulate recursive delete: AffectedPages contains the full subtree.
	child1Final, err := treeSvc.GetPage(*child1ID)
	if err != nil {
		t.Fatalf("GetPage child1Final: %v", err)
	}
	child2Final, err := treeSvc.GetPage(*child2ID)
	if err != nil {
		t.Fatalf("GetPage child2Final: %v", err)
	}
	parentFinal, err := treeSvc.GetPage(parent.ID)
	if err != nil {
		t.Fatalf("GetPage parentFinal: %v", err)
	}
	effect.Apply(PageSaveEvent{
		Operation:     PageOperationDelete,
		AffectedPages: []*tree.Page{child1Final, child2Final, parentFinal},
	})

	for _, term := range []string{"uniqueterm_parent", "uniqueterm_child1", "uniqueterm_child2"} {
		r, err := index.Search(term, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) after delete: %v", term, err)
		}
		if r.Count != 0 {
			t.Errorf("expected %q to be removed after recursive delete, got %d hits", term, r.Count)
		}
	}
}

func TestSearchIndexSideEffect_Apply_Move_PageStillSearchableAtNewPath(t *testing.T) {
	treeSvc, index, effect := setupSearchTest(t)

	// Create a parent page (will auto-convert to section when child is added).
	kind := tree.NodeKindPage
	parentID, err := treeSvc.CreateNode("system", nil, "Target Section", "target", &kind)
	if err != nil {
		t.Fatalf("CreateNode parent: %v", err)
	}

	page := createPageWithContent(t, treeSvc, "Movable Page", "movable", "movable uniqueterm_move content")

	if err := effect.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	// Move the page under the parent section.
	if err := treeSvc.MoveNode("system", page.ID, *parentID, page.Version()); err != nil {
		t.Fatalf("MoveNode: %v", err)
	}
	moved, err := treeSvc.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage after move: %v", err)
	}

	effect.Apply(PageSaveEvent{
		Operation:     PageOperationMove,
		AffectedPages: []*tree.Page{moved},
	})

	result, err := index.Search("uniqueterm_move", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count == 0 {
		t.Fatal("expected page to remain searchable after Move event")
	}
	if result.Items[0].PageID != page.ID {
		t.Errorf("expected pageID %q, got %q", page.ID, result.Items[0].PageID)
	}

	// Path in index must reflect the new location.
	wantPath := "target/movable"
	if result.Items[0].Path != wantPath {
		t.Errorf("expected path %q after move, got %q", wantPath, result.Items[0].Path)
	}
}
