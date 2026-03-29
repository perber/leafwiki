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
