package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki"
)

func createRevisionTestWiki(t *testing.T) *wiki.Wiki {
	t.Helper()

	instance, err := wiki.NewWiki(&wiki.WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "admin",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewWiki failed: %v", err)
	}
	return instance
}

func TestRestorePageHandlerReturnsStructuredRevisionError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	router := gin.New()
	router.POST("/trash/:id/restore", func(c *gin.Context) {
		c.Set("user", &coreauth.User{ID: "editor", Role: coreauth.RoleEditor})
		RestorePageHandler(w)(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/trash/missing-page/restore", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}

	var body RevisionErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "revision_restore_trash_not_found" {
		t.Fatalf("error.code = %q", body.Error.Code)
	}
	if body.Error.Template != "trash entry for page %s not found" {
		t.Fatalf("error.template = %q", body.Error.Template)
	}
	if len(body.Error.Args) != 1 || body.Error.Args[0] != "missing-page" {
		t.Fatalf("error.args = %#v", body.Error.Args)
	}
}

func TestGetPageRevisionAssetHandlerReturnsAssetBlobAfterLiveDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	kind := tree.NodeKindPage
	page, err := w.CreatePage("editor", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	tmpFile, err := os.CreateTemp(t.TempDir(), "revision-asset-*.png")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer func() { _ = tmpFile.Close() }()
	if _, err := tmpFile.WriteString("asset-image"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	if _, err := w.UploadAsset("editor", page.ID, tmpFile, "image.png", 1024); err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}

	if err := w.DeleteAsset("editor", page.ID, "image.png"); err != nil {
		t.Fatalf("DeleteAsset failed: %v", err)
	}

	router := gin.New()
	router.GET("/pages/:id/revisions/:revisionId/assets/*name", GetPageRevisionAssetHandler(w))

	req := httptest.NewRequest(http.MethodGet, "/pages/"+page.ID+"/revisions/"+rev.ID+"/assets/image.png", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	if body := resp.Body.String(); body != "asset-image" {
		t.Fatalf("body = %q", body)
	}
	if contentType := resp.Header().Get("Content-Type"); contentType == "" {
		t.Fatal("expected Content-Type header")
	}
}

func TestGetPageRevisionAssetHandlerReturnsStructuredNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	kind := tree.NodeKindPage
	page, err := w.CreatePage("editor", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "historical content"
	page, err = w.UpdatePage("editor", page.ID, page.Title, page.Slug, &content, &kind)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}

	router := gin.New()
	router.GET("/pages/:id/revisions/:revisionId/assets/*name", GetPageRevisionAssetHandler(w))

	req := httptest.NewRequest(http.MethodGet, "/pages/"+page.ID+"/revisions/"+rev.ID+"/assets/missing.png", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}

	var body RevisionErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "revision_preview_asset_not_found" {
		t.Fatalf("error.code = %q", body.Error.Code)
	}
}

func TestGetPageRevisionHandlerReturnsSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	kind := tree.NodeKindPage
	page, err := w.CreatePage("editor", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "historical content"
	page, err = w.UpdatePage("editor", page.ID, page.Title, page.Slug, &content, &kind)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}

	router := gin.New()
	router.GET("/pages/:id/revisions/:revisionId", GetPageRevisionHandler(w))

	req := httptest.NewRequest(http.MethodGet, "/pages/"+page.ID+"/revisions/"+rev.ID, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body RevisionSnapshotResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Revision == nil || body.Revision.ID != rev.ID {
		t.Fatalf("revision body = %#v", body.Revision)
	}
	if body.Content != content {
		t.Fatalf("content = %q, want %q", body.Content, content)
	}
}

func TestRestorePageRevisionHandlerRestoresLivePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	sectionKind := tree.NodeKindSection
	docs, err := w.CreatePage("editor", nil, "Docs", "docs", &sectionKind)
	if err != nil {
		t.Fatalf("CreatePage(docs) failed: %v", err)
	}
	archive, err := w.CreatePage("editor", nil, "Archive", "archive", &sectionKind)
	if err != nil {
		t.Fatalf("CreatePage(archive) failed: %v", err)
	}
	pageKind := tree.NodeKindPage
	page, err := w.CreatePage("editor", &docs.ID, "Original", "original", &pageKind)
	if err != nil {
		t.Fatalf("CreatePage(page) failed: %v", err)
	}

	content := "historical content"
	page, err = w.UpdatePage("editor", page.ID, "Original", "original", &content, &pageKind)
	if err != nil {
		t.Fatalf("UpdatePage(original) failed: %v", err)
	}

	oldAssetFile, err := os.CreateTemp(t.TempDir(), "restore-old-asset-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp(old asset) failed: %v", err)
	}
	defer func() { _ = oldAssetFile.Close() }()
	if _, err := oldAssetFile.WriteString("old-asset"); err != nil {
		t.Fatalf("WriteString(old asset) failed: %v", err)
	}
	if _, err := oldAssetFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek(old asset) failed: %v", err)
	}
	if _, err := w.UploadAsset("editor", page.ID, oldAssetFile, "old.txt"); err != nil {
		t.Fatalf("UploadAsset(old) failed: %v", err)
	}

	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}

	changedContent := "current content"
	page, err = w.UpdatePage("editor", page.ID, "Changed", "changed", &changedContent, &pageKind)
	if err != nil {
		t.Fatalf("UpdatePage(changed) failed: %v", err)
	}
	if err := w.MovePage("editor", page.ID, archive.ID); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	assetDir := filepath.Join(w.GetAssetService().GetAssetsDir(), page.ID)
	if err := os.Remove(filepath.Join(assetDir, "old.txt")); err != nil {
		t.Fatalf("Remove(old asset) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "new.txt"), []byte("new-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile(new asset) failed: %v", err)
	}

	router := gin.New()
	router.POST("/pages/:id/revisions/:revisionId/restore", func(c *gin.Context) {
		c.Set("user", &coreauth.User{ID: "editor", Role: coreauth.RoleEditor})
		RestorePageRevisionHandler(w)(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/pages/"+page.ID+"/revisions/"+rev.ID+"/restore", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var body Page
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Path != "archive/changed" {
		t.Fatalf("path = %q", body.Path)
	}
	if body.Content != content {
		t.Fatalf("content = %q, want %q", body.Content, content)
	}
	if _, err := os.Stat(filepath.Join(assetDir, "old.txt")); err != nil {
		t.Fatalf("expected old asset to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(assetDir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected new asset to be removed, got %v", err)
	}
}

func TestGetPageRevisionHandlerReturnsStructuredArtifactError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	kind := tree.NodeKindPage
	page, err := w.CreatePage("editor", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "historical content"
	page, err = w.UpdatePage("editor", page.ID, page.Title, page.Slug, &content, &kind)
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}
	rev, err := w.GetLatestRevision(page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}
	if err := os.Remove(filepath.Join(w.GetStorageDir(), ".leafwiki", "blobs", "content", "sha256", rev.ContentHash[:2], rev.ContentHash)); err != nil {
		t.Fatalf("Remove content blob failed: %v", err)
	}

	router := gin.New()
	router.GET("/pages/:id/revisions/:revisionId", GetPageRevisionHandler(w))

	req := httptest.NewRequest(http.MethodGet, "/pages/"+page.ID+"/revisions/"+rev.ID, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}

	var body RevisionErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error.Code != "revision_preview_content_unavailable" {
		t.Fatalf("error.code = %q", body.Error.Code)
	}
}

func TestComparePageRevisionsHandlerReturnsComparison(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := createRevisionTestWiki(t)
	t.Cleanup(func() { _ = w.Close() })

	kind := tree.NodeKindPage
	page, err := w.CreatePage("editor", nil, "Page", "page", &kind)
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "one"
	page, err = w.UpdatePage("editor", page.ID, page.Title, page.Slug, &content, &kind)
	if err != nil {
		t.Fatalf("UpdatePage first failed: %v", err)
	}
	baseRev, err := w.GetLatestRevision(page.ID)
	if err != nil || baseRev == nil {
		t.Fatalf("GetLatestRevision base failed: %#v %v", baseRev, err)
	}

	content = "two"
	page, err = w.UpdatePage("editor", page.ID, page.Title, page.Slug, &content, &kind)
	if err != nil {
		t.Fatalf("UpdatePage second failed: %v", err)
	}
	targetRev, err := w.GetLatestRevision(page.ID)
	if err != nil || targetRev == nil {
		t.Fatalf("GetLatestRevision target failed: %#v %v", targetRev, err)
	}

	router := gin.New()
	router.GET("/pages/:id/revisions/compare", ComparePageRevisionsHandler(w))

	req := httptest.NewRequest(http.MethodGet, "/pages/"+page.ID+"/revisions/compare?base="+baseRev.ID+"&target="+targetRev.ID, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	var body RevisionComparisonResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Base == nil || body.Target == nil {
		t.Fatalf("comparison body = %#v", body)
	}
	if !body.ContentChanged {
		t.Fatalf("expected contentChanged=true")
	}
	if body.Base.Content != "one" || body.Target.Content != "two" {
		t.Fatalf("unexpected compare content: base=%q target=%q", body.Base.Content, body.Target.Content)
	}
}
