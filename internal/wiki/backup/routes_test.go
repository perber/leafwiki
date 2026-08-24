package wikibackup

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	backupSvc "github.com/perber/wiki/internal/backup"
)

func newTestGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/backup/pull", nil)
	return c, rec
}

// TestHandleTriggerPull_NotEnabled verifies that the pull endpoint reports
// 503 when git backup is not configured (repo is nil), matching the other
// backup handlers' not-enabled behavior.
func TestHandleTriggerPull_NotEnabled(t *testing.T) {
	routes := &Routes{}
	c, rec := newTestGinContext()

	routes.handleTriggerPull(c)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

// TestHandleTriggerPull_Success verifies that the pull endpoint returns 200
// when the repository pull succeeds (here: a no-op pull, no remote configured).
func TestHandleTriggerPull_Success(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	repo, err := backupSvc.Init(backupSvc.Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	routes := &Routes{repo: repo}
	c, rec := newTestGinContext()

	routes.handleTriggerPull(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}
