package wiki

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
		EnableRevision:      true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return wikiInstance
}

func createWikiTestInstanceWithWorkspace(t *testing.T, workspace Workspace) *Wiki {
	t.Helper()
	wikiInstance, err := NewWiki(&WikiOptions{
		Workspace:           workspace,
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableRevision:      true,
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

	out, err := wikipages.NewCreatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log).Execute(
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

	out, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log).Execute(
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

	if err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.asset, w.newPageOrchestrator(), w.log).Execute(
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

func TestWiki_DefaultWorkspaceKeepsExistingStorageLayout(t *testing.T) {
	dataDir := t.TempDir()
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:          dataDir,
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	if got := wikiInstance.GetStorageDir(); got != dataDir {
		t.Fatalf("GetStorageDir() = %q, want %q", got, dataDir)
	}
	if got, want := wikiInstance.GetRootDir(), filepath.Join(dataDir, "root"); got != want {
		t.Fatalf("GetRootDir() = %q, want %q", got, want)
	}
	if got := wikiInstance.Workspace(); got.ID != "default" || got.DataDir != dataDir || got.RootDir != filepath.Join(dataDir, "root") {
		t.Fatalf("unexpected default workspace: %#v", got)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "welcome-to-leafwiki.md")); err != nil {
		t.Fatalf("expected welcome page in default root dir: %v", err)
	}
}

func TestWiki_ExplicitWorkspaceStoresContentInRootDirAndStateInDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	rootDir := filepath.Join(t.TempDir(), "content")
	w := createWikiTestInstanceWithWorkspace(t, Workspace{
		ID:      "default",
		DataDir: dataDir,
		RootDir: rootDir,
	})
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	if got := w.GetStorageDir(); got != dataDir {
		t.Fatalf("GetStorageDir() = %q, want %q", got, dataDir)
	}
	if got := w.GetRootDir(); got != rootDir {
		t.Fatalf("GetRootDir() = %q, want %q", got, rootDir)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "welcome-to-leafwiki.md")); err != nil {
		t.Fatalf("expected welcome page in explicit root dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "welcome-to-leafwiki.md")); !os.IsNotExist(err) {
		t.Fatalf("expected no welcome page in data dir root, got err=%v", err)
	}
	for _, rel := range []string{
		"users.db",
		"sessions.db",
		"search.db",
		"links.db",
		"tags.db",
		"properties.db",
		"assets",
		".leafwiki",
		".importer",
		"branding",
	} {
		if _, err := os.Stat(filepath.Join(dataDir, rel)); err != nil {
			t.Fatalf("expected app state %s in data dir: %v", rel, err)
		}
	}

	if err := w.branding.UpdateBranding("Workspace Wiki"); err != nil {
		t.Fatalf("UpdateBranding failed: %v", err)
	}
	logo, err := os.CreateTemp(t.TempDir(), "logo-*.png")
	if err != nil {
		t.Fatalf("CreateTemp logo failed: %v", err)
	}
	defer func() {
		if err := logo.Close(); err != nil {
			t.Fatalf("Close logo failed: %v", err)
		}
	}()
	if _, err := logo.Write([]byte("png")); err != nil {
		t.Fatalf("Write logo failed: %v", err)
	}
	if _, err := logo.Seek(0, 0); err != nil {
		t.Fatalf("Seek logo failed: %v", err)
	}
	if _, err := w.branding.UploadLogo(logo, "logo.png"); err != nil {
		t.Fatalf("UploadLogo failed: %v", err)
	}
	for _, rel := range []string{
		"branding.json",
		filepath.Join("branding", "logo.png"),
	} {
		if _, err := os.Stat(filepath.Join(dataDir, rel)); err != nil {
			t.Fatalf("expected branding state %s in data dir: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(rootDir, rel)); !os.IsNotExist(err) {
			t.Fatalf("expected no branding state %s in root dir, got err=%v", rel, err)
		}
	}
}

func TestWiki_RejectsWorkspaceWithSameDataAndRootDir(t *testing.T) {
	dir := t.TempDir()

	_, err := NewWiki(&WikiOptions{
		Workspace:           Workspace{ID: "default", DataDir: dir, RootDir: filepath.Clean(filepath.Join(dir, "."))},
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err == nil {
		t.Fatalf("expected same data/root dir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must be different from data dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWiki_RejectsWorkspaceWhenRootDirContainsDataDir(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "wiki")
	dataDir := filepath.Join(rootDir, "data")

	_, err := NewWiki(&WikiOptions{
		Workspace:           Workspace{ID: "default", DataDir: dataDir, RootDir: rootDir},
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err == nil {
		t.Fatalf("expected root dir containing data dir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not contain data dir") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(rootDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected invalid root dir not to be created before startup, got err=%v", statErr)
	}
}

func TestWiki_NormalizesWorkspacePathsBeforeInitializingServices(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "data")
	rootDir := filepath.Join(baseDir, "content")

	w, err := NewWiki(&WikiOptions{
		Workspace:           Workspace{ID: "default", DataDir: " " + dataDir + string(os.PathSeparator) + "." + " ", RootDir: " " + rootDir + string(os.PathSeparator) + "." + " "},
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	if got := w.GetStorageDir(); got != dataDir {
		t.Fatalf("GetStorageDir() = %q, want normalized %q", got, dataDir)
	}
	if got := w.GetRootDir(); got != rootDir {
		t.Fatalf("GetRootDir() = %q, want normalized %q", got, rootDir)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "users.db")); err != nil {
		t.Fatalf("expected auth state in normalized data dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "welcome-to-leafwiki.md")); err != nil {
		t.Fatalf("expected content in normalized root dir: %v", err)
	}
}

func TestWiki_DeletePage_WithChildren(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
	parent := createPageForTest(t, w, "system", nil, "Parent", "parent", pageNodeKind())
	createPageForTest(t, w, "system", &parent.ID, "Child", "child", pageNodeKind())

	err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.asset, w.newPageOrchestrator(), w.log).Execute(
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

func TestWiki_EnsureBaselineRevisions_SkipsUnreadablePages(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	okPage := createPageForTest(t, w, "system", nil, "Healthy", "healthy", pageNodeKind())
	brokenPage := createPageForTest(t, w, "system", nil, "Broken", "broken", pageNodeKind())

	if err := w.revision.DeletePageData(okPage.ID); err != nil {
		t.Fatalf("DeletePageData(okPage) failed: %v", err)
	}
	if err := w.revision.DeletePageData(brokenPage.ID); err != nil {
		t.Fatalf("DeletePageData(brokenPage) failed: %v", err)
	}

	brokenPath := filepath.Join(w.GetRootDir(), "broken.md")
	if err := os.Remove(brokenPath); err != nil {
		t.Fatalf("Remove(%s) failed: %v", brokenPath, err)
	}

	w.ensureBaselineRevisions()

	okRevisions, err := w.revision.ListRevisions(okPage.ID)
	if err != nil {
		t.Fatalf("ListRevisions(okPage) failed: %v", err)
	}
	if len(okRevisions) == 0 {
		t.Fatalf("expected baseline revision for readable page")
	}

	brokenRevisions, err := w.revision.ListRevisions(brokenPage.ID)
	if err != nil {
		t.Fatalf("ListRevisions(brokenPage) failed: %v", err)
	}
	if len(brokenRevisions) != 0 {
		t.Fatalf("expected no baseline revision for unreadable page, got %d", len(brokenRevisions))
	}
}
