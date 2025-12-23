package wiki

import (
	"testing"

	verrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

func setupTestWiki(t *testing.T) *Wiki {
	tempDir := t.TempDir()
	w, err := NewWiki(tempDir, "admin", "secretkey", false)
	if err != nil {
		t.Fatalf("Failed to create wiki: %v", err)
	}
	return w
}

func TestWiki_CreatePage_Root(t *testing.T) {
	w := setupTestWiki(t)

	page, err := w.CreatePage(nil, "Home", "home")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	if page.Title != "Home" {
		t.Errorf("Expected title 'Home', got %q", page.Title)
	}
}

func TestWiki_CreatePage_WithParent(t *testing.T) {
	w := setupTestWiki(t)
	rootPage, _ := w.CreatePage(nil, "Docs", "docs")

	page, err := w.CreatePage(&rootPage.ID, "API-Doc", "api-doc")
	if err != nil {
		t.Fatalf("CreatePage with parent failed: %v", err)
	}

	if page.Parent.ID != rootPage.ID {
		t.Errorf("Expected parent ID %q, got %q", rootPage.ID, page.Parent.ID)
	}
}

func TestWiki_CreatePage_EmptyTitle(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.CreatePage(nil, "", "empty")
	if err == nil {
		t.Error("Expected error for empty title, got none")
	}
}

func TestWiki_CreatePage_ReservedSlug(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.CreatePage(nil, "Reserved", "e")
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

func TestWiki_CreatePage_PageExists(t *testing.T) {
	w := setupTestWiki(t)
	_, _ = w.CreatePage(nil, "Duplicate", "duplicate")

	_, err := w.CreatePage(nil, "Duplicate", "duplicate")
	if err == nil {
		t.Error("Expected error for duplicate page, got none")
	}
}

func TestWiki_CreatePage_InvalidParent(t *testing.T) {
	w := setupTestWiki(t)
	invalidID := "not-real"
	_, err := w.CreatePage(&invalidID, "Broken", "broken")
	if err == nil {
		t.Error("Expected error with invalid parent ID, got none")
	}
}

func TestWiki_GetPage_ValidID(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "ReadMe", "readme")

	found, err := w.GetPage(page.ID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}

	if found.ID != page.ID {
		t.Errorf("Expected ID %q, got %q", page.ID, found.ID)
	}
}

func TestWiki_GetPage_InvalidID(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.GetPage("unknown")
	if err == nil {
		t.Error("Expected error for unknown ID, got none")
	}
}

func TestWiki_MovePage_Valid(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Projects", "projects")
	child, _ := w.CreatePage(nil, "Old", "old")

	err := w.MovePage(child.ID, parent.ID)
	if err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}
}

func TestWiki_DeletePage_Simple(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "Trash", "trash")

	err := w.DeletePage(page.ID, false)
	if err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
}

func TestWiki_DeletePage_WithChildren(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent", "parent")
	_, _ = w.CreatePage(&parent.ID, "Child", "child")

	err := w.DeletePage(parent.ID, false)
	if err == nil {
		t.Error("Expected error when deleting parent with children")
	}
}

func TestWiki_DeletePage_Recursive(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent", "parent")
	_, _ = w.CreatePage(&parent.ID, "Child", "child")

	err := w.DeletePage(parent.ID, true)
	if err != nil {
		t.Fatalf("DeletePage recursive failed: %v", err)
	}
}

func TestWiki_DeletePage_RootWithIDRoot(t *testing.T) {
	w := setupTestWiki(t)

	err := w.DeletePage("root", false)
	if err == nil {
		t.Error("Expected error when attempting to delete root page with ID 'root', got none")
	}

	expectedMsg := "cannot delete root page"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestWiki_DeletePage_RootWithEmptyString(t *testing.T) {
	w := setupTestWiki(t)

	err := w.DeletePage("", false)
	if err == nil {
		t.Error("Expected error when attempting to delete root page with empty string ID, got none")
	}

	expectedMsg := "cannot delete root page"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}
}

func TestWiki_UpdatePage(t *testing.T) {
	w := setupTestWiki(t)
	page, _ := w.CreatePage(nil, "Draft", "draft")

	page, err := w.UpdatePage(page.ID, "Final", "final", "# Updated")
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	updated, _ := w.GetPage(page.ID)
	if updated.Title != "Final" {
		t.Errorf("Expected title 'Final', got %q", updated.Title)
	}
}

func TestWiki_SuggestSlug_Unique(t *testing.T) {
	w := setupTestWiki(t)
	slug, err := w.SuggestSlug("root", "", "My Page")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}
	if slug != "my-page" {
		t.Errorf("Expected 'my-page', got %q", slug)
	}
}

func TestWiki_SuggestSlug_Conflict(t *testing.T) {
	w := setupTestWiki(t)
	root := w.GetTree()
	_, err := w.CreatePage(nil, "My Page", "my-page")

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
	tmpDir := t.TempDir()
	wiki, err := NewWiki(tmpDir, "admin", "secretkey", false)
	if err != nil {
		t.Fatalf("Failed to initialize Wiki: %v", err)
	}

	// Erstelle tiefere Struktur: root -> architecture -> backend
	_, err = wiki.CreatePage(nil, "Architecture", "architecture")
	if err != nil {
		t.Fatalf("Failed to create 'Architecture': %v", err)
	}
	root := wiki.GetTree()
	arch := root.Children[0]

	_, err = wiki.CreatePage(&arch.ID, "Backend", "backend")
	if err != nil {
		t.Fatalf("Failed to create 'Backend': %v", err)
	}
	backend := arch.Children[0]

	// Jetzt dort einen Slug vorschlagen
	slug, err := wiki.SuggestSlug(backend.ID, "", "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug failed: %v", err)
	}

	if slug != "data-layer" {
		t.Errorf("Expected 'data-layer', got %q", slug)
	}

	// Erzeuge ein zweites mit gleichem Namen → es muss nummeriert werden
	_, err = wiki.CreatePage(&backend.ID, "Data Layer", "data-layer")
	if err != nil {
		t.Fatalf("Failed to create 'Data Layer': %v", err)
	}

	slug2, err := wiki.SuggestSlug(backend.ID, "", "Data Layer")
	if err != nil {
		t.Fatalf("SuggestSlug 2 failed: %v", err)
	}

	if slug2 != "data-layer-1" {
		t.Errorf("Expected 'data-layer-1', got %q", slug2)
	}
}

func TestWiki_FindByPath_Valid(t *testing.T) {
	w := setupTestWiki(t)
	_, _ = w.CreatePage(nil, "Company", "company")

	found, err := w.FindByPath("company")
	if err != nil {
		t.Fatalf("FindByPath failed: %v", err)
	}
	if found.Slug != "company" {
		t.Errorf("Expected slug 'company', got %q", found.Slug)
	}
}

func TestWiki_FindByPath_Invalid(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.FindByPath("does/not/exist")
	if err == nil {
		t.Error("Expected error for invalid path, got none")
	}
}

func TestWiki_SortPages(t *testing.T) {
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent", "parent")
	child1, _ := w.CreatePage(&parent.ID, "Child1", "child1")
	child2, _ := w.CreatePage(&parent.ID, "Child2", "child2")

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
	w := setupTestWiki(t)
	original, _ := w.CreatePage(nil, "Original", "original")

	copied, err := w.CopyPage(original.ID, nil, "Copy of Original", "copy-of-original")
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
	w := setupTestWiki(t)
	parent, _ := w.CreatePage(nil, "Parent", "parent")
	original, _ := w.CreatePage(nil, "Original", "original")

	copied, err := w.CopyPage(original.ID, &parent.ID, "Copy of Original", "copy-of-original")
	if err != nil {
		t.Fatalf("CopyPage with parent failed: %v", err)
	}

	if copied.Parent.ID != parent.ID {
		t.Errorf("Expected parent ID %q, got %q", parent.ID, copied.Parent.ID)
	}
}

func TestWiki_CopyPages_NonExistentSource(t *testing.T) {
	w := setupTestWiki(t)
	_, err := w.CopyPage("non-existent-id", nil, "Copy", "copy")
	if err == nil {
		t.Error("Expected error for non-existent source page, got none")
	}
}

func TestWiki_CopyPages_WithAssets(t *testing.T) {
	w := setupTestWiki(t)
	original, _ := w.CreatePage(nil, "Original", "original")

	originalNode := tree.PageNode{
		ID:    original.ID,
		Title: original.Title,
		Slug:  original.Slug,
	}

	file, _, err := test_utils.CreateMultipartFile("image.png", []byte("image content"))
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	// Save asset for the original page
	if _, err := w.GetAssetService().SaveAssetForPage(&originalNode, file, "image.png"); err != nil {
		t.Fatalf("Failed to save asset for original page: %v", err)
	}

	copied, err := w.CopyPage(original.ID, nil, "Copy of Original", "copy-of-original")
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

func TestWiki_InitDefaultAdmin_UsesGivenPassword(t *testing.T) {
	w := setupTestWiki(t)

	_, err := w.GetUserService().GetUserByEmailOrUsernameAndPassword("admin", "admin")
	if err != nil {
		t.Fatalf("Admin user not found: %v", err)
	}
}

func TestWiki_ResetAdminUserPassword_ChangesPassword(t *testing.T) {
	w := setupTestWiki(t)

	original, err := w.GetUserService().GetUserByEmailOrUsernameAndPassword("admin", "admin")
	if err != nil {
		t.Fatalf("Admin not found: %v", err)
	}

	resetUser, err := w.ResetAdminUserPassword()
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if resetUser.Password == "" {
		t.Fatal("Reset password is empty")
	}

	match, err := w.GetUserService().DoesIDAndPasswordMatch(original.ID, resetUser.Password)
	if err != nil || !match {
		t.Error("Reset password does not match")
	}
}

func TestWiki_Login_SuccessAndFailure(t *testing.T) {
	w := setupTestWiki(t)

	token, err := w.Login("admin", "admin")
	if err != nil || token == nil {
		t.Error("Expected login to succeed with default admin password")
	}

	_, err = w.Login("admin", "wrong")
	if err == nil {
		t.Error("Expected login to fail with wrong password")
	}
}

func TestWiki_ResetAdminPasswordWithoutJWTSecret(t *testing.T) {
	tempDir := t.TempDir()

	// Verwende Dummy-Secret
	wiki, err := NewWiki(tempDir, "supersecure", "", false)
	if err != nil {
		t.Fatalf("Failed to initialize Wiki: %v", err)
	}
	defer wiki.Close()

	user, err := wiki.ResetAdminUserPassword()
	if err != nil {
		t.Fatalf("ResetAdminUserPassword failed: %v", err)
	}

	if user.Username != "admin" {
		t.Errorf("Expected username to be 'admin', got %s", user.Username)
	}
	if user.Password == "" {
		t.Error("Expected new password to be set, got empty string")
	}
}

func TestWiki_EnsurePath_HealsLinksForAllCreatedSegments(t *testing.T) {
	w := setupTestWiki(t)
	defer w.Close()

	// 1) Page A mit Links auf /x und /x/y (existieren noch nicht)
	pageA, err := w.CreatePage(nil, "Page A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}

	_, err = w.UpdatePage(pageA.ID, pageA.Title, pageA.Slug, "Links: [X](/x) and [XY](/x/y)")
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
	_, err = w.EnsurePath("/x/y", "X Y")
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
	dataDir := t.TempDir()

	w, err := NewWiki(dataDir, "admin", "secret", false)
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	defer w.Close()

	// Create A with link to /b
	a, err := w.CreatePage(nil, "Page A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Link to B: [Go](/b)")
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B
	b, err := w.CreatePage(nil, "Page B", "b")
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# Page B")
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Ensure link index
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Delete B
	if err := w.DeletePage(b.ID, false); err != nil {
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
	dataDir := t.TempDir()

	w, err := NewWiki(dataDir, "admin", "secret", false)
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	defer w.Close()

	// Create /docs
	docs, err := w.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}

	// Create /docs/a and /docs/b
	a, err := w.CreatePage(&docs.ID, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage a failed: %v", err)
	}
	b, err := w.CreatePage(&docs.ID, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage b failed: %v", err)
	}

	// A links to B inside subtree
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Link to B: [B](/docs/b)")
	if err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# B")
	if err != nil {
		t.Fatalf("UpdatePage b failed: %v", err)
	}

	// Create survivor /c with incoming link into subtree
	c, err := w.CreatePage(nil, "C", "c")
	if err != nil {
		t.Fatalf("CreatePage c failed: %v", err)
	}
	_, err = w.UpdatePage(c.ID, c.Title, c.Slug, "Incoming link: [B](/docs/b)")
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
	if err := w.DeletePage(docs.ID, true); err != nil {
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
	w := setupTestWiki(t)
	defer w.Close()

	// Create A with links to /b (exists) and /b2 (does not exist yet)
	a, err := w.CreatePage(nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Links: [B](/b) and [B2](/b2)")
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B at /b
	b, err := w.CreatePage(nil, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# B")
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
	_, err = w.UpdatePage(b.ID, b.Title, "b2", "# B (renamed)")
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
	w := setupTestWiki(t)
	defer w.Close()

	// Create subtree: /docs/b
	docs, err := w.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	b, err := w.CreatePage(&docs.ID, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage /docs/b failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# B")
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create A that links to old and future new subtree paths
	a, err := w.CreatePage(nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Links: [Old](/docs/b) and [New](/docs2/b)")
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
	_, err = w.UpdatePage(docs.ID, docs.Title, "docs2", "# Docs")
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
	w := setupTestWiki(t)
	defer w.Close()

	// Create A that links to /b (old path) and /projects/b (future path)
	a, err := w.CreatePage(nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Links: [B](/b) and [B2](/projects/b)")
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Create B at /b
	b, err := w.CreatePage(nil, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# B")
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create parent /projects (target)
	projects, err := w.CreatePage(nil, "Projects", "projects")
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
	if err := w.MovePage(b.ID, projects.ID); err != nil {
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
	w := setupTestWiki(t)
	defer w.Close()

	// Create subtree /docs/b
	docs, err := w.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}
	b, err := w.CreatePage(&docs.ID, "B", "b")
	if err != nil {
		t.Fatalf("CreatePage /docs/b failed: %v", err)
	}
	_, err = w.UpdatePage(b.ID, b.Title, b.Slug, "# B")
	if err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	// Create target parent /archive
	archive, err := w.CreatePage(nil, "Archive", "archive")
	if err != nil {
		t.Fatalf("CreatePage archive failed: %v", err)
	}

	// Create A that links to old and future new subtree paths
	a, err := w.CreatePage(nil, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Links: [Old](/docs/b) and [New](/archive/docs/b)")
	if err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	// Materialize graph
	if err := w.ReindexLinks(); err != nil {
		t.Fatalf("ReindexLinks failed: %v", err)
	}

	// Move /docs under /archive => /archive/docs/b exists, /docs/b disappears
	if err := w.MovePage(docs.ID, archive.ID); err != nil {
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
	w := setupTestWiki(t)
	defer w.Close()

	// Create /docs with /docs/shared and /docs/a
	docs, err := w.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage docs failed: %v", err)
	}

	docsShared, err := w.CreatePage(&docs.ID, "Shared", "shared")
	if err != nil {
		t.Fatalf("CreatePage /docs/shared failed: %v", err)
	}
	_, err = w.UpdatePage(docsShared.ID, docsShared.Title, docsShared.Slug, "# Docs Shared")
	if err != nil {
		t.Fatalf("UpdatePage /docs/shared failed: %v", err)
	}

	a, err := w.CreatePage(&docs.ID, "A", "a")
	if err != nil {
		t.Fatalf("CreatePage /docs/a failed: %v", err)
	}
	// Important: relative link
	_, err = w.UpdatePage(a.ID, a.Title, a.Slug, "Relative: [S](../shared)")
	if err != nil {
		t.Fatalf("UpdatePage /docs/a failed: %v", err)
	}

	// Create /guide with /guide/shared (different page!)
	guide, err := w.CreatePage(nil, "Guide", "guide")
	if err != nil {
		t.Fatalf("CreatePage guide failed: %v", err)
	}

	guideShared, err := w.CreatePage(&guide.ID, "Shared", "shared")
	if err != nil {
		t.Fatalf("CreatePage /guide/shared failed: %v", err)
	}
	_, err = w.UpdatePage(guideShared.ID, guideShared.Title, guideShared.Slug, "# Guide Shared")
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
	if err := w.MovePage(a.ID, guide.ID); err != nil {
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
