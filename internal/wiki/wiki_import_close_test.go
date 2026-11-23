package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	coreimporter "github.com/perber/wiki/internal/importer"
)

// Regression: Close used to close the link, tag and property stores while a
// background import kept writing to them, which crashed the process with a
// nil *sql.DB panic (seen on slow Windows CI runners). Close must stop the
// import first and leave it to the next start, which must not leave it stuck
// in "running".
func TestWiki_Close_StopsRunningImportBeforeClosingStores(t *testing.T) {
	const pages = 200
	src := t.TempDir()
	for i := range pages {
		body := fmt.Sprintf("# Page %d\n\nSee [previous](page-%d.md).\n", i, i-1)
		if err := os.WriteFile(filepath.Join(src, fmt.Sprintf("page-%d.md", i)), []byte(body), 0o644); err != nil {
			t.Fatalf("write source page: %v", err)
		}
	}

	opts := &WikiOptions{
		StorageDir:          t.TempDir(),
		AdminPassword:       "adminpassword",
		JWTSecret:           "secretkey",
		AccessTokenTimeout:  15 * time.Minute,
		RefreshTokenTimeout: 7 * 24 * time.Hour,
	}
	w, err := NewWiki(opts)
	if err != nil {
		t.Fatalf("NewWiki: %v", err)
	}

	if _, err := w.importer.CreateImportPlanFromFolder(src, "imported"); err != nil {
		t.Fatalf("CreateImportPlanFromFolder: %v", err)
	}
	if _, started, err := w.importer.StartCurrentPlanExecution("system"); err != nil || !started {
		t.Fatalf("StartCurrentPlanExecution started=%v err=%v", started, err)
	}
	waitForImport(t, w.importer, func(s *coreimporter.CurrentPlanState) bool { return s.ProcessedItems > 0 })

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	interrupted, err := w.importer.GetCurrentPlan()
	if err != nil {
		t.Fatalf("GetCurrentPlan after Close: %v", err)
	}
	if interrupted.ExecutionStatus != coreimporter.ExecutionStatusRunning {
		t.Fatalf("expected the import to be interrupted by Close, got status %q", interrupted.ExecutionStatus)
	}
	if interrupted.ProcessedItems >= pages {
		t.Fatalf("expected Close to stop the import before its end, got %d/%d", interrupted.ProcessedItems, pages)
	}

	reopened, err := NewWiki(opts)
	if err != nil {
		t.Fatalf("reopen NewWiki: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	final := waitForImport(t, reopened.importer, func(s *coreimporter.CurrentPlanState) bool {
		return s.ExecutionStatus != coreimporter.ExecutionStatusRunning
	})
	// Completed once a resume works across restarts; until then the root
	// node's metadata changes the tree hash on every load and the resumed
	// import fails as stale. Either way it must reach a final state.
	if final.ExecutionStatus != coreimporter.ExecutionStatusCompleted && final.ExecutionStatus != coreimporter.ExecutionStatusFailed {
		t.Fatalf("expected the resumed import to finish, got %q", final.ExecutionStatus)
	}
}

func waitForImport(t *testing.T, svc *coreimporter.ImporterService, done func(*coreimporter.CurrentPlanState) bool) *coreimporter.CurrentPlanState {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if state, err := svc.GetCurrentPlan(); err == nil && state != nil && done(state) {
			return state
		}
		time.Sleep(5 * time.Millisecond)
	}
	state, err := svc.GetCurrentPlan()
	t.Fatalf("timed out waiting for import state, state=%#v err=%v", state, err)
	return nil
}
