package wiki

import (
	"context"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
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

func createPageForTest(t *testing.T, w *Wiki, userID string, parentID *string, title, slug string, kind *tree.NodeKind) *tree.Page {
	t.Helper()

	out, err := wikipages.NewCreatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log).Execute(
		context.Background(),
		wikipages.CreatePageInput{UserID: userID, ParentID: parentID, Title: title, Slug: slug, Kind: kind},
	)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	return out.Page
}

func updatePageForTest(t *testing.T, w *Wiki, userID, id, title, slug string, content *string, kind *tree.NodeKind) *tree.Page {
	t.Helper()

	current, err := w.tree.GetPage(id)
	if err != nil {
		t.Fatalf("GetPage before update failed: %v", err)
	}

	out, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.revision, w.links, w.log).Execute(
		context.Background(),
		wikipages.UpdatePageInput{UserID: userID, ID: id, Version: current.Version(), Title: title, Slug: slug, Content: content, Kind: kind},
	)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	return out.Page
}

func deletePageForTest(t *testing.T, w *Wiki, userID, id string, recursive bool) {
	t.Helper()

	current, err := w.tree.GetPage(id)
	if err != nil {
		t.Fatalf("GetPage before delete failed: %v", err)
	}

	if err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.links, w.asset, w.log).Execute(
		context.Background(),
		wikipages.DeletePageInput{UserID: userID, ID: id, Version: current.Version(), Recursive: recursive},
	); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}
}

func TestWiki_DeletePage_Simple(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	page := createPageForTest(t, w, "system", nil, "Trash", "trash", pageNodeKind())
	deletePageForTest(t, w, "system", page.ID, false)
	if _, err := w.tree.GetPage(page.ID); err == nil {
		t.Fatalf("expected deleted page to be gone")
	}
}

func TestWiki_DeletePage_WithChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent := createPageForTest(t, w, "system", nil, "Parent", "parent", pageNodeKind())
	createPageForTest(t, w, "system", &parent.ID, "Child", "child", pageNodeKind())

	err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.links, w.asset, w.log).Execute(
		context.Background(),
		wikipages.DeletePageInput{UserID: "system", ID: parent.ID, Version: parent.Version(), Recursive: false},
	)
	if err == nil {
		t.Error("Expected error when deleting parent with children")
	}
}

func TestWiki_DeletePage_Recursive(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent := createPageForTest(t, w, "system", nil, "Parent", "parent", pageNodeKind())
	child := createPageForTest(t, w, "system", &parent.ID, "Child", "child", pageNodeKind())

	deletePageForTest(t, w, "system", parent.ID, true)
	if _, err := w.tree.GetPage(parent.ID); err == nil {
		t.Fatalf("expected deleted parent to be gone")
	}
	if _, err := w.tree.GetPage(child.ID); err == nil {
		t.Fatalf("expected deleted child to be gone")
	}
}

func TestWiki_DeletePage_PurgesRevisionData(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	page := createPageForTest(t, w, "system", nil, "Page", "page", pageNodeKind())
	content := "updated"
	updatePageForTest(t, w, "system", page.ID, page.Title, page.Slug, &content, pageNodeKind())

	deletePageForTest(t, w, "system", page.ID, false)

	revisions, err := w.revision.ListRevisions(page.ID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("expected revisions to be purged, got %#v", revisions)
	}
}

func TestWiki_InitDefaultAdmin_UsesGivenPassword(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	_, err := w.user.GetUserByEmailOrUsernameAndPassword("admin", "admin")
	if err != nil {
		t.Fatalf("Admin user not found: %v", err)
	}
}

func TestWiki_Login_SuccessAndFailure(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	authSvc := w.auth
	if authSvc == nil {
		t.Fatal("expected auth service to be initialized")
	}

	token, err := authSvc.Login("admin", "admin")
	if err != nil || token == nil {
		t.Error("Expected login to succeed with default admin password")
	}

	_, err = authSvc.Login("admin", "wrong")
	if err == nil {
		t.Error("Expected login to fail with wrong password")
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
	if wikiInstance.auth != nil {
		t.Error("Expected auth service to be nil when AuthDisabled is true")
	}
}

func TestWiki_AuthDisabled_LoginUnavailable(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Auth operations are unavailable when auth is disabled.
	if wikiInstance.auth != nil {
		t.Error("Expected auth service to be nil when AuthDisabled is true")
	}
}

func TestWiki_AuthDisabled_LogoutUnavailable(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Auth operations are unavailable when auth is disabled.
	if wikiInstance.auth != nil {
		t.Error("Expected auth service to be nil when AuthDisabled is true")
	}
}

func TestWiki_AuthDisabled_RefreshTokenUnavailable(t *testing.T) {
	// Create a wiki instance with AuthDisabled set to true
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	// Auth operations are unavailable when auth is disabled.
	if wikiInstance.auth != nil {
		t.Error("Expected auth service to be nil when AuthDisabled is true")
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
	page := createPageForTest(t, wikiInstance, "system", nil, "Test Page", "test-page", pageNodeKind())

	if page.Title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %q", page.Title)
	}

	// Test updating a page
	var updatedContent = "# Content"
	updatedPage := updatePageForTest(t, wikiInstance, "system", page.ID, "Updated Title", "updated-slug", &updatedContent, pageNodeKind())

	if updatedPage.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got %q", updatedPage.Title)
	}

	// Test getting a page
	retrievedPage, err := wikiInstance.tree.GetPage(page.ID)
	if err != nil {
		t.Fatalf("Failed to get page with AuthDisabled: %v", err)
	}

	if retrievedPage.ID != page.ID {
		t.Errorf("Expected ID %q, got %q", page.ID, retrievedPage.ID)
	}

	// Test deleting a page
	deletePageForTest(t, wikiInstance, "system", page.ID, false)
}
