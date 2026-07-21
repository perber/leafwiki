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
