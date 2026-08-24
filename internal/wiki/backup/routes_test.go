package wikibackup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	backupSvc "github.com/perber/wiki/internal/backup"
	gossh "golang.org/x/crypto/ssh"
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

// testSSHKeyPEM returns a throwaway key so buildAuth (which requires a
// parseable key even for the file:// remotes used below) doesn't error out.
func testSSHKeyPEM(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	block, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("MarshalPrivateKey: %v", err)
	}
	return string(pem.EncodeToMemory(block))
}

// TestHandleTriggerPull_ErrorHasEmptyTemplate verifies that a pull failure is
// reported with an empty Template field. respondWithBackupStatusError's
// template takes priority over Message in the frontend's mapApiError, and
// "backup internal error" isn't a registered i18n key — a non-empty template
// here would silently discard the actual (and, for conflicts, actionable)
// error detail in err.Error() and show the literal template string instead.
func TestHandleTriggerPull_ErrorHasEmptyTemplate(t *testing.T) {
	bareDir := t.TempDir()
	if _, err := gogit.PlainInit(bareDir, true); err != nil {
		t.Fatalf("PlainInit bare remote: %v", err)
	}

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}

	repo, err := backupSvc.Init(backupSvc.Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM(t),
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("initial RunBackup failed: %v", err)
	}

	// External client changes the same file on the remote...
	cloneDir := t.TempDir()
	cloned, err := gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
		URL:           "file://" + bareDir,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
	})
	if err != nil {
		t.Fatalf("PlainClone: %v", err)
	}
	wt, err := cloned.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, "page.md"), []byte("version B from remote\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := wt.Add("page.md"); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := wt.Commit("external commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "External", Email: "ext@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	remote, err := cloned.Remote("origin")
	if err != nil {
		t.Fatalf("Remote: %v", err)
	}
	if err := remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"},
	}); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// ...while the local wiki has an uncommitted change to the same file.
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile local change: %v", err)
	}

	routes := &Routes{repo: repo}
	c, rec := newTestGinContext()

	routes.handleTriggerPull(c)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}

	var body BackupErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response body: %v", err)
	}
	if body.Error.Template != "" {
		t.Errorf("expected empty template so the frontend surfaces the real message, got %q", body.Error.Template)
	}
	if !strings.Contains(body.Error.Message, "conflict") {
		t.Errorf("expected message to contain the conflict detail, got %q", body.Error.Message)
	}
}
