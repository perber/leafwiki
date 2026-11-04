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
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
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

// TestWikiMCPAdapter_UpdatePage_Slug renames the page through the optional
// slug: the returned view carries the new slug (the new route path resolves
// to the same page, the old one no longer does).
func TestWikiMCPAdapter_UpdatePage_Slug(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Slugged", Slug: "mcp-slugged",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	updated, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(), ID: created.ID, Slug: "mcp-renamed",
	})
	if err != nil {
		t.Fatalf("UpdatePage (slug) error: %v", err)
	}
	if updated.Path != "mcp-renamed" {
		t.Fatalf("updated Path = %q, want the new slug %q", updated.Path, "mcp-renamed")
	}
	// The new route path resolves to the same page…
	got, err := adapter.GetPageByPath(context.Background(), "mcp-renamed")
	if err != nil {
		t.Fatalf("GetPageByPath (new path) error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("new path resolved to the wrong page: %+v", got)
	}
	// …and the old one is gone.
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-slugged"); err == nil {
		t.Fatal("the old slug still resolves after the rename")
	}
}

// TestWikiMCPAdapter_UpdatePage_SlugOmittedKeepsTheCurrentSlug pins the
// omission: an update that carries no slug keeps the page's current one.
func TestWikiMCPAdapter_UpdatePage_SlugOmittedKeepsTheCurrentSlug(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Slug Kept", Slug: "mcp-kept",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	body := "replaced body"
	updated, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor:   mcpActor(),
		ID:      created.ID,
		Content: &body,
	})
	if err != nil {
		t.Fatalf("UpdatePage (content) error: %v", err)
	}
	if updated.Path != "mcp-kept" {
		t.Fatalf("updated Path = %q, want the unchanged slug %q", updated.Path, "mcp-kept")
	}
}

// TestWikiMCPAdapter_UpdatePage_SlugConflict answers a rename onto a sibling
// slug with the REST-parity code page_slug_conflict (the update use case's
// rename fails on the tree's page-already-exists sentinel, mapped through the
// same table as the REST route).
func TestWikiMCPAdapter_UpdatePage_SlugConflict(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	taker, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Slug Taker", Slug: "mcp-taker",
	})
	if err != nil {
		t.Fatalf("CreatePage (taker) error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Occupant", Slug: "mcp-occupant",
	}); err != nil {
		t.Fatalf("CreatePage (occupant) error: %v", err)
	}

	_, err = adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(), ID: taker.ID, Slug: "mcp-occupant",
	})
	assertPageError(t, err, wikipages.ErrCodePageSlugConflict, "Page already exists")

	// The refused rename is a no-op: the page stays at its old slug.
	got, err := adapter.GetPageByPath(context.Background(), "mcp-taker")
	if err != nil {
		t.Fatalf("GetPageByPath (old path) error: %v", err)
	}
	if got.ID != taker.ID {
		t.Fatalf("the old path resolved to the wrong page: %+v", got)
	}
}

// TestWikiMCPAdapter_UpdatePage_InvalidSlug reports a slug the slug service
// rejects (invalid characters) with the use case's validation error on the
// slug field — which the tool error mapper renders as mcp_invalid_input.
func TestWikiMCPAdapter_UpdatePage_InvalidSlug(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Slug Invalid", Slug: "mcp-invalid-slug",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	_, err = adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(), ID: created.ID, Slug: "not a valid slug",
	})
	var vErr *sharederrors.ValidationErrors
	if !errors.As(err, &vErr) {
		t.Fatalf("expected a *sharederrors.ValidationErrors, got %T (%v)", err, err)
	}
	for _, fe := range vErr.Errors {
		if fe.Field == "slug" {
			return
		}
	}
	t.Fatalf("the validation error names no slug field: %+v", vErr.Errors)
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

// TestWikiMCPAdapter_DeletePage_Effects deletes the page from the tree and the
// search index, and cleans up the per-user favorites and the revision data —
// the same cleanup the REST delete route performs through the delete use case.
func TestWikiMCPAdapter_DeletePage_Effects(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Deletable",
		Content: "doomed body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	// A content change records a revision; the delete must clear it.
	body := "a second body"
	if _, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor:   mcpActor(),
		ID:      created.ID,
		Content: &body,
	}); err != nil {
		t.Fatalf("UpdatePage error: %v", err)
	}
	revsBefore, err := w.revision.ListRevisions(created.ID)
	if err != nil {
		t.Fatalf("ListRevisions before delete error: %v", err)
	}
	if len(revsBefore) == 0 {
		t.Fatal("expected the content change to have recorded a revision")
	}
	if err := w.favorites.Add("u-1", created.ID); err != nil {
		t.Fatalf("favorites.Add error: %v", err)
	}

	if err := adapter.DeletePage(context.Background(), wikimcp.DeletePageInput{Actor: mcpActor(), ID: created.ID}); err != nil {
		t.Fatalf("DeletePage error: %v", err)
	}

	// Gone from the tree…
	if _, err := adapter.GetPage(context.Background(), created.ID); err == nil {
		t.Fatal("GetPage after delete succeeded, want page_not_found")
	}
	// …and from the search index.
	res, err := adapter.Search(context.Background(), "doomed", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	for _, item := range res.Items {
		if item.ID == created.ID {
			t.Fatal("deleted page is still in the search index")
		}
	}
	// Favorites and revision data are cleaned up.
	favIDs, err := w.favorites.ListPageIDsForUser("u-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser error: %v", err)
	}
	for _, id := range favIDs {
		if id == created.ID {
			t.Fatal("deleted page is still in the user's favorites")
		}
	}
	revs, err := w.revision.ListRevisions(created.ID)
	if err != nil {
		t.Fatalf("ListRevisions after delete error: %v", err)
	}
	if len(revs) != 0 {
		t.Fatalf("revision data survived the delete: %+v", revs)
	}
}

// TestWikiMCPAdapter_DeletePage_HasChildren reports a non-recursive delete of a
// page with children with the REST-parity code page_has_children.
func TestWikiMCPAdapter_DeletePage_HasChildren(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	section, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Section",
		Slug:  "mcp-section",
		Kind:  "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:    mcpActor(),
		Title:    "MCP Child",
		ParentID: &section.ID,
	}); err != nil {
		t.Fatalf("CreatePage (child) error: %v", err)
	}

	err = adapter.DeletePage(context.Background(), wikimcp.DeletePageInput{Actor: mcpActor(), ID: section.ID})
	assertPageError(t, err, wikipages.ErrCodePageHasChildren, "Page has children, use recursive delete")
}

// TestWikiMCPAdapter_MovePage_Effects moves the page to a new route path: the
// old path no longer resolves and the new one does.
func TestWikiMCPAdapter_MovePage_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:   mcpActor(),
		Title:   "MCP Movable",
		Content: "moving body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Destination",
		Slug:  "mcp-dest",
		Kind:  "section",
	}); err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}

	if err := adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    created.ID,
		Path:  "mcp-dest/renamed-movable",
	}); err != nil {
		t.Fatalf("MovePage error: %v", err)
	}

	// The new path resolves to the moved page…
	moved, err := adapter.GetPageByPath(context.Background(), "mcp-dest/renamed-movable")
	if err != nil {
		t.Fatalf("GetPageByPath (new path) error: %v", err)
	}
	if moved.ID != created.ID || moved.Content != "moving body" {
		t.Fatalf("new path resolved to the wrong page: %+v", moved)
	}
	// …and the old one is gone.
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-movable"); err == nil {
		t.Fatal("old path still resolves after the move")
	}
}

// TestWikiMCPAdapter_MovePage_ToRoot moves a nested page to the wiki root
// through the empty path (the tool's documented root-move path): the old path
// no longer resolves and the root path does.
func TestWikiMCPAdapter_MovePage_ToRoot(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	nest, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Nest",
		Slug:  "mcp-nest",
		Kind:  "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}
	nested, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:    mcpActor(),
		Title:    "MCP Nested",
		Slug:     "mcp-nested",
		ParentID: &nest.ID,
	})
	if err != nil {
		t.Fatalf("CreatePage (nested) error: %v", err)
	}
	// Pin the nested path down before the move.
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-nest/mcp-nested"); err != nil {
		t.Fatalf("the nested page should resolve before the move: %v", err)
	}

	if err := adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    nested.ID,
		Path:  "",
	}); err != nil {
		t.Fatalf("MovePage (to root) error: %v", err)
	}

	// The root path resolves to the moved page…
	moved, err := adapter.GetPageByPath(context.Background(), "mcp-nested")
	if err != nil {
		t.Fatalf("GetPageByPath (root path) error: %v", err)
	}
	if moved.ID != nested.ID {
		t.Fatalf("root path resolved to the wrong page: %+v", moved)
	}
	// …and the nested one is gone.
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-nest/mcp-nested"); err == nil {
		t.Fatal("the nested path still resolves after the move to root")
	}
}

// TestWikiMCPAdapter_MovePage_RootPageUnchanged is the no-op move: a page
// already at the root, moved to the root with an unchanged slug, is refused
// with the REST-identical page_slug_conflict (the on-disk move onto itself
// fails) and the page must stay reachable at its root path, unchanged.
func TestWikiMCPAdapter_MovePage_RootPageUnchanged(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Rooted",
		Slug:  "mcp-rooted",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	err = adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    created.ID,
		Path:  "",
	})
	assertPageError(t, err, wikipages.ErrCodePageSlugConflict, "Page already exists")

	// The refused no-op leaves the page exactly where it was.
	got, err := adapter.GetPageByPath(context.Background(), "mcp-rooted")
	if err != nil {
		t.Fatalf("GetPageByPath after the no-op move error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("the page did not stay reachable at its root path: %+v", got)
	}
}

// TestWikiMCPAdapter_MovePage_CircularMove reports a move into the page's own
// subtree with the REST-parity code page_circular_move — and, because the
// destination is validated before any mutation, the page is left exactly
// where it was: the old path still resolves and the slug is unchanged.
func TestWikiMCPAdapter_MovePage_CircularMove(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	section, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Cycle Section",
		Slug:  "mcp-cycle",
		Kind:  "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:    mcpActor(),
		Title:    "MCP Cycle Child",
		Slug:     "mcp-cycle-child",
		ParentID: &section.ID,
	}); err != nil {
		t.Fatalf("CreatePage (child) error: %v", err)
	}

	err = adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    section.ID,
		Path:  "mcp-cycle/mcp-cycle-child/x",
	})
	assertPageError(t, err, wikipages.ErrCodePageCircularMove, "Move would create a circular reference")

	// The refused move must be a no-op: the section keeps its path (and so its
	// slug) and a renamed slug would have left the tree.
	got, err := adapter.GetPageByPath(context.Background(), "mcp-cycle")
	if err != nil {
		t.Fatalf("the section's path must survive a circular move: %v", err)
	}
	if got.ID != section.ID || got.Path != "mcp-cycle" {
		t.Fatalf("the circular move changed the page: %+v", got)
	}
}

// TestWikiMCPAdapter_MovePage_ParentNotFound reports a destination whose parent
// path does not exist with the REST-parity code page_parent_not_found (the
// REST move route answers the same code for an unknown parent).
func TestWikiMCPAdapter_MovePage_ParentNotFound(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	created, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Stranded",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	err = adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    created.ID,
		Path:  "no-such-section/stranded",
	})
	assertPageError(t, err, wikipages.ErrCodePageParentNotFound, "Parent page not found")
}

// TestWikiMCPAdapter_MovePage_FailedCollisionLeavesSlugBehind pins the residual
// property stated in the move tool's description: a destination slug collision
// is only discovered by the move itself (after the rename is committed), so a
// failed move may leave the new slug in place — here the page ends up at its
// old location under the new slug.
func TestWikiMCPAdapter_MovePage_FailedCollisionLeavesSlugBehind(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Clash",
		Slug:  "mcp-clash",
		Kind:  "section",
	}); err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}
	mover, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(),
		Title: "MCP Mover",
		Slug:  "mover",
	})
	if err != nil {
		t.Fatalf("CreatePage (mover) error: %v", err)
	}
	// The destination section already has a child with the target slug, so the
	// move will collide even though the rename to "dup" succeeds at root.
	dest, err := adapter.GetPageByPath(context.Background(), "mcp-clash")
	if err != nil {
		t.Fatalf("GetPageByPath (mcp-clash) error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor:    mcpActor(),
		Title:    "MCP Occupant",
		Slug:     "dup",
		ParentID: &dest.ID,
	}); err != nil {
		t.Fatalf("CreatePage (occupant) error: %v", err)
	}

	err = adapter.MovePage(context.Background(), wikimcp.MovePageInput{
		Actor: mcpActor(),
		ID:    mover.ID,
		Path:  "mcp-clash/dup",
	})
	assertPageError(t, err, wikipages.ErrCodePageSlugConflict, "Page already exists")

	// The move failed, but the rename is committed: the page is at its old
	// location (root) under the new slug.
	got, err := adapter.GetPageByPath(context.Background(), "dup")
	if err != nil {
		t.Fatalf("the renamed slug must resolve at the old location: %v", err)
	}
	if got.ID != mover.ID {
		t.Fatalf("the new slug resolved to the wrong page: %+v", got)
	}
	if _, err := adapter.GetPageByPath(context.Background(), "mover"); err == nil {
		t.Fatal("the old slug still resolves after the rename")
	}
}

// TestWikiMCPAdapter_EnsurePath_Effects proves the ensure-path effects on the
// live tree: the missing segments of the path are created (intermediates as
// sections, the last with the given kind), an existing segment is reused
// rather than clobbered (its title survives), and a second call is
// idempotent — the same final node, no duplicate segments, the new title
// ignored for the existing nodes.
func TestWikiMCPAdapter_EnsurePath_Effects(t *testing.T) {
	w, adapter := newMCPWriteTestWiki(t)

	// An existing node the path must reuse (and not rename).
	section, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Ensure Section", Slug: "mcp-ensure-sec", Kind: "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}

	out, err := adapter.EnsurePath(context.Background(), wikimcp.EnsurePathInput{
		Actor: mcpActor(), Path: "mcp-ensure-sec/deep/final-page", Title: "Final Page",
	})
	if err != nil {
		t.Fatalf("EnsurePath error: %v", err)
	}
	if out.Path != "mcp-ensure-sec/deep/final-page" || out.Kind != "page" {
		t.Fatalf("ensured node = %+v, want the final page at the full path", out)
	}

	// Every segment exists with the right kind…
	existing, err := adapter.GetPageByPath(context.Background(), "mcp-ensure-sec")
	if err != nil {
		t.Fatalf("the pre-existing segment must survive: %v", err)
	}
	if existing.ID != section.ID || existing.Kind != "section" {
		t.Fatalf("existing segment = %+v, want the reused section", existing)
	}
	if existing.Title != "MCP Ensure Section" {
		t.Fatalf("existing segment was clobbered: title = %q, want %q", existing.Title, "MCP Ensure Section")
	}
	mid, err := adapter.GetPageByPath(context.Background(), "mcp-ensure-sec/deep")
	if err != nil {
		t.Fatalf("the intermediate segment must be created: %v", err)
	}
	if mid.Kind != "section" {
		t.Fatalf("intermediate kind = %q, want section", mid.Kind)
	}

	// A second call is idempotent: the same final node, no new nodes, and the
	// new title is ignored (existing nodes are not clobbered).
	again, err := adapter.EnsurePath(context.Background(), wikimcp.EnsurePathInput{
		Actor: mcpActor(), Path: "mcp-ensure-sec/deep/final-page", Title: "Renamed Title",
	})
	if err != nil {
		t.Fatalf("second EnsurePath error: %v", err)
	}
	if again.ID != out.ID {
		t.Fatalf("second call ensured a different node: %q, want %q", again.ID, out.ID)
	}
	if again.Title != "Final Page" {
		t.Fatalf("second call clobbered the final node's title: %q", again.Title)
	}

	// …and no duplicate segment was created: exactly one "deep" node anywhere.
	count := 0
	var walk func(nodes []*tree.PageNode)
	walk = func(nodes []*tree.PageNode) {
		for _, n := range nodes {
			if n.Slug == "deep" {
				count++
			}
			walk(n.Children)
		}
	}
	walk(w.tree.GetTree().Children)
	if count != 1 {
		t.Fatalf("duplicate intermediate segments after a second call: %d \"deep\" nodes, want 1", count)
	}
}

// TestWikiMCPAdapter_ConvertNode_Effects converts a page to a section: the
// kind flips and the page's children survive the conversion (still reachable
// at their route paths).
func TestWikiMCPAdapter_ConvertNode_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	page, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Convertible", Slug: "mcp-convertible", Content: "convert body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	child, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Convert Child", Slug: "mcp-convert-child", ParentID: &page.ID,
	})
	if err != nil {
		t.Fatalf("CreatePage (child) error: %v", err)
	}

	if err := adapter.ConvertNode(context.Background(), wikimcp.ConvertNodeInput{
		Actor: mcpActor(), ID: page.ID, Kind: "section",
	}); err != nil {
		t.Fatalf("ConvertNode error: %v", err)
	}

	got, err := adapter.GetPage(context.Background(), page.ID)
	if err != nil {
		t.Fatalf("GetPage after convert error: %v", err)
	}
	if got.Kind != "section" {
		t.Fatalf("kind = %q, want section", got.Kind)
	}
	if got.Content != "convert body" {
		t.Fatalf("content = %q, want the page's body kept", got.Content)
	}
	// The child survives the conversion, still parented under the section.
	gotChild, err := adapter.GetPageByPath(context.Background(), "mcp-convertible/mcp-convert-child")
	if err != nil {
		t.Fatalf("the child must survive the conversion: %v", err)
	}
	if gotChild.ID != child.ID {
		t.Fatalf("child path resolved to the wrong page: %+v", gotChild)
	}
}

// TestWikiMCPAdapter_ConvertNode_SectionToPage pins the success direction the
// Disallowed test's refusal contrasts with: an empty section converts to a
// page, the kind flips, and the node keeps its ID and route path.
func TestWikiMCPAdapter_ConvertNode_SectionToPage(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	section, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Empty Section", Slug: "mcp-empty-sec", Kind: "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}

	if err := adapter.ConvertNode(context.Background(), wikimcp.ConvertNodeInput{
		Actor: mcpActor(), ID: section.ID, Kind: "page",
	}); err != nil {
		t.Fatalf("ConvertNode error: %v", err)
	}

	got, err := adapter.GetPage(context.Background(), section.ID)
	if err != nil {
		t.Fatalf("GetPage after convert error: %v", err)
	}
	if got.Kind != "page" {
		t.Fatalf("kind = %q, want page", got.Kind)
	}
	if got.ID != section.ID {
		t.Fatalf("ID = %q, want the stable ID %q", got.ID, section.ID)
	}
	if got.Path != "mcp-empty-sec" {
		t.Fatalf("path = %q, want the unchanged route path", got.Path)
	}
	// The converted page is reachable by the same route path.
	byPath, err := adapter.GetPageByPath(context.Background(), "mcp-empty-sec")
	if err != nil {
		t.Fatalf("GetPageByPath after convert error: %v", err)
	}
	if byPath.ID != section.ID {
		t.Fatalf("path resolved to the wrong page: %+v", byPath)
	}
}

// TestWikiMCPAdapter_ConvertNode_Disallowed answers a conversion the tree
// refuses — a section that still has children to a page — with the
// REST-parity code page_has_children (the REST convert route answers the same
// code: the tree's section-to-page check runs before the node store's
// not-empty-folder check can surface page_convert_not_allowed), leaving the
// section and its child untouched.
func TestWikiMCPAdapter_ConvertNode_Disallowed(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	section, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Busy Section", Slug: "mcp-busy-sec", Kind: "section",
	})
	if err != nil {
		t.Fatalf("CreatePage (section) error: %v", err)
	}
	if _, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Busy Child", Slug: "mcp-busy-child", ParentID: &section.ID,
	}); err != nil {
		t.Fatalf("CreatePage (child) error: %v", err)
	}

	err = adapter.ConvertNode(context.Background(), wikimcp.ConvertNodeInput{
		Actor: mcpActor(), ID: section.ID, Kind: "page",
	})
	assertPageError(t, err, wikipages.ErrCodePageHasChildren, "Page has children, use recursive delete")

	// The refused conversion is a no-op: still a section, child intact.
	got, err := adapter.GetPage(context.Background(), section.ID)
	if err != nil {
		t.Fatalf("the section must survive a refused convert: %v", err)
	}
	if got.Kind != "section" {
		t.Fatalf("kind = %q after a refused convert, want section", got.Kind)
	}
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-busy-sec/mcp-busy-child"); err != nil {
		t.Fatalf("the child must survive a refused convert: %v", err)
	}
}

// TestWikiMCPAdapter_ConvertNode_NotFound reports an unknown ID with the
// REST-parity code page_not_found, the way the REST convert route answers a
// missing page.
func TestWikiMCPAdapter_ConvertNode_NotFound(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	err := adapter.ConvertNode(context.Background(), wikimcp.ConvertNodeInput{
		Actor: mcpActor(), ID: "no-such-page", Kind: "section",
	})
	assertPageError(t, err, wikipages.ErrCodePageNotFound, "Page not found")
}

// TestWikiMCPAdapter_CopyPage_Effects proves the copy effects: the copy
// exists at the new slug with the source's content, and the source is left
// untouched — the single-page REST copy semantics (a child of the source
// stays with the source).
func TestWikiMCPAdapter_CopyPage_Effects(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	src, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Copy Source", Slug: "mcp-copy-src", Content: "copy body",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	child, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Copy Child", Slug: "mcp-copy-child", ParentID: &src.ID,
	})
	if err != nil {
		t.Fatalf("CreatePage (child) error: %v", err)
	}

	copied, err := adapter.CopyPage(context.Background(), wikimcp.CopyPageInput{
		Actor: mcpActor(), SourceID: src.ID, Title: "MCP Copied", Slug: "mcp-copied",
	})
	if err != nil {
		t.Fatalf("CopyPage error: %v", err)
	}
	if copied.ID == src.ID {
		t.Fatal("the copy shares the source's ID")
	}
	if copied.Path != "mcp-copied" || copied.Kind != "page" {
		t.Fatalf("copy = %+v, want a page at the new root path", copied)
	}
	if copied.Content != "copy body" {
		t.Fatalf("copy content = %q, want the source's content", copied.Content)
	}

	// The source is untouched…
	got, err := adapter.GetPage(context.Background(), src.ID)
	if err != nil {
		t.Fatalf("GetPage (source) error: %v", err)
	}
	if got.Content != "copy body" {
		t.Fatalf("source content = %q, want the original", got.Content)
	}
	// …and its child stays with the source, not the copy.
	still, err := adapter.GetPageByPath(context.Background(), "mcp-copy-src/mcp-copy-child")
	if err != nil {
		t.Fatalf("the source's child must stay under the source: %v", err)
	}
	if still.ID != child.ID {
		t.Fatalf("source child path resolved to the wrong page: %+v", still)
	}
	if _, err := adapter.GetPageByPath(context.Background(), "mcp-copied/mcp-copy-child"); err == nil {
		t.Fatal("the child was copied with the page; the REST copy is single-page")
	}
}

// TestWikiMCPAdapter_CopyPage_ExistingSlugConflict answers a copy onto a slug
// already taken under the same parent with the REST-parity code
// page_slug_conflict (the REST copy route fails the destination create the
// same way).
func TestWikiMCPAdapter_CopyPage_ExistingSlugConflict(t *testing.T) {
	_, adapter := newMCPWriteTestWiki(t)

	src, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Copy Source", Slug: "mcp-copy-src",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	_, err = adapter.CopyPage(context.Background(), wikimcp.CopyPageInput{
		Actor: mcpActor(), SourceID: src.ID, Title: "MCP Copied Again", Slug: "mcp-copy-src",
	})
	assertPageError(t, err, wikipages.ErrCodePageSlugConflict, "Page already exists")
}

// ─── Revision ops ────────────────────────────────────────────────────────────

type revisionSample struct {
	id     string
	first  string
	second string
}

// newMCPRevisionTestWiki is newMCPWriteTestWiki with one page saved twice, so
// its history holds at least two content revisions (the first and the second
// saved content, newest first in the list).
func newMCPRevisionTestWiki(t *testing.T) (*Wiki, *WikiMCPAdapter, *revisionSample) {
	t.Helper()
	w, adapter := newMCPWriteTestWiki(t)

	page, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "MCP Revisions Sample", Slug: "mcp-revisions-sample",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}
	first := "first version of the sample page"
	if _, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(), ID: page.ID, Content: &first,
	}); err != nil {
		t.Fatalf("UpdatePage (first) error: %v", err)
	}
	second := "second version of the sample page"
	if _, err := adapter.UpdatePage(context.Background(), wikimcp.UpdatePageInput{
		Actor: mcpActor(), ID: page.ID, Content: &second,
	}); err != nil {
		t.Fatalf("UpdatePage (second) error: %v", err)
	}
	return w, adapter, &revisionSample{id: page.ID, first: first, second: second}
}

// revisionByContent returns the entry of the listed history whose content is
// the given one, failing the test if none carries it.
func revisionByContent(t *testing.T, adapter *WikiMCPAdapter, pageID string, revisions []wikimcp.Revision, content string) wikimcp.Revision {
	t.Helper()
	for _, r := range revisions {
		snap, err := adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: pageID, RevisionID: r.ID})
		if err != nil {
			t.Fatalf("GetRevision(%s) error: %v", r.ID, err)
		}
		if snap.Content == content {
			return r
		}
	}
	t.Fatalf("no listed revision carries content %q", content)
	return wikimcp.Revision{}
}

// assertRevisionError asserts err is the expected revisions-domain
// LocalizedError with its client-facing code and message.
func assertRevisionError(t *testing.T, err error, wantCode, wantMessage string) {
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

// TestWikiMCPAdapter_ListRevisions returns the history newest first: the
// newest entry postdates the oldest and holds the latest saved content.
func TestWikiMCPAdapter_ListRevisions(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	if len(res.Revisions) < 2 {
		t.Fatalf("expected at least two revisions after two saves, got %d", len(res.Revisions))
	}
	newest, oldest := res.Revisions[0], res.Revisions[len(res.Revisions)-1]
	if newest.ID == oldest.ID {
		t.Fatalf("expected distinct revision IDs, got %q twice", newest.ID)
	}
	if newest.CreatedAt.Before(oldest.CreatedAt) {
		t.Errorf("expected newest first; newest %q (at %s) predates oldest %q (at %s)",
			newest.ID, newest.CreatedAt, oldest.ID, oldest.CreatedAt)
	}
	snap, err := adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: sample.id, RevisionID: newest.ID})
	if err != nil {
		t.Fatalf("GetRevision (newest) error: %v", err)
	}
	if snap.Content != sample.second {
		t.Errorf("newest revision content = %q, want the second saved content %q", snap.Content, sample.second)
	}
}

// TestWikiMCPAdapter_GetRevision reads each saved content state back by the
// ID the list reports.
func TestWikiMCPAdapter_GetRevision(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	for _, want := range []string{sample.first, sample.second} {
		rev := revisionByContent(t, adapter, sample.id, res.Revisions, want)
		snap, err := adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: sample.id, RevisionID: rev.ID})
		if err != nil {
			t.Fatalf("GetRevision(%s) error: %v", rev.ID, err)
		}
		if snap.Revision.ID != rev.ID {
			t.Errorf("snapshot revision ID = %q, want %q", snap.Revision.ID, rev.ID)
		}
		if snap.Content != want {
			t.Errorf("GetRevision(%s) content = %q, want %q", rev.ID, snap.Content, want)
		}
	}
}

// TestWikiMCPAdapter_CompareRevisions flags the content change between the two
// saved states, and no change for a revision compared with itself.
func TestWikiMCPAdapter_CompareRevisions(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	base := revisionByContent(t, adapter, sample.id, res.Revisions, sample.first)
	target := res.Revisions[0]

	cmp, err := adapter.CompareRevisions(context.Background(), wikimcp.CompareRevisionsInput{
		ID: sample.id, BaseRevisionID: base.ID, TargetRevisionID: target.ID,
	})
	if err != nil {
		t.Fatalf("CompareRevisions error: %v", err)
	}
	if !cmp.ContentChanged {
		t.Error("expected ContentChanged for two different saved states")
	}
	if cmp.Base.Content != sample.first || cmp.Target.Content != sample.second {
		t.Errorf("base content = %q, target content = %q; want the first and second saved contents",
			cmp.Base.Content, cmp.Target.Content)
	}

	same, err := adapter.CompareRevisions(context.Background(), wikimcp.CompareRevisionsInput{
		ID: sample.id, BaseRevisionID: base.ID, TargetRevisionID: base.ID,
	})
	if err != nil {
		t.Fatalf("CompareRevisions (same) error: %v", err)
	}
	if same.ContentChanged {
		t.Error("a revision compared with itself must not change the content")
	}
	if len(same.AssetChanges) != 0 {
		t.Errorf("a revision compared with itself must have no asset changes, got %+v", same.AssetChanges)
	}
}

// TestWikiMCPAdapter_RestoreRevision reverts the page to an earlier saved
// state through the same use case the REST route uses: the page's content
// becomes the restored one, and the restore itself is recorded — the history
// grows by one with a "restore"-type entry at the top carrying that content.
func TestWikiMCPAdapter_RestoreRevision(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	base := revisionByContent(t, adapter, sample.id, res.Revisions, sample.first)
	before := len(res.Revisions)

	page, err := adapter.RestoreRevision(context.Background(), wikimcp.RestoreRevisionInput{
		Actor: mcpActor(), ID: sample.id, RevisionID: base.ID,
	})
	if err != nil {
		t.Fatalf("RestoreRevision error: %v", err)
	}
	if page.Content != sample.first {
		t.Errorf("restored page content = %q, want %q", page.Content, sample.first)
	}

	after, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions (after restore) error: %v", err)
	}
	if len(after.Revisions) != before+1 {
		t.Fatalf("history grew from %d to %d, want %d (the restore is recorded)",
			before, len(after.Revisions), before+1)
	}
	if top := after.Revisions[0]; top.Type != "restore" {
		t.Errorf("top revision type = %q, want %q", top.Type, "restore")
	}
	snap, err := adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: sample.id, RevisionID: after.Revisions[0].ID})
	if err != nil {
		t.Fatalf("GetRevision (restored) error: %v", err)
	}
	if snap.Content != sample.first {
		t.Errorf("restored revision content = %q, want %q", snap.Content, sample.first)
	}
}

// TestWikiMCPAdapter_RestoreRevision_CurrentRevision pins the no-op restore:
// restoring a page to the revision it already carries succeeds and the
// content is unchanged — but the restore is still recorded, so the history
// grows by one with a restore-type entry at the top.
func TestWikiMCPAdapter_RestoreRevision_CurrentRevision(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	current := res.Revisions[0]
	before := len(res.Revisions)

	page, err := adapter.RestoreRevision(context.Background(), wikimcp.RestoreRevisionInput{
		Actor: mcpActor(), ID: sample.id, RevisionID: current.ID,
	})
	if err != nil {
		t.Fatalf("RestoreRevision (current) error: %v", err)
	}
	if page.Content != sample.second {
		t.Errorf("page content = %q, want the unchanged %q", page.Content, sample.second)
	}

	after, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions (after no-op restore) error: %v", err)
	}
	if len(after.Revisions) != before+1 {
		t.Fatalf("history grew from %d to %d, want %d (a no-op restore is still recorded)",
			before, len(after.Revisions), before+1)
	}
	if top := after.Revisions[0]; top.Type != "restore" {
		t.Errorf("top revision type = %q, want %q", top.Type, "restore")
	}
}

// TestWikiMCPAdapter_GetRevision_Unknown pins the os.ErrNotExist arm of the
// revision error mapper: the store's not-found error is not a LocalizedError,
// so the adapter answers with the code the REST route answers for the same
// request, revision_not_found.
func TestWikiMCPAdapter_GetRevision_Unknown(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	_, err := adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: sample.id, RevisionID: "no-such-revision"})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionNotFound, "Revision resource not found")
}

// TestWikiMCPAdapter_CompareRevisions_Unknown pins the same mapping for the
// compare op, with the unknown revision in the base slot.
func TestWikiMCPAdapter_CompareRevisions_Unknown(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	_, err := adapter.CompareRevisions(context.Background(), wikimcp.CompareRevisionsInput{
		ID: sample.id, BaseRevisionID: "no-such-revision", TargetRevisionID: "also-unknown",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionNotFound, "Revision resource not found")
}

// TestWikiMCPAdapter_CompareRevisions_UnknownTarget pins the same mapping for
// the unknown revision in the target slot (the unknown base is pinned in
// TestWikiMCPAdapter_CompareRevisions_Unknown).
func TestWikiMCPAdapter_CompareRevisions_UnknownTarget(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: sample.id})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	known := res.Revisions[0].ID

	_, err = adapter.CompareRevisions(context.Background(), wikimcp.CompareRevisionsInput{
		ID: sample.id, BaseRevisionID: known, TargetRevisionID: "no-such-revision",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionNotFound, "Revision resource not found")
}

// TestWikiMCPAdapter_RevisionOps_RevisionDisabled pins the intended flag-off
// behaviour: with EnableRevision false (like the REST revision routes, which
// hide behind the flag) the ops degrade uniformly — an empty list, the
// not-found code for the reads, the service-unavailable code for restore —
// with no page-existence leak — instead of erroring.
func TestWikiMCPAdapter_RevisionOps_RevisionDisabled(t *testing.T) {
	w, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(w.Close, t) })
	adapter := NewWikiMCPAdapter(w)

	page, err := adapter.CreatePage(context.Background(), wikimcp.CreatePageInput{
		Actor: mcpActor(), Title: "Flag Off Sample", Slug: "flag-off-sample",
	})
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	res, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: page.ID})
	if err != nil {
		t.Fatalf("ListRevisions error: %v", err)
	}
	if len(res.Revisions) != 0 {
		t.Fatalf("flag-off list returned %d revisions, want none", len(res.Revisions))
	}

	_, err = adapter.GetRevision(context.Background(), wikimcp.GetRevisionInput{ID: page.ID, RevisionID: "no-such-revision"})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionNotFound, "Revision not found")

	_, err = adapter.CompareRevisions(context.Background(), wikimcp.CompareRevisionsInput{
		ID: page.ID, BaseRevisionID: "r-1", TargetRevisionID: "r-2",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionNotFound, "Revision not found")

	_, err = adapter.RestoreRevision(context.Background(), wikimcp.RestoreRevisionInput{
		Actor: mcpActor(), ID: page.ID, RevisionID: "no-such-revision",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionServiceUnavailable, "Revision service is not available")
}

// TestWikiMCPAdapter_ListRevisions_InvalidPageID pins the mapper's default
// arm: a page ID the store rejects (a path separator) is a plain error, so
// the adapter answers revision_internal_error, the code the REST route
// answers for the same request.
func TestWikiMCPAdapter_ListRevisions_InvalidPageID(t *testing.T) {
	_, adapter, _ := newMCPRevisionTestWiki(t)

	_, err := adapter.ListRevisions(context.Background(), wikimcp.ListRevisionsInput{ID: "a/b"})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionInternalError, "Revision request failed")
}

// TestWikiMCPAdapter_RestoreRevision_UnknownRevision pins the restore use
// case's own code for an unknown revision: it is a LocalizedError, so the
// adapter passes it through unchanged.
func TestWikiMCPAdapter_RestoreRevision_UnknownRevision(t *testing.T) {
	_, adapter, sample := newMCPRevisionTestWiki(t)

	_, err := adapter.RestoreRevision(context.Background(), wikimcp.RestoreRevisionInput{
		Actor: mcpActor(), ID: sample.id, RevisionID: "no-such-revision",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionRestoreRevisionNotFound, "Restore revision not found")
}

// TestWikiMCPAdapter_RestoreRevision_UnknownPage pins the restore use case's
// code for a page that does not exist.
func TestWikiMCPAdapter_RestoreRevision_UnknownPage(t *testing.T) {
	_, adapter, _ := newMCPRevisionTestWiki(t)

	_, err := adapter.RestoreRevision(context.Background(), wikimcp.RestoreRevisionInput{
		Actor: mcpActor(), ID: "no-such-page", RevisionID: "no-such-revision",
	})
	assertRevisionError(t, err, wikirevisions.ErrCodeRevisionRestorePageNotFound, "Page not found")
}
