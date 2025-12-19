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
	if err := w.ReindexBacklinks(); err != nil {
		t.Fatalf("ReindexBacklinks failed: %v", err)
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
