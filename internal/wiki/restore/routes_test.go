// White-box test: package wikirestore so we can register the real handler methods.
package wikirestore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		SnapshotManager: snapshotManager,
		DataDir:         base,
		WikiVersion:     "v0.0.0-test",
		WriteGate:       restore.NewWriteGate(),
		AuthService:     authService,
		BrandingService: brandingService,
		TriggerResync:   func() {},
	})
}

func newTestRouter(routes *Routes) *gin.Engine {
	r := gin.New()
	r.POST("/restore/:id", routes.handleTrigger)
	r.GET("/restore/status", routes.handleStatus)
	r.POST("/restore/self-restart", routes.handleSelfRestart)
	return r
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
