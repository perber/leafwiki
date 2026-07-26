// White-box test: package wikirestore so we can register the real handler methods.
package wikirestore

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/branding"
	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/restore"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestManager wires a real restore.Manager (real snapshot manager, real
// AuthService, real BrandingService) against fresh temp dirs — the
// Manager-level behavior itself is covered exhaustively in
// internal/restore's own tests; this package only needs to verify the HTTP
// wiring (routing, status codes, response shapes) on top of it.
func newTestManager(t *testing.T) *restore.Manager {
	return newTestManagerWithMaxUploadSize(t, 0)
}

// newTestManagerWithMaxUploadSize is newTestManager with control over
// Config.MaxUploadSizeBytes, used to exercise the 413-too-large path with a
// small configured limit instead of the real (500 MiB) default.
func newTestManagerWithMaxUploadSize(t *testing.T, maxUploadSizeBytes int64) *restore.Manager {
	t.Helper()
	base := t.TempDir()

	rootDir := filepath.Join(base, "root")
	test_utils.WriteFile(t, rootDir, "page.md", "# Hello\n")

	userStore, err := coreauth.NewUserStore(base)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessionStore, err := coreauth.NewSessionStore(base)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := coreauth.NewAuthService(coreauth.NewUserService(userStore), sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })

	brandingService, err := branding.NewBrandingService(base)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	snapshotManager := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  filepath.Join(base, "backups"),
		RootDir:     rootDir,
		UsersDBPath: filepath.Join(base, "users.db"),
		WikiVersion: "v0.0.0-test",
	})

	return restore.NewManager(restore.Config{
		SnapshotManager:    snapshotManager,
		DataDir:            base,
		WikiVersion:        "v0.0.0-test",
		WriteGate:          restore.NewWriteGate(),
		AuthService:        authService,
		BrandingService:    brandingService,
		TriggerResync:      func() {},
		MaxUploadSizeBytes: maxUploadSizeBytes,
	})
}

func newTestRouter(routes *Routes) *gin.Engine {
	r := gin.New()
	r.POST("/restore/upload", routes.handleTriggerUpload)
	r.POST("/restore/:id", routes.handleTrigger)
	r.GET("/restore/status", routes.handleStatus)
	r.POST("/restore/self-restart", routes.handleSelfRestart)
	return r
}

// buildFixtureZipPath builds a real, minimal-but-valid backup ZIP (the shape
// extractAndValidate requires: backup-meta.json + users.db) from a fresh
// source layout, independent of whatever dataDir a given test's Manager is
// wired against, and returns the path to the resulting ZIP file.
func buildFixtureZipPath(t *testing.T) string {
	t.Helper()
	src := t.TempDir()

	rootDir := filepath.Join(src, "root")
	test_utils.WriteFile(t, rootDir, "welcome.md", "# Uploaded backup content\n")

	userStore, err := coreauth.NewUserStore(src)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	if _, err := coreauth.NewUserService(userStore).CreateUser("upload-admin", "upload-admin@example.com", "upload-password-123", "admin"); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if err := userStore.Close(); err != nil {
		t.Fatalf("failed to close user store: %v", err)
	}

	snapshotManager := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  filepath.Join(src, "backups"),
		RootDir:     rootDir,
		UsersDBPath: filepath.Join(src, "users.db"),
		WikiVersion: "v0.0.0-test",
	})
	if err := snapshotManager.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotManager.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	zipPath, err := snapshotManager.SnapshotZipPath(entries[0].ID)
	if err != nil {
		t.Fatalf("SnapshotZipPath failed: %v", err)
	}
	return zipPath
}

// newMultipartUploadRequest builds a POST request whose body is a multipart
// form carrying zipPath's contents under the "file" field, mirroring what the
// frontend's FormData-based upload sends.
func newMultipartUploadRequest(t *testing.T, url, zipPath string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filepath.Base(zipPath))
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	src, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = src.Close() }()
	if _, err := io.Copy(fw, src); err != nil {
		t.Fatalf("failed to copy fixture zip into multipart body: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestHandleTrigger_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/restore/snapshot-20260101-000000", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleStatus_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/restore/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleTrigger_UnknownSnapshotID_AsyncJobReportsError(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/restore/snapshot-does-not-exist", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (validation runs async), got %d: %s", w.Code, w.Body.String())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m.Status().Done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if m.Status().Error == "" {
		t.Error("expected the async job to report an error for an unknown snapshot id")
	}
}

func TestHandleStatus_ReturnsJobStatus(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/restore/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var status restore.JobStatus
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if status.Running {
		t.Error("expected a fresh manager to report running=false")
	}
}

func TestHandleSelfRestart_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/restore/self-restart", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleTriggerUpload_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	zipPath := buildFixtureZipPath(t)
	w := httptest.NewRecorder()
	req := newMultipartUploadRequest(t, "/restore/upload", zipPath)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleTriggerUpload_HappyPath_AsyncJobCompletes(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	zipPath := buildFixtureZipPath(t)
	w := httptest.NewRecorder()
	req := newMultipartUploadRequest(t, "/restore/upload", zipPath)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m.Status().Done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	status := m.Status()
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}
}

func TestHandleTriggerUpload_ExceedsConfiguredMaxSize(t *testing.T) {
	m := newTestManagerWithMaxUploadSize(t, 8) // smaller than any real fixture zip
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	zipPath := buildFixtureZipPath(t)
	w := httptest.NewRecorder()
	req := newMultipartUploadRequest(t, "/restore/upload", zipPath)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
	var resp RestoreErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Error.Code != ErrCodeRestoreUploadTooLarge {
		t.Errorf("expected code %q, got %q", ErrCodeRestoreUploadTooLarge, resp.Error.Code)
	}
}

// TestHandleTriggerUpload_MalformedMultipartBody_DistinctFromTooLarge is the
// regression test for a review finding: every ParseMultipartForm error used
// to be reported as ErrCodeRestoreUploadTooLarge (413), even when the body
// wasn't too large at all — e.g. malformed multipart framing from a broken
// client or a mangled proxy. That masked the real cause. A malformed-but-not-
// oversized body should surface as ErrCodeRestoreUploadInvalid (400) instead.
func TestHandleTriggerUpload_MalformedMultipartBody_DistinctFromTooLarge(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	req, _ := http.NewRequest(http.MethodPost, "/restore/upload", bytes.NewBufferString("not a multipart body"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=broken")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var resp RestoreErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Error.Code != ErrCodeRestoreUploadInvalid {
		t.Errorf("expected code %q, got %q", ErrCodeRestoreUploadInvalid, resp.Error.Code)
	}
}

func TestHandleTriggerUpload_MissingFile(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close empty multipart writer: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/restore/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	var resp RestoreErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Error.Code != ErrCodeRestoreMissingFile {
		t.Errorf("expected code %q, got %q", ErrCodeRestoreMissingFile, resp.Error.Code)
	}
}

// TestHandleSelfRestart_RejectedWithoutNeedsIntervention is the only
// self-restart HTTP test: it deliberately never reaches restore.SelfRestart()
// (which would syscall.Exec / os.Exit the *test process itself*). A fresh
// Manager never has NeedsIntervention set, so this only exercises the guard.
func TestHandleSelfRestart_RejectedWithoutNeedsIntervention(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/restore/self-restart", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 when the job hasn't reported NeedsIntervention, got %d", w.Code)
	}
}
