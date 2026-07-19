package restore

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
)

func TestExtractAndValidate_HappyPath(t *testing.T) {
	zipPath := buildFixtureSnapshot(t, "v1.2.3")
	dataDir := t.TempDir()

	stagingDir, meta, err := extractAndValidate(zipPath, dataDir)
	if err != nil {
		t.Fatalf("extractAndValidate failed: %v", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	if meta.Version != "v1.2.3" {
		t.Errorf("meta.Version = %q, want v1.2.3", meta.Version)
	}
	for _, want := range []string{"root/welcome.md", "assets/logo.png", "branding/logo.png", "branding.json", "schema.json", "users.db"} {
		if _, err := os.Stat(filepath.Join(stagingDir, want)); err != nil {
			t.Errorf("expected staged %s: %v", want, err)
		}
	}

	rel, err := filepath.Rel(dataDir, stagingDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Errorf("expected staging dir inside dataDir (so the later rename stays on one filesystem), got %s", stagingDir)
	}
}

func TestExtractAndValidate_MissingUsersDB(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"backup-meta.json": `{"id":"x","version":"v1"}`,
	})

	if _, _, err := extractAndValidate(zipPath, t.TempDir()); err == nil {
		t.Fatal("expected error for a zip missing users.db")
	}
}

func TestExtractAndValidate_CorruptZip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "corrupt.zip")
	if err := os.WriteFile(zipPath, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := extractAndValidate(zipPath, t.TempDir()); err == nil {
		t.Fatal("expected error for a corrupt zip")
	}
}

func TestExtractAndValidate_RejectsZipSlip(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"backup-meta.json": `{"id":"x","version":"v1"}`,
		"users.db":         "irrelevant here, rejected before it's ever opened",
		"../../evil.txt":   "pwned",
	})

	if _, _, err := extractAndValidate(zipPath, t.TempDir()); err == nil {
		t.Fatal("expected error for a zip entry escaping the staging dir")
	}
}

func TestExtractAndValidate_RejectsCorruptUsersDB(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"backup-meta.json": `{"id":"x","version":"v1"}`,
		"users.db":         "this is not a sqlite database",
	})

	if _, _, err := extractAndValidate(zipPath, t.TempDir()); err == nil {
		t.Fatal("expected error for a users.db that fails the sanity query")
	}
}

func TestSwapper_SwapAll_ReplacesLiveContentAndKeepsPreRestoreCopyUntilCommit(t *testing.T) {
	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	dataDir := t.TempDir()

	test_utils.WriteFile(t, dataDir, "root/live-only-page.md", "# Live content, not in the snapshot\n")

	stagingDir, _, err := extractAndValidate(zipPath, dataDir)
	if err != nil {
		t.Fatalf("extractAndValidate failed: %v", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	sw := newSwapper(dataDir, stagingDir)
	if err := sw.SwapAll(); err != nil {
		t.Fatalf("SwapAll failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "root", "welcome.md")); err != nil {
		t.Errorf("expected restored root/welcome.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "live-only-page.md")); !os.IsNotExist(err) {
		t.Errorf("expected pre-swap live content to be moved aside, got err=%v", err)
	}
	if !hasPreRestoreEntries(t, dataDir) {
		t.Error("expected .pre-restore-* backup entries to exist after SwapAll (before CommitAll)")
	}

	sw.CommitAll()
	if hasPreRestoreEntries(t, dataDir) {
		t.Error("expected .pre-restore-* entries to be gone after CommitAll")
	}
}

func TestSwapper_SwapAll_LeavesItemUntouchedWhenNotCapturedBySnapshot(t *testing.T) {
	// A minimal snapshot (no branding.json/schema.json/branding assets).
	src := t.TempDir()
	rootDir := filepath.Join(src, "root")
	usersDBPath := filepath.Join(src, "users.db")
	test_utils.WriteFile(t, rootDir, "page.md", "# hi\n")
	createTestUsersDB(t, usersDBPath, "a@example.com")

	m := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  filepath.Join(src, "backups"),
		RootDir:     rootDir,
		UsersDBPath: usersDBPath,
		WikiVersion: "v1",
	})
	entries, id := mustRunOnce(t, m)
	_ = entries
	zipPath, err := m.SnapshotZipPath(id)
	if err != nil {
		t.Fatalf("SnapshotZipPath failed: %v", err)
	}

	dataDir := t.TempDir()
	const liveBranding = `{"siteName":"Existing Live Branding"}`
	test_utils.WriteFile(t, dataDir, "branding.json", liveBranding)

	stagingDir, _, err := extractAndValidate(zipPath, dataDir)
	if err != nil {
		t.Fatalf("extractAndValidate failed: %v", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	sw := newSwapper(dataDir, stagingDir)
	if err := sw.SwapAll(); err != nil {
		t.Fatalf("SwapAll failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "branding.json"))
	if err != nil {
		t.Fatalf("expected branding.json to still exist: %v", err)
	}
	if string(got) != liveBranding {
		t.Errorf("expected branding.json to be left untouched (not captured by this snapshot), got %q", got)
	}
}

func TestSwapper_RollbackAll_RestoresPreRestoreCopy(t *testing.T) {
	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	dataDir := t.TempDir()
	test_utils.WriteFile(t, dataDir, "root/original.md", "# original live content\n")

	stagingDir, _, err := extractAndValidate(zipPath, dataDir)
	if err != nil {
		t.Fatalf("extractAndValidate failed: %v", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	sw := newSwapper(dataDir, stagingDir)
	if err := sw.SwapAll(); err != nil {
		t.Fatalf("SwapAll failed: %v", err)
	}

	if err := sw.RollbackAll(); err != nil {
		t.Fatalf("RollbackAll failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "root", "original.md")); err != nil {
		t.Errorf("expected original live content restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "welcome.md")); !os.IsNotExist(err) {
		t.Errorf("expected restored snapshot content to be gone after rollback, got err=%v", err)
	}
	if hasPreRestoreEntries(t, dataDir) {
		t.Error("expected .pre-restore-* entries to be consumed by RollbackAll")
	}
}

func hasPreRestoreEntries(t *testing.T, dataDir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".pre-restore-") {
			return true
		}
	}
	return false
}

func mustRunOnce(t *testing.T, m *snapshotSvc.Manager) ([]snapshotSvc.SnapshotEntry, string) {
	t.Helper()
	if err := m.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}
	entries, err := m.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 snapshot, got %v (err=%v)", entries, err)
	}
	return entries, entries[0].ID
}

// writeRawZip writes a zip file with exactly the given entries (name ->
// content) for tests that need to control the zip's contents precisely
// (missing/corrupt entries) rather than going through a real snapshot build.
func writeRawZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "fixture.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		zw, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := zw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close zip file: %v", err)
	}
	return zipPath
}
