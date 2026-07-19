// White-box test: package wikisnapshot so we can register the real handler methods.
package wikisnapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestManager(t *testing.T) *snapshotSvc.Manager {
	t.Helper()
	base := t.TempDir()

	rootDir := filepath.Join(base, "root")
	assetsDir := filepath.Join(base, "assets")
	backupsDir := filepath.Join(base, "backups")
	usersDBPath := filepath.Join(base, "users.db")

	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Hello\n"), 0o644); err != nil {
		t.Fatalf("failed to write page: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	db, err := sql.Open("sqlite", usersDBPath)
	if err != nil {
		t.Fatalf("failed to open users db: %v", err)
	}
	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("failed to close users db: %v", err)
	}

	return snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  backupsDir,
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		UsersDBPath: usersDBPath,
		WikiVersion: "v0.0.0-test",
	})
}

// waitForSnapshotDone blocks until the manager finishes any in-flight run
// (e.g. a scheduler's pre-seeded startup run), so tests can deterministically
// trigger a fresh run afterward instead of racing the startup one.
func waitForSnapshotDone(t *testing.T, m *snapshotSvc.Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !m.Status().IsRunning && m.Status().LastSnapshotAt != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for snapshot to finish")
}

// newTestRouter wires the real handler methods on a bare Gin engine, skipping
// the auth/CSRF middleware chain (exercised elsewhere) to focus on handler behavior.
func newTestRouter(routes *Routes) *gin.Engine {
	r := gin.New()
	r.GET("/snapshot/status", routes.handleStatus)
	r.GET("/snapshot", routes.handleList)
	r.POST("/snapshot", routes.handleTrigger)
	r.GET("/snapshot/:id/download", routes.handleDownload)
	r.DELETE("/snapshot/:id", routes.handleDelete)
	return r
}

func TestHandleStatus_Disabled(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/status", nil)
	router.ServeHTTP(w, req)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["enabled"] != false {
		t.Errorf("expected enabled=false, got %v", body)
	}
}

func TestHandleStatus_Enabled(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m, retentionCount: 10}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/status", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", body)
	}
	if body["retentionCount"].(float64) != 10 {
		t.Errorf("expected retentionCount=10, got %v", body["retentionCount"])
	}
}

func TestHandleList_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleDownload_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/snapshot-20260101-000000/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleDelete_NotEnabledWhenManagerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/snapshot/snapshot-20260101-000000", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleTrigger_NotEnabledWhenSchedulerNil(t *testing.T) {
	router := newTestRouter(&Routes{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/snapshot", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleTrigger_Returns202(t *testing.T) {
	m := newTestManager(t)
	scheduler := snapshotSvc.NewScheduler(m)
	defer scheduler.Stop()

	// NewScheduler pre-seeds an immediate startup run; wait for it to drain
	// before triggering, otherwise TriggerNow's buffered signal slot is
	// still occupied and the trigger is correctly rejected as a dropped
	// duplicate rather than 202.
	waitForSnapshotDone(t, m)

	routes := &Routes{manager: m, scheduler: scheduler}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/snapshot", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
}

func TestHandleList_ReturnsCreatedSnapshots(t *testing.T) {
	m := newTestManager(t)
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Snapshots []snapshotSvc.SnapshotEntry `json:"snapshots"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(body.Snapshots))
	}
}

func TestHandleDownload_ReturnsZip(t *testing.T) {
	m := newTestManager(t)
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	entries, err := m.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 snapshot, got %v (err=%v)", entries, err)
	}

	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/"+entries[0].ID+"/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("expected Content-Type application/zip, got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("expected Cache-Control no-store (snapshot contains users.db), got %q", cc)
	}
	disposition := w.Header().Get("Content-Disposition")
	if disposition == "" {
		t.Error("expected Content-Disposition header to be set")
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty zip body")
	}
}

func TestHandleDownload_NotFound(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/snapshot-20260101-000000/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleDownload_InvalidID(t *testing.T) {
	// Single-segment IDs that fail validateSnapshotID's prefix/".." checks but
	// don't contain "/" (a "/" in the id wouldn't even route to this handler —
	// gin matches path segments, so that case 404s at the router, not here).
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/snapshot/not-a-snapshot/download", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleDelete_RemovesSnapshot(t *testing.T) {
	m := newTestManager(t)
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	entries, err := m.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 snapshot, got %v (err=%v)", entries, err)
	}

	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/snapshot/"+entries[0].ID, nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	remaining, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(remaining))
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	m := newTestManager(t)
	routes := &Routes{manager: m}
	router := newTestRouter(routes)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/snapshot/snapshot-20260101-000000", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
