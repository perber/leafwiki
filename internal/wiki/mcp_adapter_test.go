package wiki

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	wikimcp "github.com/perber/wiki/internal/wiki/mcp"
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

// ─── write ops ───────────────────────────────────────────────────────────────

// newMCPWriteTestWiki boots a real wiki with revision history enabled (so the
// write ops' revision side effects and cleanup are observable) and returns it
// along with the adapter under test.
func newMCPWriteTestWiki(t *testing.T) (*Wiki, *WikiMCPAdapter) {
	t.Helper()
	w, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableRevision:      true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	return w, NewWikiMCPAdapter(w)
}

func mcpActor() wikimcp.Actor {
	return wikimcp.Actor{ID: "u-1"}
}

// updatePageMetadataForTest applies a UI-style structured metadata update (the
// REST update route's Tags field, with the body resent as the editor does) so
// a page carries frontmatter the write ops must not report stale.
func updatePageMetadataForTest(t *testing.T, w *Wiki, userID, id string, tags []string) *tree.Page {
	t.Helper()
	current, err := w.tree.GetPage(id)
	if err != nil {
		t.Fatalf("GetPage before metadata update failed: %v", err)
	}
	body := current.Content
	out, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log, nil).Execute(
		context.Background(),
		wikipages.UpdatePageInput{UserID: userID, ID: id, Version: current.Version(), Title: current.Title, Slug: current.Slug, Content: &body, Kind: pageNodeKind(), Tags: tags},
	)
	if err != nil {
		t.Fatalf("UpdatePage (metadata) failed: %v", err)
	}
	return out.Page
}

// TestWikiMCPAdapter_CreatePage_Effects proves the create tool's effects on the
// live wiki: the page exists in the tree with the right content, and the
// search index sees it (the orchestrator's side effects fire exactly as they
// do for the REST create route).
func TestWikiMCPAdapter_CreatePage_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	page, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Created",
		Content: "brand new searchable content",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	if page.ID == "" || page.Title != "MCP Created" {
		t.Fatalf("CreatePage returned a page without identity: %+v", page)
	}
	if page.Content != "brand new searchable content" {
		t.Fatalf("Content = %q, want the created content", page.Content)
	}

	// The page is readable back by ID…
	got, err := adapter.GetPage(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("GetPage after create error: %v", err)
	}
	if got.Content != "brand new searchable content" {
		t.Fatalf("GetPage content = %q, want the created content", got.Content)
	}
	// …and the search index picked it up through the save side effects.
	res, err := adapter.Search(context.Background(), "brand new searchable content", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	found := false
	for _, item := range res.Items {
		if item.ID == page.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected the created page in the search index, got %+v", res.Items)
	}
}

// TestWikiMCPAdapter_CreatePage_OmittedSlugDeduplicatesAmongSiblings pins the
// slug suggestion: an omitted slug is derived from the title and deduplicated
// among the siblings through the same SuggestSlugUseCase the REST
// slug-suggestion route uses.
func TestWikiMCPAdapter_CreatePage_OmittedSlugDeduplicatesAmongSiblings(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	first, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{Actor: mcpActor(), Title: "Twin Page"})
	if err != nil {
		t.Fatalf("CreatePage (first) error: %v", err)
	}
	second, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{Actor: mcpActor(), Title: "Twin Page"})
	if err != nil {
		t.Fatalf("CreatePage (second) error: %v", err)
	}
	if first.Path == second.Path {
		t.Fatalf("both pages share the path %q; the second slug must be deduplicated", first.Path)
	}
}

// TestWikiMCPAdapter_CreatePage_ParentNotFound pins REST parity for a missing
// parent: the REST create route answers an unknown parentId with
// page_not_found (the use case's parent check passes the raw
// tree.ErrPageNotFound through to respondWithPageError), and the adapter must
// report the same code — not page_parent_not_found, which the REST create
// route never answers.
func TestWikiMCPAdapter_CreatePage_ParentNotFound(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	missing := "no-such-parent"
	_, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:    mcpActor(),
		Title:    "Orphan",
		ParentID: &missing,
	})
	assertPageError(t, err, wikipages.ErrCodePageNotFound, "Page not found")
}

// TestWikiMCPAdapter_CreatePage_RollsBackHalfCreatedOnGenuineFailure proves a
// failed content set leaves no orphan page behind: removeHalfCreated removes
// the half-created page for every failure other than a version conflict,
// where a concurrent writer already owns the page and must keep it.
func TestWikiMCPAdapter_CreatePage_RollsBackHalfCreatedOnGenuineFailure(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	halfCreated := createPageForTest(t, w, "u-1", nil, "Half Created", "half-created", pageNodeKind())
	page, err := w.tree.GetPage(halfCreated.ID)
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	// A genuine failure (a wrapped storage error lands in the write mapper's
	// internal-error arm): the cleanup must run and remove the page.
	genuine := sharederrors.NewLocalizedError(wikipages.ErrCodePageInternalError, "could not write page content", "internal error", errors.New("disk full"))
	adapter.removeHalfCreated(context.Background(), "u-1", page, genuine)
	if _, err := w.tree.GetPage(halfCreated.ID); err == nil {
		t.Fatal("the half-created page survived a genuine content failure")
	}
}

// TestWikiMCPAdapter_CreatePage_KeepsHalfCreatedOnVersionConflict proves the
// cleanup skip: a content failure reported as page_version_conflict means a
// concurrent writer already saved to the page, so removeHalfCreated must not
// delete it.
func TestWikiMCPAdapter_CreatePage_KeepsHalfCreatedOnVersionConflict(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	saved := createPageForTest(t, w, "u-1", nil, "Concurrent Save", "concurrent-save", pageNodeKind())
	page, err := w.tree.GetPage(saved.ID)
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	conflict := sharederrors.NewLocalizedError(wikipages.ErrCodePageVersionConflict, "Page was changed by another request", "page was changed by another request", tree.ErrVersionConflict)
	adapter.removeHalfCreated(context.Background(), "u-1", page, conflict)
	if _, err := w.tree.GetPage(saved.ID); err != nil {
		t.Fatalf("the concurrently saved page was deleted on a version conflict: %v", err)
	}
}

// TestWikiMCPAdapter_UpdatePage_Effects proves the update tool changes the
// live page: a content-only update replaces the body and keeps the title, a
// title-only update renames the page and keeps the body.
func TestWikiMCPAdapter_UpdatePage_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Updatable",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	body := "freshly replaced body"
	updated, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor:   mcpActor(),
		ID:      created.ID,
		Content: &body,
	})
	if err != nil {
		t.Fatalf("UpdatePage (content) error: %v", err)
	}
	if updated.Content != body {
		t.Fatalf("updated Content = %q, want %q", updated.Content, body)
	}
	if updated.Title != "MCP Updatable" {
		t.Fatalf("updated Title = %q, want the unchanged title", updated.Title)
	}
	got, err := adapter.GetPage(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetPage after update error: %v", err)
	}
	if got.Content != body {
		t.Fatalf("live Content = %q, want the updated body", got.Content)
	}

	renamed, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(),
		ID:    created.ID,
		Title: "MCP Renamed",
	})
	if err != nil {
		t.Fatalf("UpdatePage (title) error: %v", err)
	}
	if renamed.Title != "MCP Renamed" || renamed.Content != body {
		t.Fatalf("renamed page = %+v, want the new title with the body kept", renamed)
	}
}

// TestWikiMCPAdapter_UpdatePage_MetadataReflectsThePageAfterTheWrite proves
// the result's tags and properties are the page's own after the write, not a
// cached pre-update view: the page's frontmatter carries tags (set the way
// the UI sets them), a content replacement keeps them (body-only writes
// leave editor-owned frontmatter untouched), so the updated result must
// report them — matching the fresh read, the way the REST update route
// re-reads the metadata for its response.
func TestWikiMCPAdapter_UpdatePage_MetadataReflectsThePageAfterTheWrite(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Metadata",
		Content: "original body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	updatePageMetadataForTest(t, w, "u-1", created.ID, []string{"kept"})

	body := "replaced body"
	updated, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor:   mcpActor(),
		ID:      created.ID,
		Content: &body,
	})
	if err != nil {
		t.Fatalf("UpdatePage error: %v", err)
	}
	if updated.Content != body {
		t.Fatalf("updated Content = %q, want %q", updated.Content, body)
	}
	// The frontmatter survives the content replacement…
	if len(updated.Tags) != 1 || updated.Tags[0] != "kept" {
		t.Fatalf("updated Tags = %v, want the surviving [\"kept\"]", updated.Tags)
	}
	// …and the result's metadata is what the page has after the write,
	// per a fresh read.
	fresh, err := adapter.GetPage(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetPage after update error: %v", err)
	}
	if len(updated.Tags) != len(fresh.Tags) || (len(fresh.Tags) > 0 && updated.Tags[0] != fresh.Tags[0]) {
		t.Fatalf("updated Tags = %v, want the fresh read's %v", updated.Tags, fresh.Tags)
	}
}

// TestWikiMCPAdapter_PatchPage_Effects applies a line-range patch to the live
// page: the returned and the re-read page carry the patched body.
func TestWikiMCPAdapter_PatchPage_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Patchable",
		Content: "alpha\nbeta\ngamma",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	patched, err := adapter.PatchPage(context.Background(), wikimcp.PatchPageInput{
		Actor:  mcpActor(),
		ID:     created.ID,
		Ranges: []wikimcp.PatchRange{{Start: 2, End: 2, Replace: "BETA"}},
	})
	if err != nil {
		t.Fatalf("PatchPage error: %v", err)
	}
	if want := "alpha\nBETA\ngamma"; patched.Content != want {
		t.Fatalf("patched Content = %q, want %q", patched.Content, want)
	}
	got, err := adapter.GetPage(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetPage after patch error: %v", err)
	}
	if want := "alpha\nBETA\ngamma"; got.Content != want {
		t.Fatalf("live Content = %q, want %q", got.Content, want)
	}
}

// TestWikiMCPAdapter_PatchPage_OutOfRange reports a range past the end of the
// page as a validation error on the patch field, which the tool error mapper
// renders as mcp_invalid_input (a clean error, not a 500-equivalent).
func TestWikiMCPAdapter_PatchPage_OutOfRange(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Short",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	_, err = adapter.PatchPage(context.Background(), wikimcp.PatchPageInput{
		Actor:  mcpActor(),
		ID:     created.ID,
		Ranges: []wikimcp.PatchRange{{Start: 1, End: 99, Replace: "x"}},
	})
	var vErr *sharederrors.ValidationErrors
	if !errors.As(err, &vErr) {
		t.Fatalf("expected a *sharederrors.ValidationErrors, got %T (%v)", err, err)
	}
}

// TestWikiMCPAdapter_PatchPage_MetadataReflectsThePageAfterTheWrite proves the
// patch result is re-read after the write (as UpdatePage is), so its tags and
// properties are the page's own after the patch, per a fresh read.
func TestWikiMCPAdapter_PatchPage_MetadataReflectsThePageAfterTheWrite(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Patch Metadata",
		Content: "alpha\nbeta\ngamma",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	updatePageMetadataForTest(t, w, "u-1", created.ID, []string{"kept"})

	patched, err := adapter.PatchPage(context.Background(), wikimcp.PatchPageInput{
		Actor:  mcpActor(),
		ID:     created.ID,
		Ranges: []wikimcp.PatchRange{{Start: 2, End: 2, Replace: "BETA"}},
	})
	if err != nil {
		t.Fatalf("PatchPage error: %v", err)
	}
	fresh, err := adapter.GetPage(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetPage after patch error: %v", err)
	}
	if len(patched.Tags) != len(fresh.Tags) || (len(fresh.Tags) > 0 && patched.Tags[0] != fresh.Tags[0]) {
		t.Fatalf("patched Tags = %v, want the fresh read's %v", patched.Tags, fresh.Tags)
	}
}
