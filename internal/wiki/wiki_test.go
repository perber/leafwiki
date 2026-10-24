package wiki

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/restore"
	"github.com/perber/wiki/internal/test_utils"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikiresync "github.com/perber/wiki/internal/wiki/resync"
)

func createWikiTestInstance(t *testing.T) *Wiki {
	return createWikiTestInstanceWithMetrics(t, nil)
}

func createWikiTestInstanceWithMetrics(t *testing.T, metrics *httpmetrics.HTTPMetrics) *Wiki {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
		EnableRevision:      true,
		Metrics:             metrics,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	return wikiInstance
}

func metricsBody(t *testing.T, metrics *httpmetrics.HTTPMetrics) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.HTTPHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected metrics endpoint to return 200, got %d", rec.Code)
	}
	return rec.Body.String()
}

func pageNodeKind() *tree.NodeKind {
	kind := tree.NodeKindPage
	return &kind
}

func createPageForTest(t *testing.T, w *Wiki, userID string, parentID *string, title, slug string, kind *tree.NodeKind) *tree.Page {
	t.Helper()

	out, err := wikipages.NewCreatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log, nil).Execute(
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

	out, err := wikipages.NewUpdatePageUseCase(w.tree, w.slug, w.newPageOrchestrator(), w.log, nil).Execute(
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

	if err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.asset, w.favorites, w.newPageOrchestrator(), w.log, nil).Execute(
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

	err := wikipages.NewDeletePageUseCase(w.tree, w.revision, w.asset, w.favorites, w.newPageOrchestrator(), w.log, nil).Execute(
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

func TestWiki_TriggerResyncAsync_EmitsMetrics(t *testing.T) {
	metrics := httpmetrics.NewHTTPMetrics("test")
	w := createWikiTestInstanceWithMetrics(t, metrics)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	w.TriggerResyncAsync()

	deadline := time.Now().Add(5 * time.Second)
	for {
		status := w.resyncJob.Status()
		if status.Done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for resync to finish")
		}
		time.Sleep(10 * time.Millisecond)
	}

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_resync_runs_total{result="success"} 1`) {
		t.Fatalf("expected resync run metric, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_resync_duration_seconds_bucket{result="success"`) {
		t.Fatalf("expected resync duration metric, got: %s", body)
	}
}

func TestWiki_Metrics_ExposesPageAndUserGauges(t *testing.T) {
	metrics := httpmetrics.NewHTTPMetrics("test")
	w := createWikiTestInstanceWithMetrics(t, metrics)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	body := metricsBody(t, metrics)
	// Fresh instance: the seeded admin plus the auto-created welcome page.
	if !strings.Contains(body, `leafwiki_users{role="admin"} 1`) {
		t.Fatalf("expected one admin user gauge, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_users{role="editor"} 0`) {
		t.Fatalf("expected editor role series seeded at zero, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_users_with_totp 0`) {
		t.Fatalf("expected totp gauge at zero, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_pages{kind="page"} 1`) {
		t.Fatalf("expected one page (welcome) gauge, got: %s", body)
	}

	createPageForTest(t, w, "system", nil, "Second", "second", pageNodeKind())

	body = metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_pages{kind="page"} 2`) {
		t.Fatalf("expected page gauge to reflect the new page, got: %s", body)
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

	_, err := w.user.GetUserByEmailOrUsernameAndPassword("admin", "adminpassword")
	if err != nil {
		t.Fatalf("Admin user not found: %v", err)
	}
}

func TestWiki_InitDefaultAdmin_UsesGivenUsernameAndEmail(t *testing.T) {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:          t.TempDir(),
		AdminUsername:       "root",
		AdminEmail:          "root@example.com",
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	user, err := wikiInstance.user.GetUserByEmailOrUsernameAndPassword("root", "adminpassword")
	if err != nil {
		t.Fatalf("Admin user with custom username not found: %v", err)
	}
	if user.Email != "root@example.com" {
		t.Errorf("Expected admin email %q, got %q", "root@example.com", user.Email)
	}
}

// Regression test: an unreadable users.db used to be replaced by an empty
// one, and InitDefaultAdmin then created a fresh admin from the startup
// flags. NewWiki must fail before that and leave the file alone (ADR-0013).
func TestNewWiki_CorruptUsersDB_FailsWithoutCreatingDefaultAdmin(t *testing.T) {
	dir := t.TempDir()
	files := test_utils.WriteCorruptSQLiteDB(t, filepath.Join(dir, "users.db"))

	w, err := NewWiki(&WikiOptions{
		StorageDir:          dir,
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})

	if err == nil {
		_ = w.Close()
		t.Fatal("NewWiki: expected an error for a corrupt users.db, got nil")
	}
	var corrupt *sqliteutil.CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		t.Fatalf("NewWiki error = %v (%T), want it to wrap *sqliteutil.CorruptDatabaseError", err, err)
	}
	test_utils.AssertFilesUnchanged(t, files)
}

// assertNoOpenSQLiteDBs fails for every *.db-wal file left in dir. SQLite
// removes the WAL file when the last connection to a database closes, so a
// leftover one means some store's handle was never closed — which on Windows
// keeps the file locked.
func assertNoOpenSQLiteDBs(t *testing.T, dir string) {
	t.Helper()
	leftovers, err := filepath.Glob(filepath.Join(dir, "*.db-wal"))
	if err != nil {
		t.Fatalf("glob WAL files: %v", err)
	}
	for _, wal := range leftovers {
		t.Errorf("%s still open after Close (WAL file left behind)", strings.TrimSuffix(filepath.Base(wal), "-wal"))
	}
}

// Regression test: tags.db and properties.db were opened at startup but
// never closed by Wiki.Close.
func TestWiki_Close_ClosesAllDatabases(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWiki(&WikiOptions{
		StorageDir:             dir,
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableRevision:         true,
		EnableAPIKeyManagement: true,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	assertNoOpenSQLiteDBs(t, dir)
}

// Regression test: when a later boot step failed, NewWiki returned the error
// but left the databases opened by earlier steps open.
func TestNewWiki_FailedBoot_ClosesOpenedDatabases(t *testing.T) {
	dir := t.TempDir()
	// A regular file where the content directory belongs makes the tree
	// load fail after the auth stores are already open.
	if err := os.WriteFile(filepath.Join(dir, "root"), []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}

	w, err := NewWiki(&WikiOptions{
		StorageDir:          dir,
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err == nil {
		_ = w.Close()
		t.Fatal("NewWiki: expected an error when root is not a directory, got nil")
	}

	assertNoOpenSQLiteDBs(t, dir)
}

func TestWiki_Login_SuccessAndFailure(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	authSvc := w.auth
	if authSvc == nil {
		t.Fatal("expected auth service to be initialized")
	}

	token, err := authSvc.Login("admin", "adminpassword")
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

// TestWiki_AuthDisabled_APIKeysUnavailable guards against the exact
// regression a code review caught: API-key auth must not remain active when
// --disable-auth is set, or a key created before a later restart with
// --disable-auth could keep authenticating (and narrowing/blocking)
// requests in a mode where auth is supposed to be irrelevant.
func TestWiki_AuthDisabled_APIKeysUnavailable(t *testing.T) {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:   t.TempDir(),
		AuthDisabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance with AuthDisabled: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	if wikiInstance.apiKeys != nil {
		t.Error("Expected api key service to be nil when AuthDisabled is true")
	}
	if wikiInstance.APIKeyService() != nil {
		t.Error("Expected APIKeyService() to be nil when AuthDisabled is true")
	}
}

// TestWiki_APIKeyManagementDisabled_APIKeysUnavailable ensures the
// experimental API key feature stays off by default: with auth enabled but
// EnableAPIKeyManagement left false, APIKeyService() must be nil so the
// Bearer middleware never gets registered and the admin CRUD routes report
// the feature as disabled.
func TestWiki_APIKeyManagementDisabled_APIKeysUnavailable(t *testing.T) {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableAPIKeyManagement: false,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	if wikiInstance.APIKeyService() != nil {
		t.Error("Expected APIKeyService() to be nil when EnableAPIKeyManagement is false")
	}
}

// TestWiki_APIKeyManagementEnabled_APIKeysAvailable confirms opting in via
// EnableAPIKeyManagement actually constructs the service (auth must also be
// enabled, since API keys are meaningless without it).
func TestWiki_APIKeyManagementEnabled_APIKeysAvailable(t *testing.T) {
	wikiInstance, err := NewWiki(&WikiOptions{
		StorageDir:             t.TempDir(),
		AdminPassword:          "adminpassword",
		JWTSecret:              "secretkey",
		AccessTokenTimeout:     15 * time.Minute,
		RefreshTokenTimeout:    7 * 24 * time.Hour,
		EnableAPIKeyManagement: true,
	})
	if err != nil {
		t.Fatalf("Failed to create wiki instance: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(wikiInstance.Close, t)

	if wikiInstance.APIKeyService() == nil {
		t.Error("Expected APIKeyService() to be non-nil when EnableAPIKeyManagement is true")
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

	brokenPath := filepath.Join(w.storageDir, "root", "broken.md")
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

func TestWiki_ResyncRespectsLeafwikiignore(t *testing.T) {
	tmp := t.TempDir()

	rootDir := filepath.Join(tmp, "root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// .leafwikiignore goes in root/ alongside the markdown files
	test_utils.WriteFile(t, tmp, "root/.leafwikiignore", "secret.md")
	test_utils.WriteFile(t, tmp, "root/public.md", "# Public Page")
	test_utils.WriteFile(t, tmp, "root/secret.md", "# Secret Page")

	w, err := NewWiki(&WikiOptions{
		StorageDir:          tmp,
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	// Verify tree has only the public page
	tree := w.tree.GetTree()
	if len(tree.Children) != 1 {
		t.Fatalf("expected 1 child in root, got %d", len(tree.Children))
	}
	if tree.Children[0].Slug != "public" {
		t.Fatalf("expected child slug 'public', got %q", tree.Children[0].Slug)
	}

	// Verify ignored page is not findable
	lookup, err := w.tree.LookupPagePath("secret")
	if err != nil {
		t.Fatalf("LookupPagePath: %v", err)
	}
	if lookup.Exists {
		t.Fatal("expected ignored page 'secret' to not exist")
	}
}

func TestWiki_ReloadWithProgress_PicksUpDiskChangesAndFinishesInSearchPhase(t *testing.T) {
	w := createWikiTestInstance(t)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	if err := os.WriteFile(filepath.Join(w.GetStorageDir(), "root", "from-disk.md"), []byte("# From Disk\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if !w.resyncJob.Start() {
		t.Fatal("expected resync job to start")
	}

	w.reloadWithProgress(context.Background())

	status := w.resyncJob.Status()
	want := wikiresync.JobStatus{Running: false, Phase: string(wikiresync.PhaseSearch), Done: true}
	if status != want {
		t.Fatalf("status = %+v, want %+v", status, want)
	}
	found := false
	if err := w.tree.WalkNodes(func(id string) error {
		if page, err := w.tree.GetPage(id); err == nil && page.Slug == "from-disk" {
			found = true
		}
		return nil
	}); err != nil {
		t.Fatalf("WalkNodes failed: %v", err)
	}
	if !found {
		t.Fatal("expected reload to pick up the page written to disk")
	}
}

func TestWiki_ReloadWithProgress_CanceledContext_FinishesWithErrorAndFailureMetric(t *testing.T) {
	metrics := httpmetrics.NewHTTPMetrics("test")
	w := createWikiTestInstanceWithMetrics(t, metrics)
	defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !w.resyncJob.Start() {
		t.Fatal("expected resync job to start")
	}

	w.reloadWithProgress(ctx)

	status := w.resyncJob.Status()
	want := wikiresync.JobStatus{
		Running: false,
		Phase:   string(wikiresync.PhaseTree),
		Done:    true,
		Error:   "tree reconstruction failed: context canceled",
	}
	if status != want {
		t.Fatalf("status = %+v, want %+v", status, want)
	}
	body := metricsBody(t, metrics)
	for _, metric := range []string{
		`leafwiki_resync_runs_total{result="error"} 1`,
		`leafwiki_resync_failures_total{result="error"} 1`,
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("expected %s in metrics, got: %s", metric, body)
		}
	}
}

func newWikiOnDir(dir string) (*Wiki, error) {
	return NewWiki(&WikiOptions{
		StorageDir:          dir,
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
}

// seedWikiWithUser boots a wiki on dir, adds the user alice with a favorite
// and closes it again, as a previous run of the server would. Like
// cmd/leafwiki, it creates the data dir first.
func seedWikiWithUser(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}
	w, err := newWikiOnDir(dir)
	if err != nil {
		t.Fatalf("seed NewWiki: %v", err)
	}
	alice, err := w.UserService().CreateUser("alice", "alice@example.com", "alicepassword", "editor")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := w.Favorites().Add(alice.ID, "page-1"); err != nil {
		t.Fatalf("Favorites.Add: %v", err)
	}
	if _, err := w.auth.Login("alice", "alicepassword"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("seed Close: %v", err)
	}
}

// A corrupt sessions.db, favorites.db or usersettings.db is quarantined and
// the wiki still boots with every user (and, for the other stores, their
// data) intact, with an absolute and the default relative --data-dir
// (ADR-0013).
func TestNewWiki_CorruptQuarantinedStore_BootsAndKeepsUsers(t *testing.T) {
	stores := []struct {
		filename      string
		wantFavorites int // favorites alice still has after the restart
	}{
		{filename: "sessions.db", wantFavorites: 1},
		{filename: "favorites.db", wantFavorites: 0},
		{filename: "usersettings.db", wantFavorites: 1},
	}

	for _, relative := range []bool{false, true} {
		for _, s := range stores {
			name := "absolute/" + s.filename
			if relative {
				name = "relative/" + s.filename
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				if relative {
					t.Chdir(dir)
					dir = "./data"
				}
				seedWikiWithUser(t, dir)
				dbPath := filepath.Join(dir, s.filename)
				files := test_utils.WriteCorruptSQLiteDB(t, dbPath)

				w, err := newWikiOnDir(dir)
				if err != nil {
					t.Fatalf("NewWiki with a corrupt %s: %v", s.filename, err)
				}
				defer test_utils.WrapCloseWithErrorCheck(w.Close, t)

				alice, err := w.UserService().GetUserByUsername("alice")
				if err != nil {
					t.Fatalf("alice lost after the restart: %v", err)
				}
				if _, err := w.auth.Login("alice", "alicepassword"); err != nil {
					t.Fatalf("Login after the restart: %v", err)
				}
				favs, err := w.Favorites().ListPageIDsForUser(alice.ID)
				if err != nil {
					t.Fatalf("ListPageIDsForUser: %v", err)
				}
				if len(favs) != s.wantFavorites {
					t.Fatalf("favorites = %v, want %d", favs, s.wantFavorites)
				}
				if err := w.Favorites().Add(alice.ID, "page-2"); err != nil {
					t.Fatalf("Favorites.Add after the restart: %v", err)
				}
				test_utils.AssertSQLiteQuarantined(t, dbPath, files)
			})
		}
	}
}

// The default --data-dir is the relative ./data: a corrupt users.db there
// must stop NewWiki with every byte (and the -wal) in place, and a healthy
// one must keep its users across restarts (ADR-0013).
func TestNewWiki_RelativeDataDir_UsersDB(t *testing.T) {
	t.Run("healthy users.db keeps its users", func(t *testing.T) {
		t.Chdir(t.TempDir())
		seedWikiWithUser(t, "./data")

		w, err := newWikiOnDir("./data")
		if err != nil {
			t.Fatalf("NewWiki: %v", err)
		}
		defer test_utils.WrapCloseWithErrorCheck(w.Close, t)
		if _, err := w.UserService().GetUserByUsername("alice"); err != nil {
			t.Fatalf("alice lost after the restart: %v", err)
		}
		test_utils.AssertNoSQLiteQuarantine(t, filepath.Join("data", "users.db"))
	})

	t.Run("corrupt users.db fails and keeps every byte", func(t *testing.T) {
		t.Chdir(t.TempDir())
		seedWikiWithUser(t, "./data")
		files := test_utils.WriteCorruptSQLiteDB(t, filepath.Join("data", "users.db"))

		w, err := newWikiOnDir("./data")

		if err == nil {
			_ = w.Close()
			t.Fatal("NewWiki: expected an error for a corrupt users.db, got nil")
		}
		var corrupt *sqliteutil.CorruptDatabaseError
		if !errors.As(err, &corrupt) {
			t.Fatalf("NewWiki error = %v (%T), want *sqliteutil.CorruptDatabaseError", err, err)
		}
		test_utils.AssertFilesUnchanged(t, files)
		test_utils.AssertNoSQLiteQuarantine(t, filepath.Join("data", "users.db"))
	})
}

// Every store the composition root registers for live restore must also be
// listed in restore.StateDBFiles, or snapshots would not capture its file and
// a restore would not swap it. Its File label must also be the database the
// store actually opens: the store packages name their files themselves, so a
// renamed file or a mixed-up label would otherwise drop it from snapshots.
func TestWiki_StateStores_AreRegisteredForBackupAndRestore(t *testing.T) {
	tests := []struct {
		name          string
		enableAPIKeys bool
		wantFiles     []string
	}{
		{name: "api key management on", enableAPIKeys: true, wantFiles: []string{"api_keys.db", "favorites.db", "usersettings.db"}},
		{name: "api key management off", enableAPIKeys: false, wantFiles: []string{"favorites.db", "usersettings.db"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storageDir := t.TempDir()
			w, err := NewWiki(&WikiOptions{
				StorageDir:             storageDir,
				AdminPassword:          "adminpassword",
				JWTSecret:              "secretkey",
				AccessTokenTimeout:     15 * time.Minute,
				RefreshTokenTimeout:    7 * 24 * time.Hour,
				EnableAPIKeyManagement: tc.enableAPIKeys,
			})
			if err != nil {
				t.Fatalf("NewWiki: %v", err)
			}
			t.Cleanup(func() { _ = w.Close() })

			registered := map[string]bool{}
			for _, f := range restore.StateDBFiles() {
				registered[f] = true
			}
			var got []string
			for _, s := range w.StateStores() {
				if s.Store == nil || reflect.ValueOf(s.Store).IsNil() {
					t.Errorf("StateStores lists %s with a nil store", s.File)
				}
				if !registered[s.File] {
					t.Errorf("StateStores lists %s, which restore.StateDBFiles does not capture", s.File)
				}
				got = append(got, s.File)
			}
			if !reflect.DeepEqual(got, tc.wantFiles) {
				t.Errorf("StateStores files = %v, want %v", got, tc.wantFiles)
			}

			// Release every store and delete its files, then reopen one store
			// at a time: the file named by its label must be the one that
			// comes back.
			for _, s := range w.StateStores() {
				if err := s.Store.PauseForSwap(); err != nil {
					t.Fatalf("PauseForSwap %s: %v", s.File, err)
				}
			}
			for _, f := range restore.StateDBFiles() {
				for _, suffix := range []string{"", "-wal", "-shm"} {
					if err := os.Remove(filepath.Join(storageDir, f+suffix)); err != nil && !os.IsNotExist(err) {
						t.Fatalf("remove %s%s: %v", f, suffix, err)
					}
				}
			}
			for _, s := range w.StateStores() {
				if err := s.Store.Replace(storageDir); err != nil {
					t.Fatalf("Replace %s: %v", s.File, err)
				}
				if _, err := os.Stat(filepath.Join(storageDir, s.File)); err != nil {
					t.Errorf("StateStores lists %s, but its store does not open that file: %v", s.File, err)
				}
			}
		})
	}
}
