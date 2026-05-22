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
	if err := treeSvc.UpdateNode("system", *id, title, slug, &content, page.Version(), false); err != nil {
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
	treeSvc, index, effect := setupSearchTest(t)

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
	_ = treeSvc
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
	if err := treeSvc.UpdateNode("system", page.ID, page.Title, page.Slug, &newContent, page.Version(), false); err != nil {
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
