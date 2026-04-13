package wiki

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/revision"
	verrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

func createWikiTestInstance(t *testing.T) *Wiki {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return wikiInstance
}

func pageNodeKind() *tree.NodeKind {
	kind := tree.NodeKindPage
	return &kind
}

func TestWiki_CreatePage_Root(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, err := w.CreatePage("system", nil, "Home", "home", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if page.Title != "Home" {
		t.Errorf("Expected title 'Home', got %q", page.Title)
	}
}

func TestWiki_CreatePage_WithParent(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	kind := tree.NodeKindPage
	rootPage, _ := w.CreatePage("system", nil, "Docs", "docs", &kind)

	page, err := w.CreatePage("system", &rootPage.ID, "API-Doc", "api-doc", &kind)
	if err != nil {
		t.Fatalf("CreatePage with parent failed: %v", err)
	}

	if page.Parent.ID != rootPage.ID {
		t.Errorf("Expected parent ID %q, got %q", rootPage.ID, page.Parent.ID)
	}
}

func TestWiki_CreatePage_EmptyTitle(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, err := w.CreatePage("system", nil, "", "empty", pageNodeKind())
	if err == nil {
		t.Error("Expected error for empty title, got none")
	}
}

func TestWiki_CreatePage_ReservedSlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, err := w.CreatePage("system", nil, "Reserved", "e", pageNodeKind())
	if err == nil {
		t.Error("Expected error for reserved slug, got none")
	}

	// Check if the error message is correct
	if ve, ok := err.(*verrors.ValidationErrors); ok {
		if len(ve.Errors) != 1 || ve.Errors[0].Field != "slug" {
			t.Errorf("Expected validation error for slug, got %v", ve)
		}
	} else {
		t.Errorf("Expected ValidationErrors, got %T", err)
	}
}

func TestWiki_CreatePage_ReservedHistorySlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	_, err := w.CreatePage("system", nil, "Reserved", "history", pageNodeKind())
	if err == nil {
		t.Fatal("Expected error for reserved history slug, got none")
	}
}

func TestWiki_UpdatePage_AllowsUppercaseSlug(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, err := w.CreatePage("system", nil, "Original", "original", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	content := "# Updated"
	updated, err := w.UpdatePage("system", page.ID, "Original", "ABCD-efg", &content, pageNodeKind())
	if err != nil {
		t.Fatalf("expected uppercase slug update to succeed, got %v", err)
	}
	if updated.Slug != "ABCD-efg" {
		t.Fatalf("expected slug to be preserved, got %q", updated.Slug)
	}
}

func TestWiki_CreatePage_RejectsCaseInsensitiveSlugConflict(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	if _, err := w.CreatePage("system", nil, "Upper", "ABCD-efg", pageNodeKind()); err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	_, err := w.CreatePage("system", nil, "Lower", "abcd-efg", pageNodeKind())
	if err == nil {
		t.Fatal("expected conflict for case-insensitive duplicate slug")
	}
}

func TestWiki_CreatePage_PageExists(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, _ = w.CreatePage("system", nil, "Duplicate", "duplicate", pageNodeKind())

	_, err := w.CreatePage("system", nil, "Duplicate", "duplicate", pageNodeKind())
	if err == nil {
		t.Error("Expected error for duplicate page, got none")
	}
}

func TestWiki_CreatePage_InvalidParent(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	invalidID := "not-real"
	_, err := w.CreatePage("system", &invalidID, "Broken", "broken", pageNodeKind())
	if err == nil {
		t.Error("Expected error with invalid parent ID, got none")
	}
}

func TestWiki_GetPage_ValidID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	page, _ := w.CreatePage("system", nil, "ReadMe", "readme", pageNodeKind())

	found, err := w.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if found.ID != page.ID {
		t.Errorf("Expected ID %q, got %q", page.ID, found.ID)
	}
}

func TestWiki_GetPage_InvalidID(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, err := w.GetPage("unknown")
	if err == nil {
		t.Error("Expected error for unknown ID, got none")
	}
}

func TestWiki_MovePage_Valid(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent, _ := w.CreatePage("system", nil, "Projects", "projects", pageNodeKind())
	child, _ := w.CreatePage("system", nil, "Old", "old", pageNodeKind())

	err := w.MovePage("system", child.ID, parent.ID)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}
}

func TestWiki_PreviewPageRefactor_RenameListsAffectedPages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	target, _ := w.CreatePage("system", nil, "Target", "target", pageNodeKind())
	ref, _ := w.CreatePage("system", nil, "Ref", "ref", pageNodeKind())
	content := "[Target](/target)"
	if _, err := w.UpdatePage("system", ref.ID, ref.Title, ref.Slug, &content, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	preview, err := w.PreviewPageRefactor(target.ID, PageRefactorPreviewRequest{
		Kind:  PageRefactorKindRename,
		Title: target.Title,
		Slug:  "target-renamed",
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}

	if preview.OldPath != "/target" {
		t.Fatalf("OldPath = %q, want %q", preview.OldPath, "/target")
	}
	if preview.NewPath != "/target-renamed" {
		t.Fatalf("NewPath = %q, want %q", preview.NewPath, "/target-renamed")
	}
	if preview.Counts.AffectedPages != 1 {
		t.Fatalf("AffectedPages = %d, want 1", preview.Counts.AffectedPages)
	}
	if len(preview.AffectedPages) != 1 {
		t.Fatalf("expected 1 affected page, got %d", len(preview.AffectedPages))
	}
	if preview.AffectedPages[0].FromPageID != ref.ID {
		t.Fatalf("FromPageID = %q, want %q", preview.AffectedPages[0].FromPageID, ref.ID)
	}
}

func TestWiki_ApplyPageRefactor_RenameRewritesIncomingLinks(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	target, _ := w.CreatePage("system", nil, "Target", "target", pageNodeKind())
	ref, _ := w.CreatePage("system", nil, "Ref", "ref", pageNodeKind())
	content := "[Target](/target)"
	if _, err := w.UpdatePage("system", ref.ID, ref.Title, ref.Slug, &content, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}
	beforeRefRevision, err := w.GetLatestRevision(ref.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(ref before refactor) failed: %v", err)
	}

	updated, err := w.ApplyPageRefactor("system", target.ID, ApplyPageRefactorRequest{
		PageRefactorPreviewRequest: PageRefactorPreviewRequest{
			Kind:    PageRefactorKindRename,
			Title:   "Target Renamed",
			Slug:    "target-renamed",
			Content: &target.Content,
		},
		RewriteLinks: true,
	})
	if err != nil {
		t.Fatalf("ApplyPageRefactor failed: %v", err)
	}

	if updated.CalculatePath() != "/target-renamed" {
		t.Fatalf("updated path mismatch: %q", updated.CalculatePath())
	}

	refPage, err := w.GetPage(ref.ID)
	if err != nil {
		t.Fatalf("GetPage(ref) failed: %v", err)
	}
	if refPage.Content != "[Target](/target-renamed)" {
		t.Fatalf("ref content = %q, want %q", refPage.Content, "[Target](/target-renamed)")
	}

	outgoing, err := w.GetOutgoingLinks(ref.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if outgoing.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", outgoing.Count)
	}
	if outgoing.Outgoings[0].ToPath != "/target-renamed" {
		t.Fatalf("ToPath = %q, want %q", outgoing.Outgoings[0].ToPath, "/target-renamed")
	}
	if outgoing.Outgoings[0].Broken {
		t.Fatalf("expected rewritten link to be healed")
	}

	afterRefRevision, err := w.GetLatestRevision(ref.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(ref after refactor) failed: %v", err)
	}
	if afterRefRevision == nil {
		t.Fatalf("expected latest revision for rewritten ref page")
	}
	if beforeRefRevision == nil {
		t.Fatalf("expected initial revision for ref page")
	}
	if afterRefRevision.ID == beforeRefRevision.ID {
		t.Fatalf("expected rewritten ref page to create a new revision")
	}
	if afterRefRevision.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("expected rewritten ref page latest revision type %q, got %q", revision.RevisionTypeContentUpdate, afterRefRevision.Type)
	}
}

func TestWiki_PreviewPageRefactor_UsesEmptyWarningArrays(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, _ := w.CreatePage("system", nil, "Target", "target", pageNodeKind())

	preview, err := w.PreviewPageRefactor(page.ID, PageRefactorPreviewRequest{
		Kind:  PageRefactorKindRename,
		Title: page.Title,
		Slug:  "target-renamed",
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}

	if preview.Warnings == nil {
		t.Fatalf("expected preview warnings to be an empty slice, got nil")
	}
	if len(preview.Warnings) != 0 {
		t.Fatalf("expected no preview warnings, got %d", len(preview.Warnings))
	}
	for i, affected := range preview.AffectedPages {
		if affected.Warnings == nil {
			t.Fatalf("affected page %d warnings should be empty slice, got nil", i)
		}
		if affected.MatchedPaths == nil {
			t.Fatalf("affected page %d matched paths should be empty slice, got nil", i)
		}
	}
}

func TestWiki_PreviewPageRefactor_Move_ExcludesMovedSubtreeFromOptionalAffectedPages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	docs, _ := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	pageA, _ := w.CreatePage("system", &docs.ID, "Page A", "page-a", pageNodeKind())
	pageB, _ := w.CreatePage("system", &docs.ID, "Page B", "page-b", pageNodeKind())
	archive, _ := w.CreatePage("system", nil, "Archive", "archive", pageNodeKind())

	contentA := "[To B](../page-b)"
	if _, err := w.UpdatePage("system", pageA.ID, pageA.Title, pageA.Slug, &contentA, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage(pageA) failed: %v", err)
	}
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	preview, err := w.PreviewPageRefactor(pageA.ID, PageRefactorPreviewRequest{
		Kind:        PageRefactorKindMove,
		NewParentID: &archive.ID,
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}

	if preview.Counts.AffectedPages != 0 {
		t.Fatalf("expected no optional affected pages, got %d", preview.Counts.AffectedPages)
	}
	if len(preview.AffectedPages) != 0 {
		t.Fatalf("expected no affected pages, got %d", len(preview.AffectedPages))
	}

	_ = pageB
}

func TestWiki_ApplyPageRefactor_Move_RewritesRelativeOutgoingLinksInMovedPage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	docs, _ := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	pageA, _ := w.CreatePage("system", &docs.ID, "Page A", "page-a", pageNodeKind())
	pageB, _ := w.CreatePage("system", &docs.ID, "Page B", "page-b", pageNodeKind())
	archive, _ := w.CreatePage("system", nil, "Archive", "archive", pageNodeKind())

	contentA := "[To B](../page-b)"
	if _, err := w.UpdatePage("system", pageA.ID, pageA.Title, pageA.Slug, &contentA, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage(pageA) failed: %v", err)
	}
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}
	beforeMovedRevision, err := w.GetLatestRevision(pageA.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(pageA before refactor) failed: %v", err)
	}

	updated, err := w.ApplyPageRefactor("system", pageA.ID, ApplyPageRefactorRequest{
		PageRefactorPreviewRequest: PageRefactorPreviewRequest{
			Kind:        PageRefactorKindMove,
			NewParentID: &archive.ID,
		},
		RewriteLinks: false,
	})
	if err != nil {
		t.Fatalf("ApplyPageRefactor(move) failed: %v", err)
	}

	if updated.CalculatePath() != "/archive/page-a" {
		t.Fatalf("updated path = %q, want %q", updated.CalculatePath(), "/archive/page-a")
	}

	movedPage, err := w.GetPage(pageA.ID)
	if err != nil {
		t.Fatalf("GetPage(pageA) failed: %v", err)
	}
	if movedPage.Content != "[To B](../../docs/page-b)" {
		t.Fatalf("moved page content = %q, want %q", movedPage.Content, "[To B](../../docs/page-b)")
	}

	outgoing, err := w.GetOutgoingLinks(pageA.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(pageA) failed: %v", err)
	}
	if outgoing.Count != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", outgoing.Count)
	}
	if outgoing.Outgoings[0].ToPageID != pageB.ID {
		t.Fatalf("ToPageID = %q, want %q", outgoing.Outgoings[0].ToPageID, pageB.ID)
	}
	if outgoing.Outgoings[0].ToPath != "/docs/page-b" {
		t.Fatalf("ToPath = %q, want %q", outgoing.Outgoings[0].ToPath, "/docs/page-b")
	}
	if outgoing.Outgoings[0].Broken {
		t.Fatalf("expected outgoing link to remain valid after move refactor")
	}

	afterMovedRevision, err := w.GetLatestRevision(pageA.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(pageA after refactor) failed: %v", err)
	}
	if afterMovedRevision == nil {
		t.Fatalf("expected latest revision for moved page")
	}
	if beforeMovedRevision == nil {
		t.Fatalf("expected initial revision for moved page")
	}
	if afterMovedRevision.ID == beforeMovedRevision.ID {
		t.Fatalf("expected moved page rewrite to create a new revision")
	}
	if afterMovedRevision.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("expected moved page latest revision type %q, got %q", revision.RevisionTypeContentUpdate, afterMovedRevision.Type)
	}
}

func TestWiki_DeletePage_Simple(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	page, _ := w.CreatePage("system", nil, "Trash", "trash", pageNodeKind())
	err := w.DeletePage("system", page.ID, false)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
	if _, err := w.GetPage(page.ID); err == nil {
		t.Fatalf("expected deleted page to be gone")
	}
}

func TestWiki_DeletePage_WithChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent, _ := w.CreatePage("system", nil, "Parent", "parent", pageNodeKind())
	_, _ = w.CreatePage("system", &parent.ID, "Child", "child", pageNodeKind())

	err := w.DeletePage("system", parent.ID, false)
	if err == nil {
		t.Error("Expected error when deleting parent with children")
	}
}

func TestWiki_DeletePage_Recursive(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent, _ := w.CreatePage("system", nil, "Parent", "parent", pageNodeKind())
	child, _ := w.CreatePage("system", &parent.ID, "Child", "child", pageNodeKind())

	err := w.DeletePage("system", parent.ID, true)
	if err != nil {
		t.Fatalf("DeletePage recursive failed: %v", err)
	}
	if _, err := w.GetPage(parent.ID); err == nil {
		t.Fatalf("expected deleted parent to be gone")
	}
	if _, err := w.GetPage(child.ID); err == nil {
		t.Fatalf("expected deleted child to be gone")
	}
}

func TestWiki_DeletePage_PurgesRevisionData(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, err := w.CreatePage("system", nil, "Page", "page", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "updated"
	if _, err := w.UpdatePage("system", page.ID, page.Title, page.Slug, &content, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	if _, _, err := w.revision.RecordDelete(page.ID, "system", "preexisting"); err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}

	if err := w.DeletePage("system", page.ID, false); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	revisions, err := w.ListRevisions(page.ID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("expected revisions to be purged, got %#v", revisions)
	}
	if _, err := w.GetTrashEntry(page.ID); err == nil {
		t.Fatalf("expected trash entry to be purged")
	}
}

func TestWiki_DeletePage_RootWithIDRoot(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	err := w.DeletePage("system", "root", false)
	if err == nil {
		t.Error("Expected error when attempting to delete root page with ID 'root', got none")
	}

	expectedMsg := "cannot delete root page"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestWiki_DeletePage_RootWithEmptyString(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	err := w.DeletePage("system", "", false)
	if err == nil {
		t.Error("Expected error when attempting to delete root page with empty string ID, got none")
	}

	expectedMsg := "cannot delete root page"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestWiki_UpdatePage(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, _ := w.CreatePage("system", nil, "Draft", "draft", pageNodeKind())
	var updatedstr = "# Updated"
	page, err := w.UpdatePage("system", page.ID, "Final", "final", &updatedstr, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	updated, _ := w.GetPage(page.ID)
	if updated.Title != "Final" {
		t.Errorf("Expected title 'Final', got %q", updated.Title)
	}
}

func TestWiki_SuggestSlug_Unique(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	slug, err := w.SuggestSlug("root", "", "My Page")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}
	if slug != "my-page" {
		t.Errorf("Expected 'my-page', got %q", slug)
	}
}

func TestWiki_SuggestSlug_Conflict(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	root := w.GetTree()
	_, err := w.CreatePage("system", nil, "My Page", "my-page", pageNodeKind())

	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	slug, err := w.SuggestSlug(root.ID, "", "My Page")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}
	if slug != "my-page-1" {
		t.Errorf("Expected 'my-page-1', got %q", slug)
	}
}

func TestWiki_SuggestSlug_DeepHierarchy(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// create a deep hierarchy of pages (Architecture -> Backend)
	_, err := w.CreatePage("system", nil, "Architecture", "architecture", pageNodeKind())
	if err != nil {
		t.Fatalf("Failed to create 'Architecture': %v", err)
	}
	root := w.GetTree()
	arch := root.Children[0]

	_, err = w.CreatePage("system", &arch.ID, "Backend", "backend", pageNodeKind())
	if err != nil {
		t.Fatalf("Failed to create 'Backend': %v", err)
	}
	backend := arch.Children[0]

	// Now suggest a slug there
	slug, err := w.SuggestSlug(backend.ID, "", "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}

	if slug != "data-layer" {
		t.Errorf("Expected 'data-layer', got %q", slug)
	}

	// Create a second one with the same name → it must be numbered
	_, err = w.CreatePage("system", &backend.ID, "Data Layer", "data-layer", pageNodeKind())
	if err != nil {
		t.Fatalf("Failed to create 'Data Layer': %v", err)
	}

	slug2, err := w.SuggestSlug(backend.ID, "", "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug 2 failed: %v", err)
	}

	if slug2 != "data-layer-1" {
		t.Errorf("Expected 'data-layer-1', got %q", slug2)
	}
}

func TestWiki_FindByPath_Valid(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, _ = w.CreatePage("system", nil, "Company", "company", pageNodeKind())

	found, err := w.FindByPath("company")
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}
	if found.Slug != "company" {
		t.Errorf("Expected slug 'company', got %q", found.Slug)
	}
}

func TestWiki_FindByPath_Invalid(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, err := w.FindByPath("does/not/exist")
	if err == nil {
		t.Error("Expected error for invalid path, got none")
	}
}

func TestWiki_SortPages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent, _ := w.CreatePage("system", nil, "Parent", "parent", pageNodeKind())
	child1, _ := w.CreatePage("system", &parent.ID, "Child1", "child1", pageNodeKind())
	child2, _ := w.CreatePage("system", &parent.ID, "Child2", "child2", pageNodeKind())

	err := w.SortPages(parent.ID, []string{child2.ID, child1.ID})
	if err != nil {
		t.Fatalf("SortPages failed: %v", err)
	}

	// Check if the order is correct
	sortedChildren := parent.Children
	if sortedChildren[0].ID != child2.ID || sortedChildren[1].ID != child1.ID {
		t.Errorf("Expected order [child2, child1], got [%s, %s]", sortedChildren[0].Slug, sortedChildren[1].Slug)
	}
}

func TestWiki_CopyPages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	original, _ := w.CreatePage("system", nil, "Original", "original", pageNodeKind())

	copied, err := w.CopyPage("system", original.ID, nil, "Copy of Original", "copy-of-original")
	if err != nil {
		t.Fatalf("CopyPage failed: %v", err)
	}

	if copied.Title != "Copy of Original" {
		t.Errorf("Expected title 'Copy of Original', got %q", copied.Title)
	}
	if copied.Slug != "copy-of-original" {
		t.Errorf("Expected slug 'copy-of-original', got %q", copied.Slug)
	}
	if copied.ID == original.ID {
		t.Error("Expected different ID for copied page")
	}
}

func TestWiki_CopyPages_WithParent(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent, _ := w.CreatePage("system", nil, "Parent", "parent", pageNodeKind())
	original, _ := w.CreatePage("system", nil, "Original", "original", pageNodeKind())

	copied, err := w.CopyPage("system", original.ID, &parent.ID, "Copy of Original", "copy-of-original")
	if err != nil {
		t.Fatalf("CopyPage with parent failed: %v", err)
	}

	if copied.Parent.ID != parent.ID {
		t.Errorf("Expected parent ID %q, got %q", parent.ID, copied.Parent.ID)
	}
}

func TestWiki_CopyPages_NonExistentSource(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	_, err := w.CopyPage("system", "non-existent-id", nil, "Copy", "copy")
	if err == nil {
		t.Error("Expected error for non-existent source page, got none")
	}
}

func TestWiki_CopyPages_WithAssets(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	original, _ := w.CreatePage("system", nil, "Original", "original", pageNodeKind())

	originalNode := tree.PageNode{
		ID:    original.ID,
		Title: original.Title,
		Slug:  original.Slug,
	}

	file, _, err := test_utils.CreateMultipartFile("image.png", []byte("image content"))
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(file.Close, t)

	// Save asset for the original page
	if _, err := w.GetAssetService().SaveAssetForPage(&originalNode, file, "image.png", 1024); err != nil {
		t.Fatalf("Failed to save asset for original page: %v", err)
	}

	copied, err := w.CopyPage("system", original.ID, nil, "Copy of Original", "copy-of-original")
	if err != nil {
		t.Fatalf("CopyPage failed: %v", err)
	}

	copiedNode := tree.PageNode{
		ID:    copied.ID,
		Title: copied.Title,
		Slug:  copied.Slug,
	}

	// Check if the asset was copied
	copiedAssetPath, err := w.GetAssetService().ListAssetsForPage(&copiedNode)
	if err != nil {
		t.Fatalf("Failed to list assets for copied page: %v", err)
	}
	if len(copiedAssetPath) != 1 {
		t.Errorf("Expected 1 asset for copied page, got %d", len(copiedAssetPath))
	}
}

func TestWiki_CopyPages_RecordsContentRevision(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	original, err := w.CreatePage("system", nil, "Original", "original", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	content := "original content"
	if _, err := w.UpdatePage("system", original.ID, original.Title, original.Slug, &content, pageNodeKind()); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	copied, err := w.CopyPage("editor", original.ID, nil, "Copy of Original", "copy-of-original")
	if err != nil {
		t.Fatalf("CopyPage failed: %v", err)
	}

	latest, err := w.GetLatestRevision(copied.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest revision for copied page")
	}
	if latest.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("latest revision type = %q, want %q", latest.Type, revision.RevisionTypeContentUpdate)
	}
	if latest.AuthorID != "editor" {
		t.Fatalf("latest author = %q, want %q", latest.AuthorID, "editor")
	}
	if latest.Summary != "page copied" {
		t.Fatalf("latest summary = %q, want %q", latest.Summary, "page copied")
	}
}

func TestWiki_InitDefaultAdmin_UsesGivenPassword(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	_, err := w.GetUserService().GetUserByEmailOrUsernameAndPassword("admin", "admin")
	if err != nil {
		t.Fatalf("Admin user not found: %v", err)
	}
}

func TestWiki_Login_SuccessAndFailure(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	token, err := w.Login("admin", "admin")
	if err != nil || token == nil {
		t.Error("Expected login to succeed with default admin password")
	}

	_, err = w.Login("admin", "wrong")
	if err == nil {
		t.Error("Expected login to fail with wrong password")
	}
}

func TestWiki_EnsurePath_HealsLinksForAllCreatedSegments(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// 1) Create page A with links to /x and /x/y (both non-existing)
	pageA, err := w.CreatePage("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}

	var contentA = "Links: [X](/x) and [XY](/x/y)"
	_, err = w.UpdatePage("system", pageA.ID, pageA.Title, pageA.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// 2) Reindex once so that broken links are stored in the DB
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	out1, err := w.GetOutgoingLinks(pageA.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before ensure, got %d: %#v", out1.Count, out1.Outgoings)
	}

	byPath := map[string]bool{}
	for _, it := range out1.Outgoings {
		byPath[it.ToPath] = it.Broken
	}
	if broken, ok := byPath["/x"]; !ok || broken != true {
		t.Fatalf("expected /x to be broken before ensure, got map=%#v, out=%#v", byPath, out1.Outgoings)
	}
	if broken, ok := byPath["/x/y"]; !ok || broken != true {
		t.Fatalf("expected /x/y to be broken before ensure, got map=%#v, out=%#v", byPath, out1.Outgoings)
	}

	// 3) EnsurePath creates /x and /x/y and triggers Heal for all newly created segments
	_, err = w.EnsurePath("system", "/x/y", "X Y", pageNodeKind())
	if err != nil {
		t.Fatalf("EnsurePath failed: %v", err)
	}

	// 4) Without reindexing: outgoing links from A should now be resolved
	out2, err := w.GetOutgoingLinks(pageA.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks (after ensure) failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after ensure, got %d: %#v", out2.Count, out2.Outgoings)
	}

	var gotX, gotXY *struct {
		broken bool
		toPage string
	}
	for _, it := range out2.Outgoings {
		if it.ToPath == "/x" {
			gotX = &struct {
				broken bool
				toPage string
			}{it.Broken, it.ToPageID}
		}
		if it.ToPath == "/x/y" {
			gotXY = &struct {
				broken bool
				toPage string
			}{it.Broken, it.ToPageID}
		}
	}

	if gotX == nil {
		t.Fatalf("missing outgoing to /x: %#v", out2.Outgoings)
	}
	if gotX.broken {
		t.Fatalf("expected /x to be healed, got broken=true: %#v", out2.Outgoings)
	}
	if gotX.toPage == "" {
		t.Fatalf("expected /x ToPageID to be set after heal, got empty: %#v", out2.Outgoings)
	}

	if gotXY == nil {
		t.Fatalf("missing outgoing to /x/y: %#v", out2.Outgoings)
	}
	if gotXY.broken {
		t.Fatalf("expected /x/y to be healed, got broken=true: %#v", out2.Outgoings)
	}
	if gotXY.toPage == "" {
		t.Fatalf("expected /x/y ToPageID to be set after heal, got empty: %#v", out2.Outgoings)
	}
}

func TestWiki_DeletePage_NonRecursive_MarksIncomingBroken(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create A with link to /b
	a, err := w.CreatePage("system", nil, "Page A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}

	var contentA = "Link to B: [Go](/b)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B
	b, err := w.CreatePage("system", nil, "Page B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}

	var contentB = "# Page B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Ensure link index
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Delete B
	if err := w.DeletePage("system", b.ID, false); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	// Outgoing links for A should still exist but be broken
	out, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", out.Count)
	}

	got := out.Outgoings[0]
	if got.ToPath != "/b" {
		t.Fatalf("ToPath = %q, want %q", got.ToPath, "/b")
	}
	if got.Broken != true {
		t.Fatalf("Broken = %v, want true", got.Broken)
	}
	if got.ToPageID != "" {
		t.Fatalf("ToPageID = %q, want empty", got.ToPageID)
	}

	// Backlinks for B must be 0 because query filters on broken=0/to_page_id match
	bl, err := w.GetBacklinks(b.ID)
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if bl.Count != 0 {
		t.Fatalf("expected 0 backlinks after delete, got %d", bl.Count)
	}
}

func TestWiki_DeletePage_Recursive_RemovesOutgoingForSubtree_AndBreaksIncomingByPrefix(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create /docs
	docs, err := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}

	// Create /docs/a and /docs/b
	a, err := w.CreatePage("system", &docs.ID, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage a failed: %v", err)
	}
	b, err := w.CreatePage("system", &docs.ID, "B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage b failed: %v", err)
	}

	// A links to B inside subtree
	var contentA = "Link to B: [B](/docs/b)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}
	var contentB = "# B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage b failed: %v", err)
	}

	// Create survivor /c with incoming link into subtree
	c, err := w.CreatePage("system", nil, "C", "c", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage c failed: %v", err)
	}
	var contentC = "Incoming link: [B](/docs/b)"
	_, err = w.UpdatePage("system", c.ID, c.Title, c.Slug, &contentC, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage c failed: %v", err)
	}

	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Sanity check: A has an outgoing link before delete
	outA, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(a) before delete failed: %v", err)
	}
	if outA.Count != 1 {
		t.Fatalf("expected 1 outgoing from a before delete, got %d", outA.Count)
	}

	// Delete /docs recursively
	if err := w.DeletePage("system", docs.ID, true); err != nil {
		t.Fatalf("DeletePage(docs, recursive) failed: %v", err)
	}

	// 1) Outgoing links FROM deleted child page must be gone
	outAAfter, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(a) after delete failed: %v", err)
	}
	if outAAfter.Count != 0 {
		t.Fatalf("expected 0 outgoing from deleted page a, got %d", outAAfter.Count)
	}

	// 2) Incoming link from survivor page /c into subtree must still exist, but be broken
	outC, err := w.GetOutgoingLinks(c.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(c) after delete failed: %v", err)
	}
	if outC.Count != 1 {
		t.Fatalf("expected 1 outgoing from c, got %d", outC.Count)
	}

	got := outC.Outgoings[0]
	if got.ToPath != "/docs/b" {
		t.Fatalf("ToPath = %q, want %q", got.ToPath, "/docs/b")
	}
	if got.Broken != true {
		t.Fatalf("Broken = %v, want true", got.Broken)
	}
	if got.ToPageID != "" {
		t.Fatalf("ToPageID = %q, want empty", got.ToPageID)
	}
}

func TestWiki_RenamePage_MarksOldBroken_HealsNewExactPath(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create A with links to /b (exists) and /b2 (does not exist yet)
	a, err := w.CreatePage("system", nil, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	var contentA = "Links: [B](/b) and [B2](/b2)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B at /b
	b, err := w.CreatePage("system", nil, "B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	var contentB = "# B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Index once so outgoing links exist + broken state is materialized
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	out1, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before rename, got %d: %#v", out1.Count, out1.Outgoings)
	}

	byPath1 := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out1.Outgoings {
		byPath1[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	if got, ok := byPath1["/b"]; !ok || got.broken {
		t.Fatalf("expected /b to be valid before rename, got %#v", byPath1)
	}
	if got, ok := byPath1["/b2"]; !ok || !got.broken {
		t.Fatalf("expected /b2 to be broken before rename, got %#v", byPath1)
	}

	// Rename B: /b -> /b2
	var contentB2 = "# B (renamed)"
	_, err = w.UpdatePage("system", b.ID, b.Title, "b2", &contentB2, pageNodeKind())
	if err != nil {
		t.Fatalf("Rename B failed: %v", err)
	}

	// Without reindex: outgoing from A should reflect:
	// - /b becomes broken
	// - /b2 becomes healed and points to B's ID
	out2, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) after rename failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after rename, got %d: %#v", out2.Count, out2.Outgoings)
	}

	byPath2 := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		byPath2[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	// old path broken
	if got, ok := byPath2["/b"]; !ok || !got.broken || got.toID != "" {
		t.Fatalf("expected /b to be broken with empty to_page_id after rename, got %#v", byPath2)
	}

	// new path healed
	gotNew, ok := byPath2["/b2"]
	if !ok || gotNew.broken || gotNew.toID == "" {
		t.Fatalf("expected /b2 to be healed with to_page_id set, got %#v", byPath2)
	}
	if gotNew.toID != b.ID {
		t.Fatalf("expected /b2 to resolve to page %q, got %q", b.ID, gotNew.toID)
	}
}

func TestWiki_RenameSubtree_BreaksOldPrefix_HealsNewSubpaths(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create subtree: /docs/b
	docs, err := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	b, err := w.CreatePage("system", &docs.ID, "B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage /docs/b failed: %v", err)
	}
	var contentB = "# B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create A that links to old and future new subtree paths
	a, err := w.CreatePage("system", nil, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	var contentA = "Links: [Old](/docs/b) and [New](/docs2/b)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Materialize graph state
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	out1, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before rename, got %d: %#v", out1.Count, out1.Outgoings)
	}

	// Rename /docs -> /docs2
	var contentDocs2 = "# Docs"
	nodeSection := tree.NodeKindSection
	_, err = w.UpdatePage("system", docs.ID, docs.Title, "docs2", &contentDocs2, &nodeSection)
	if err != nil {
		t.Fatalf("Rename docs failed: %v", err)
	}

	// Without reindex: A should now have
	// - /docs/b broken
	// - /docs2/b healed and resolves to the same page id as the child B
	out2, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) after subtree rename failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after rename, got %d: %#v", out2.Count, out2.Outgoings)
	}

	byPath := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		byPath[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	// old prefix path broken
	if got, ok := byPath["/docs/b"]; !ok || !got.broken || got.toID != "" {
		t.Fatalf("expected /docs/b broken with empty to_page_id, got %#v", byPath)
	}

	// new subpath healed
	gotNew, ok := byPath["/docs2/b"]
	if !ok || gotNew.broken || gotNew.toID == "" {
		t.Fatalf("expected /docs2/b healed with to_page_id set, got %#v", byPath)
	}
	if gotNew.toID != b.ID {
		t.Fatalf("expected /docs2/b to resolve to page %q, got %q", b.ID, gotNew.toID)
	}
}

func TestWiki_MovePage_MarksOldBroken_HealsNewExactPath(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create A that links to /b (old path) and /projects/b (future path)
	a, err := w.CreatePage("system", nil, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	var contentA = "Links: [B](/b) and [B2](/projects/b)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B at /b
	b, err := w.CreatePage("system", nil, "B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	var contentB = "# B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create parent /projects (target)
	projects, err := w.CreatePage("system", nil, "Projects", "projects", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage projects failed: %v", err)
	}

	// Materialize links once (so broken links exist in DB)
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Sanity: /b should be valid, /projects/b should be broken before move
	out1, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before move, got %d: %#v", out1.Count, out1.Outgoings)
	}

	state1 := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out1.Outgoings {
		state1[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	if got := state1["/b"]; got.broken || got.toID == "" {
		t.Fatalf("expected /b valid before move, got %#v", state1)
	}
	if got := state1["/projects/b"]; !got.broken || got.toID != "" {
		t.Fatalf("expected /projects/b broken before move, got %#v", state1)
	}

	// Move B under /projects => /projects/b now exists
	if err := w.MovePage("system", b.ID, projects.ID); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	// Without reindex: /b must become broken, /projects/b must be healed to B
	out2, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) after move failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after move, got %d: %#v", out2.Count, out2.Outgoings)
	}

	state2 := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		state2[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	// old path broken
	if got := state2["/b"]; !got.broken || got.toID != "" {
		t.Fatalf("expected /b broken after move (to_page_id empty), got %#v", state2)
	}

	// new path healed
	if got := state2["/projects/b"]; got.broken || got.toID != b.ID {
		t.Fatalf("expected /projects/b healed to page %q, got %#v", b.ID, state2)
	}
}

func TestWiki_MoveSubtree_BreaksOldPrefix_HealsNewSubpaths(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create subtree /docs/b
	docs, err := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	b, err := w.CreatePage("system", &docs.ID, "B", "b", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage /docs/b failed: %v", err)
	}
	var contentB = "# B"
	_, err = w.UpdatePage("system", b.ID, b.Title, b.Slug, &contentB, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create target parent /archive
	archive, err := w.CreatePage("system", nil, "Archive", "archive", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage archive failed: %v", err)
	}

	// Create A that links to old and future new subtree paths
	a, err := w.CreatePage("system", nil, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	var contentA = "Links: [Old](/docs/b) and [New](/archive/docs/b)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Materialize graph
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Move /docs under /archive => /archive/docs/b exists, /docs/b disappears
	if err := w.MovePage("system", docs.ID, archive.ID); err != nil {
		t.Fatalf("MovePage(docs -> archive) failed: %v", err)
	}

	// Without reindex: A should now have /docs/b broken and /archive/docs/b healed to the same B id
	out, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(A) after subtree move failed: %v", err)
	}
	if out.Count != 2 {
		t.Fatalf("expected 2 outgoings after move, got %d: %#v", out.Count, out.Outgoings)
	}

	state := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out.Outgoings {
		state[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}

	if got := state["/docs/b"]; !got.broken || got.toID != "" {
		t.Fatalf("expected /docs/b broken after move, got %#v", state)
	}

	if got := state["/archive/docs/b"]; got.broken || got.toID != b.ID {
		t.Fatalf("expected /archive/docs/b healed to page %q, got %#v", b.ID, state)
	}
}

func TestWiki_MovePage_ReindexesRelativeLinks(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Create /docs with /docs/shared and /docs/a
	docs, err := w.CreatePage("system", nil, "Docs", "docs", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}

	docsShared, err := w.CreatePage("system", &docs.ID, "Shared", "shared", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage /docs/shared failed: %v", err)
	}
	var contentDocsShared = "# Docs Shared"
	_, err = w.UpdatePage("system", docsShared.ID, docsShared.Title, docsShared.Slug, &contentDocsShared, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage /docs/shared failed: %v", err)
	}

	a, err := w.CreatePage("system", &docs.ID, "A", "a", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage /docs/a failed: %v", err)
	}
	// Important: relative link
	var contentA = "Relative: [S](../shared)"
	_, err = w.UpdatePage("system", a.ID, a.Title, a.Slug, &contentA, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage /docs/a failed: %v", err)
	}

	// Create /guide with /guide/shared (different page!)
	guide, err := w.CreatePage("system", nil, "Guide", "guide", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage guide failed: %v", err)
	}

	guideShared, err := w.CreatePage("system", &guide.ID, "Shared", "shared", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage /guide/shared failed: %v", err)
	}
	var contentGuideShared = "# Guide Shared"
	_, err = w.UpdatePage("system", guideShared.ID, guideShared.Title, guideShared.Slug, &contentGuideShared, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage /guide/shared failed: %v", err)
	}

	// Materialize graph
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Before move: /docs/a's outgoing must resolve to /docs/shared
	out1, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(/docs/a) before move failed: %v", err)
	}
	if out1.Count != 1 {
		t.Fatalf("expected 1 outgoing before move, got %d: %#v", out1.Count, out1.Outgoings)
	}
	if out1.Outgoings[0].ToPath != "/docs/shared" {
		t.Fatalf("ToPath before move = %q, want %q", out1.Outgoings[0].ToPath, "/docs/shared")
	}
	if out1.Outgoings[0].Broken {
		t.Fatalf("expected link to be valid before move, got broken=true")
	}
	if out1.Outgoings[0].ToPageID != docsShared.ID {
		t.Fatalf("ToPageID before move = %q, want %q", out1.Outgoings[0].ToPageID, docsShared.ID)
	}

	// Move /docs/a under /guide => page path becomes /guide/a
	if err := w.MovePage("system", a.ID, guide.ID); err != nil {
		t.Fatalf("MovePage(a -> guide) failed: %v", err)
	}

	// After move (without reindex): relative link must now resolve to /guide/shared
	out2, err := w.GetOutgoingLinks(a.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(/guide/a) after move failed: %v", err)
	}
	if out2.Count != 1 {
		t.Fatalf("expected 1 outgoing after move, got %d: %#v", out2.Count, out2.Outgoings)
	}
	if out2.Outgoings[0].ToPath != "/guide/shared" {
		t.Fatalf("ToPath after move = %q, want %q", out2.Outgoings[0].ToPath, "/guide/shared")
	}
	if out2.Outgoings[0].Broken {
		t.Fatalf("expected link to be valid after move, got broken=true")
	}
	if out2.Outgoings[0].ToPageID != guideShared.ID {
		t.Fatalf("ToPageID after move = %q, want %q", out2.Outgoings[0].ToPageID, guideShared.ID)
	}
}

func TestWiki_AuthDisabled_Initialization(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "",
		JWTSecret:           "",
		AccessTokenTimeout:  0,
		RefreshTokenTimeout: 0,
		AuthDisabled:        true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Verify that the auth service is nil
	if wikiInstance.GetAuthService() != nil {
		t.Error("Expected auth service to be nil when AuthDisabled is true")
	}
}

func TestWiki_AuthDisabled_LoginReturnsError(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Attempt to login should return ErrAuthDisabled
	_, err = wikiInstance.Login("admin", "admin")
	if err != ErrAuthDisabled {
		t.Errorf("Expected ErrAuthDisabled, got %v", err)
	}
}

func TestWiki_AuthDisabled_LogoutReturnsError(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Attempt to logout should return ErrAuthDisabled
	err = wikiInstance.Logout("some-token")
	if err != ErrAuthDisabled {
		t.Errorf("Expected ErrAuthDisabled, got %v", err)
	}
}

func TestWiki_AuthDisabled_RefreshTokenReturnsError(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Attempt to refresh token should return ErrAuthDisabled
	_, err = wikiInstance.RefreshToken("some-token")
	if err != ErrAuthDisabled {
		t.Errorf("Expected ErrAuthDisabled, got %v", err)
	}
}

func TestWiki_AuthDisabled_CoreFunctionalityWorks(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Test creating a page
	page, err := wikiInstance.CreatePage("system", nil, "Test Page", "test-page", pageNodeKind())
	if err != nil {
		t.Fatalf("Failed to create page with AuthDisabled: %v", err)
	}

	if page.Title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %q", page.Title)
	}

	// Test updating a page
	var updatedContent = "# Content"
	updatedPage, err := wikiInstance.UpdatePage("system", page.ID, "Updated Title", "updated-slug", &updatedContent, pageNodeKind())
	if err != nil {
		t.Fatalf("Failed to update page with AuthDisabled: %v", err)
	}

	if updatedPage.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %q", updatedPage.Title)
	}

	// Test getting a page
	retrievedPage, err := wikiInstance.GetPage(page.ID)
	if err != nil {
		t.Fatalf("Failed to get page with AuthDisabled: %v", err)
	}

	if retrievedPage.ID != page.ID {
		t.Errorf("Expected ID %q, got %q", page.ID, retrievedPage.ID)
	}

	// Test deleting a page
	err = wikiInstance.DeletePage("system", page.ID, false)
	if err != nil {
		t.Fatalf("Failed to delete page with AuthDisabled: %v", err)
	}
}

func TestWiki_RestorePageWithNewParentPreservesIDAndAssets(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	sectionKind := tree.NodeKindSection
	parent, err := w.CreatePage("system", nil, "Docs", "docs", &sectionKind)
	if err != nil {
		t.Fatalf("CreatePage(parent) failed: %v", err)
	}

	child, err := w.CreatePage("system", &parent.ID, "Child", "child", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage(child) failed: %v", err)
	}

	content := "restored content"
	child, err = w.UpdatePage("system", child.ID, child.Title, child.Slug, &content, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	assetDir := filepath.Join(w.GetAssetService().GetAssetsDir(), child.ID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(assetDir) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "note.txt"), []byte("asset payload"), 0o644); err != nil {
		t.Fatalf("WriteFile(asset) failed: %v", err)
	}
	w.recordAssetRevision(child.ID, "system", "")

	if _, _, err := w.revision.RecordDelete(child.ID, "system", "delete"); err != nil {
		t.Fatalf("RecordDelete(child) failed: %v", err)
	}
	if _, _, err := w.revision.RecordDelete(parent.ID, "system", "delete"); err != nil {
		t.Fatalf("RecordDelete(parent) failed: %v", err)
	}
	if err := w.tree.DeleteNode("system", parent.ID, true); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := w.asset.DeleteAllAssetsForPage(&tree.PageNode{ID: child.ID}); err != nil {
		t.Fatalf("DeleteAllAssetsForPage(child) failed: %v", err)
	}
	if err := w.asset.DeleteAllAssetsForPage(&tree.PageNode{ID: parent.ID}); err != nil {
		t.Fatalf("DeleteAllAssetsForPage(parent) failed: %v", err)
	}

	rootParent := "root"
	restored, err := w.RestorePage("system", child.ID, &rootParent)
	if err != nil {
		t.Fatalf("RestorePage failed: %v", err)
	}

	if restored.ID != child.ID {
		t.Fatalf("restored ID = %q, want %q", restored.ID, child.ID)
	}
	if restored.CalculatePath() != "/child" {
		t.Fatalf("restored path = %q, want %q", restored.CalculatePath(), "/child")
	}
	if restored.Content != content {
		t.Fatalf("restored content = %q, want %q", restored.Content, content)
	}

	assetBytes, err := os.ReadFile(filepath.Join(w.GetAssetService().GetAssetsDir(), child.ID, "note.txt"))
	if err != nil {
		t.Fatalf("ReadFile(restored asset) failed: %v", err)
	}
	if string(assetBytes) != "asset payload" {
		t.Fatalf("restored asset = %q", string(assetBytes))
	}

	if _, err := w.GetTrashEntry(child.ID); err == nil {
		t.Fatalf("expected trash entry to be deleted after restore")
	}

	latest, err := w.GetLatestRevision(child.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != revision.RevisionTypeRestore {
		t.Fatalf("latest revision type = %#v", latest)
	}
}

func TestWiki_RestorePageRequiresTargetWhenOriginalParentMissing(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	sectionKind := tree.NodeKindSection
	parent, _ := w.CreatePage("system", nil, "Docs", "docs", &sectionKind)
	child, _ := w.CreatePage("system", &parent.ID, "Child", "child", pageNodeKind())

	if _, _, err := w.revision.RecordDelete(child.ID, "system", "delete"); err != nil {
		t.Fatalf("RecordDelete(child) failed: %v", err)
	}
	if _, _, err := w.revision.RecordDelete(parent.ID, "system", "delete"); err != nil {
		t.Fatalf("RecordDelete(parent) failed: %v", err)
	}
	if err := w.tree.DeleteNode("system", parent.ID, true); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	_, err := w.RestorePage("system", child.ID, nil)
	if err == nil {
		t.Fatalf("expected restore to fail without target parent")
	}

	localized, ok := verrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized revision error, got %T", err)
	}
	if localized.Code != "revision_restore_parent_required" {
		t.Fatalf("error code = %q", localized.Code)
	}
}

func TestWiki_RestoreRevisionRestoresAssetsAndStructure(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	sectionKind := tree.NodeKindSection
	docs, err := w.CreatePage("system", nil, "Docs", "docs", &sectionKind)
	if err != nil {
		t.Fatalf("CreatePage(docs) failed: %v", err)
	}
	archive, err := w.CreatePage("system", nil, "Archive", "archive", &sectionKind)
	if err != nil {
		t.Fatalf("CreatePage(archive) failed: %v", err)
	}
	page, err := w.CreatePage("system", &docs.ID, "Original", "original", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage(page) failed: %v", err)
	}

	originalContent := "first version"
	page, err = w.UpdatePage("system", page.ID, "Original", "original", &originalContent, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage(original) failed: %v", err)
	}

	assetDir := filepath.Join(w.GetAssetService().GetAssetsDir(), page.ID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(assetDir) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "old.txt"), []byte("old-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile(old asset) failed: %v", err)
	}
	w.recordAssetRevision(page.ID, "system", "")

	originalRevision, err := w.GetLatestRevision(page.ID)
	if err != nil || originalRevision == nil {
		t.Fatalf("GetLatestRevision(original) failed: %#v %v", originalRevision, err)
	}

	changedContent := "second version"
	page, err = w.UpdatePage("system", page.ID, "Changed", "changed", &changedContent, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage(changed) failed: %v", err)
	}
	if err := w.MovePage("system", page.ID, archive.ID); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}
	if err := os.Remove(filepath.Join(assetDir, "old.txt")); err != nil {
		t.Fatalf("Remove(old asset) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "new.txt"), []byte("new-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile(new asset) failed: %v", err)
	}
	w.recordAssetRevision(page.ID, "system", "")

	restored, err := w.RestoreRevision("system", page.ID, originalRevision.ID)
	if err != nil {
		t.Fatalf("RestoreRevision failed: %v", err)
	}

	if restored.Title != "Original" || restored.Slug != "original" {
		t.Fatalf("restored identity = (%q,%q)", restored.Title, restored.Slug)
	}
	if restored.CalculatePath() != "/archive/original" {
		t.Fatalf("restored path = %q", restored.CalculatePath())
	}
	if restored.Content != originalContent {
		t.Fatalf("restored content = %q", restored.Content)
	}

	oldAsset, err := os.ReadFile(filepath.Join(assetDir, "old.txt"))
	if err != nil {
		t.Fatalf("ReadFile(old asset) failed: %v", err)
	}
	if string(oldAsset) != "old-asset" {
		t.Fatalf("old asset = %q", string(oldAsset))
	}
	if _, err := os.Stat(filepath.Join(assetDir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected new asset to be removed, got %v", err)
	}
}

func TestWiki_MovePageRecordsStructureRevision(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	dest, err := w.CreatePage("system", nil, "Dest", "dest", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage(dest) failed: %v", err)
	}
	page, err := w.CreatePage("system", nil, "Move Me", "move-me", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage(page) failed: %v", err)
	}

	if err := w.MovePage("system", page.ID, dest.ID); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	latest, err := w.GetLatestRevision(page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != revision.RevisionTypeStructureUpdate {
		t.Fatalf("latest revision = %#v", latest)
	}
	if latest.ParentID != dest.ID {
		t.Fatalf("latest parent id = %q, want %q", latest.ParentID, dest.ID)
	}
}

func TestWiki_UpdatePage_TitleOnlyCreatesStructureRevision(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, err := w.CreatePage("system", nil, "Original", "original", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	content := "same content"
	page, err = w.UpdatePage("system", page.ID, page.Title, page.Slug, &content, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage(initial content) failed: %v", err)
	}

	beforeLatest, err := w.GetLatestRevision(page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(before rename) failed: %v", err)
	}
	if beforeLatest == nil {
		t.Fatal("expected initial content revision")
	}

	updatedPage, err := w.UpdatePage("system", page.ID, "Renamed Title", page.Slug, nil, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage(title only) failed: %v", err)
	}
	if updatedPage.Title != "Renamed Title" {
		t.Fatalf("updated title = %q", updatedPage.Title)
	}

	afterLatest, err := w.GetLatestRevision(page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(after rename) failed: %v", err)
	}
	if afterLatest == nil {
		t.Fatal("expected latest revision after title update")
	}
	if afterLatest.ID == beforeLatest.ID {
		t.Fatalf("expected new revision for title-only change")
	}
	if afterLatest.Type != revision.RevisionTypeStructureUpdate {
		t.Fatalf("latest revision type = %q", afterLatest.Type)
	}

	revisions, err := w.ListRevisions(page.ID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revision count = %d, want 2", len(revisions))
	}
}

func TestWiki_AssetMutationsRecordAssetRevisionForUser(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	writeAsset := func(t *testing.T, pageID, name string, content []byte) {
		t.Helper()

		assetDir := filepath.Join(w.GetAssetService().GetAssetsDir(), pageID)
		if err := os.MkdirAll(assetDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(assetDir) failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, name), content, 0o644); err != nil {
			t.Fatalf("WriteFile(asset) failed: %v", err)
		}
	}

	tests := []struct {
		name      string
		setup     func(t *testing.T, pageID string)
		operate   func(t *testing.T, pageID string)
		wantAsset string
	}{
		{
			name: "upload",
			operate: func(t *testing.T, pageID string) {
				t.Helper()

				file, err := os.CreateTemp(t.TempDir(), "asset-upload-*")
				if err != nil {
					t.Fatalf("CreateTemp failed: %v", err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Fatalf("Close(file) failed: %v", err)
					}
				})
				if _, err := file.WriteString("payload"); err != nil {
					t.Fatalf("WriteString(file) failed: %v", err)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatalf("Seek(file) failed: %v", err)
				}

				if _, err := w.UploadAsset("editor", pageID, file, "uploaded.txt", 1024); err != nil {
					t.Fatalf("UploadAsset failed: %v", err)
				}
			},
			wantAsset: "uploaded.txt",
		},
		{
			name: "rename",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				writeAsset(t, pageID, "old.txt", []byte("payload"))
			},
			operate: func(t *testing.T, pageID string) {
				t.Helper()
				if _, err := w.RenameAsset("editor", pageID, "old.txt", "new.txt"); err != nil {
					t.Fatalf("RenameAsset failed: %v", err)
				}
			},
			wantAsset: "new.txt",
		},
		{
			name: "delete",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				writeAsset(t, pageID, "delete.txt", []byte("payload"))
			},
			operate: func(t *testing.T, pageID string) {
				t.Helper()
				if err := w.DeleteAsset("editor", pageID, "delete.txt"); err != nil {
					t.Fatalf("DeleteAsset failed: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := w.CreatePage("system", nil, "Asset Page "+tc.name, "asset-page-"+tc.name, pageNodeKind())
			if err != nil {
				t.Fatalf("CreatePage failed: %v", err)
			}

			if tc.setup != nil {
				tc.setup(t, page.ID)
			}

			tc.operate(t, page.ID)

			latest, err := w.GetLatestRevision(page.ID)
			if err != nil {
				t.Fatalf("GetLatestRevision failed: %v", err)
			}
			if latest == nil || latest.Type != revision.RevisionTypeAssetUpdate {
				t.Fatalf("latest revision = %#v", latest)
			}
			if latest.AuthorID != "editor" {
				t.Fatalf("latest author = %q, want %q", latest.AuthorID, "editor")
			}

			assets, err := w.ListAssets(page.ID)
			if err != nil {
				t.Fatalf("ListAssets failed: %v", err)
			}

			if tc.wantAsset == "" {
				if len(assets) != 0 {
					t.Fatalf("assets = %#v, want empty", assets)
				}
				return
			}

			if len(assets) != 1 || !strings.HasSuffix(assets[0], "/"+tc.wantAsset) {
				t.Fatalf("assets = %#v, want suffix %q", assets, tc.wantAsset)
			}
		})
	}
}

func TestWiki_CheckRevisionIntegrityPassthrough(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page, err := w.CreatePage("system", nil, "Page", "page", pageNodeKind())
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "hello"
	page, err = w.UpdatePage("system", page.ID, page.Title, page.Slug, &content, pageNodeKind())
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}
	contentBlobPath := filepath.Join(w.GetStorageDir(), ".leafwiki", "blobs", "content", "sha256", rev.ContentHash[:2], rev.ContentHash)
	if err := os.Remove(contentBlobPath); err != nil {
		t.Fatalf("Remove content blob failed: %v", err)
	}

	issues, err := w.CheckRevisionIntegrity(page.ID)
	if err != nil {
		t.Fatalf("CheckRevisionIntegrity failed: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 integrity issue, got %#v", issues)
	}
	if issues[0].Code != "missing_content_blob" {
		t.Fatalf("unexpected integrity issue: %#v", issues[0])
	}
}
