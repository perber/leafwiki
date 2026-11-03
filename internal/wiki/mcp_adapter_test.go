package wiki

import (
	"context"
	"errors"
	"strings"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
)

// newMCPTestWiki boots a real wiki, creates a page with searchable content, and
// returns it along with the adapter under test. Every read op is exercised
// against the same live page so the delegation is observable end to end.
func newMCPTestWiki(t *testing.T) (*Wiki, *WikiMCPAdapter, *createdPage) {
	t.Helper()
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })

	page := createPageForTest(t, w, "system", nil, "MCP Sample", "mcp-sample", pageNodeKind())
	content := "The quick brown fox jumps over the lazy dog."
	updatePageForTest(t, w, "system", page.ID, page.Title, page.Slug, &content, pageNodeKind())
	return w, NewWikiMCPAdapter(w), &createdPage{id: page.ID, slug: page.Slug, content: content}
}

type createdPage struct {
	id      string
	slug    string
	content string
}

// TestWikiMCPAdapter_GetPage_Delegates reads the live page back through the
// existing get-page-by-ID use case and surfaces its content and metadata.
func TestWikiMCPAdapter_GetPage_Delegates(t *testing.T) {
	_, adapter, sample := newMCPTestWiki(t)

	page, err := adapter.GetPage(context.Background(), sample.id)
	if err != nil {
		t.Fatalf("GetPage(%q) error: %v", sample.id, err)
	}
	if page.ID != sample.id {
		t.Errorf("GetPage ID = %q, want %q", page.ID, sample.id)
	}
	if page.Title != "MCP Sample" {
		t.Errorf("GetPage Title = %q, want %q", page.Title, "MCP Sample")
	}
	if page.Path != sample.slug {
		t.Errorf("GetPage Path = %q, want %q", page.Path, sample.slug)
	}
	if page.Content != sample.content {
		t.Errorf("GetPage Content = %q, want %q", page.Content, sample.content)
	}
	if page.Kind != "page" {
		t.Errorf("GetPage Kind = %q, want %q", page.Kind, "page")
	}
}

// TestWikiMCPAdapter_GetPage_NotFound maps the tree sentinel the use case
// passes through onto the LocalizedError the REST route would return: code
// page_not_found with the client-facing message, not a bare sentinel the
// tool error mapper would render as mcp_internal_error.
func TestWikiMCPAdapter_GetPage_NotFound(t *testing.T) {
	_, adapter, _ := newMCPTestWiki(t)

	_, err := adapter.GetPage(context.Background(), "no-such-page")
	assertPageError(t, err, wikipages.ErrCodePageNotFound, "Page not found")
}

// TestWikiMCPAdapter_GetPageByPath_NotFound pins the same mapping for the
// read-by-route-path op.
func TestWikiMCPAdapter_GetPageByPath_NotFound(t *testing.T) {
	_, adapter, _ := newMCPTestWiki(t)

	_, err := adapter.GetPageByPath(context.Background(), "no/such/page")
	assertPageError(t, err, wikipages.ErrCodePageNotFound, "Page not found")
}

// TestWikiMCPAdapter_TreeNotLoaded maps the reachable ErrTreeNotLoaded
// sentinel (both page-read ops check it before touching the tree) onto the
// page_internal_error LocalizedError the REST route would return.
func TestWikiMCPAdapter_TreeNotLoaded(t *testing.T) {
	// A zero-value TreeService has a nil tree, which is the exact state the
	// read ops report as ErrTreeNotLoaded.
	adapter := &WikiMCPAdapter{tree: &tree.TreeService{}}

	tests := []struct {
		name string
		get  func() error
	}{
		{name: "GetPage", get: func() error { _, err := adapter.GetPage(context.Background(), "p-1"); return err }},
		{name: "GetPageByPath", get: func() error { _, err := adapter.GetPageByPath(context.Background(), "some/path"); return err }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertPageError(t, tc.get(), wikipages.ErrCodePageInternalError, "Tree not loaded")
		})
	}
}

// assertPageError asserts err is the expected page-domain LocalizedError with
// its client-facing code and message.
func assertPageError(t *testing.T, err error, wantCode, wantMessage string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("error = %T (%v), want *sharederrors.LocalizedError", err, err)
	}
	if loc.Code != wantCode {
		t.Errorf("code = %q, want %q", loc.Code, wantCode)
	}
	if loc.Message != wantMessage {
		t.Errorf("message = %q, want %q", loc.Message, wantMessage)
	}
}

// TestWikiMCPAdapter_GetPageByPath_Delegates reads a page by its route path via
// the existing get-page-by-path use case.
func TestWikiMCPAdapter_GetPageByPath_Delegates(t *testing.T) {
	_, adapter, sample := newMCPTestWiki(t)

	page, err := adapter.GetPageByPath(context.Background(), sample.slug)
	if err != nil {
		t.Fatalf("GetPageByPath(%q) error: %v", sample.slug, err)
	}
	if page.ID != sample.id {
		t.Errorf("GetPageByPath ID = %q, want %q", page.ID, sample.id)
	}
	if page.Content != sample.content {
		t.Errorf("GetPageByPath Content = %q, want %q", page.Content, sample.content)
	}
}

// TestWikiMCPAdapter_Search_Delegates finds the live page by a word in its
// content through the existing search use case.
func TestWikiMCPAdapter_Search_Delegates(t *testing.T) {
	_, adapter, _ := newMCPTestWiki(t)

	res, err := adapter.Search(context.Background(), "quick brown fox", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if res == nil || len(res.Items) == 0 {
		t.Fatalf("expected at least one search hit, got %+v", res)
	}
	found := false
	for _, item := range res.Items {
		if strings.Contains(item.Excerpt, "quick") || item.Title == "MCP Sample" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the sample page among search hits, got %+v", res.Items)
	}
}

// TestWikiMCPAdapter_ListTree_Delegates returns the page tree with the live page
// present (top-level node, synthetic root stripped).
func TestWikiMCPAdapter_ListTree_Delegates(t *testing.T) {
	_, adapter, sample := newMCPTestWiki(t)

	nodes, err := adapter.ListTree(context.Background())
	if err != nil {
		t.Fatalf("ListTree error: %v", err)
	}
	var found bool
	for _, node := range nodes {
		if node.Slug == sample.slug && node.Kind == "page" {
			found = true
			if node.Path != sample.slug {
				t.Errorf("ListTree path = %q, want %q", node.Path, sample.slug)
			}
		}
	}
	if !found {
		t.Errorf("expected the sample page in the tree, got %+v", nodes)
	}
	// The synthetic root must never be surfaced.
	for _, node := range nodes {
		if node.ID == "root" {
			t.Fatal("synthetic root leaked into ListTree output")
		}
	}
}

// TestWikiMCPAdapter_ListTree_Empty returns an empty (nil) slice when the tree
// has no nodes, rather than erroring.
func TestWikiMCPAdapter_ListTree_Empty(t *testing.T) {
	w := createWikiTestInstance(t)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	adapter := NewWikiMCPAdapter(w)

	// A fresh wiki has at least the welcome page; delete every page so the tree
	// is empty. This exercises the "no nodes" branch of ListTree.
	if err := w.tree.WalkNodes(func(id string) error {
		return deleteTreePage(t, w, id)
	}); err != nil {
		t.Fatalf("clearing the tree: %v", err)
	}

	nodes, err := adapter.ListTree(context.Background())
	if err != nil {
		t.Fatalf("ListTree error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected an empty tree, got %+v", nodes)
	}
}

// deleteTreePage removes a page by ID regardless of kind, tolerating pages that
// have children (recursively) or are already gone.
func deleteTreePage(t *testing.T, w *Wiki, id string) error {
	t.Helper()
	_, err := w.tree.GetPage(id)
	if err != nil {
		if errors.Is(err, tree.ErrPageNotFound) {
			return nil
		}
		return err
	}
	deletePageForTest(t, w, "system", id, true)
	return nil
}
