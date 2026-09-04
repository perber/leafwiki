package restore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func TestRestoreOffline_SwapsFilesWithoutAnyLiveMachinery(t *testing.T) {
	zipPath := buildFixtureSnapshot(t, "v1.0.0")

	dataDir := t.TempDir()
	test_utils.WriteFile(t, dataDir, "root/live-page.md", "# Live content before restore\n")
	createTestUsersDB(t, filepath.Join(dataDir, "users.db"), "live-admin@example.com")

	if err := RestoreOffline(dataDir, zipPath); err != nil {
		t.Fatalf("RestoreOffline failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "root", "welcome.md")); err != nil {
		t.Errorf("expected restored root/welcome.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "live-page.md")); !os.IsNotExist(err) {
		t.Errorf("expected pre-restore live content to be gone, got err=%v", err)
	}
	if hasPreRestoreEntries(t, dataDir) {
		t.Error("expected .pre-restore-* backup entries to be cleaned up by CommitAll")
	}
	// No leftover staging directory either.
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == "" && e.IsDir() && e.Name()[0] == '.' {
			t.Errorf("expected no leftover staging directory, found %s", e.Name())
		}
	}
}

// TestRestoreOffline_LeavesWALSidecarsUntouchedForDatabaseNotInSnapshot is
// the regression test for a real bug found in review: the WAL-sidecar
// cleanup that runs before SwapAll used to remove every known database's
// stale -wal/-shm files unconditionally, even for a database SwapAll would
// then leave untouched because the snapshot never captured it (see
// newSwapper's doc comment) — buildFixtureSnapshot's snapshot has no
// favorites.db (FavoritesDBPath unset). Deleting that WAL sidecar first
// would discard any committed-but-uncheckpointed data it holds, for a
// database the restore was never even going to touch.
func TestRestoreOffline_LeavesWALSidecarsUntouchedForDatabaseNotInSnapshot(t *testing.T) {
	zipPath := buildFixtureSnapshot(t, "v1.0.0")

	dataDir := t.TempDir()
	createTestUsersDB(t, filepath.Join(dataDir, "users.db"), "live-admin@example.com")
	test_utils.WriteFile(t, dataDir, "favorites.db", "live favorites content")
	test_utils.WriteFile(t, dataDir, "favorites.db-wal", "live favorites WAL content, must survive")

	if err := RestoreOffline(dataDir, zipPath); err != nil {
		t.Fatalf("RestoreOffline failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "favorites.db-wal"))
	if err != nil {
		t.Fatalf("expected favorites.db-wal to survive (not part of the restored snapshot): %v", err)
	}
	if string(got) != "live favorites WAL content, must survive" {
		t.Errorf("favorites.db-wal content changed: got %q", got)
	}
}

func TestRestoreOffline_InvalidZip_LeavesDataDirUntouched(t *testing.T) {
	dataDir := t.TempDir()
	test_utils.WriteFile(t, dataDir, "root/live-page.md", "# Live content\n")

	badZip := filepath.Join(t.TempDir(), "not-a-zip.zip")
	if err := os.WriteFile(badZip, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RestoreOffline(dataDir, badZip); err == nil {
		t.Fatal("expected an error for an invalid zip")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "live-page.md")); err != nil {
		t.Errorf("expected live content to be untouched after a failed validation: %v", err)
	}
}
