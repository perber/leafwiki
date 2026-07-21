package snapshot

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func newTestConfig(t *testing.T) Config {
	t.Helper()
	base := t.TempDir()

	rootDir := filepath.Join(base, "root")
	assetsDir := filepath.Join(base, "assets")
	backupsDir := filepath.Join(base, "backups")
	usersDBPath := filepath.Join(base, "users.db")
	brandingConfigFile := test_utils.WriteFile(t, base, "branding.json", `{"siteName":"Test Site"}`)

	test_utils.WriteFile(t, rootDir, "page.md", "# Hello\n")
	test_utils.WriteFile(t, assetsDir, "image.png", "fake-image-bytes")

	createTestUsersDB(t, usersDBPath)

	return Config{
		BackupsDir:         backupsDir,
		RootDir:            rootDir,
		AssetsDir:          assetsDir,
		BrandingConfigFile: brandingConfigFile,
		UsersDBPath:        usersDBPath,
		WikiVersion:        "v0.0.0-test",
	}
}

func createTestUsersDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("failed to open users db: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)

	if _, err := db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT)"); err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}
	if _, err := db.Exec("INSERT INTO users (email) VALUES (?)", "admin@example.com"); err != nil {
		t.Fatalf("failed to seed users table: %v", err)
	}
}

func TestCreateSnapshot_CreatesZip(t *testing.T) {
	cfg := newTestConfig(t)

	id, err := createSnapshot(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createSnapshot failed: %v", err)
	}

	zipPath := filepath.Join(cfg.BackupsDir, id+".zip")
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("expected zip file at %s: %v", zipPath, err)
	}

	sidecarPath := filepath.Join(cfg.BackupsDir, id+".json")
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("expected sidecar file at %s: %v", sidecarPath, err)
	}

	var entry SnapshotEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("failed to parse sidecar json: %v", err)
	}
	if entry.ID != id {
		t.Errorf("sidecar id = %q, want %q", entry.ID, id)
	}
	if entry.SizeBytes <= 0 {
		t.Errorf("sidecar sizeBytes = %d, want > 0", entry.SizeBytes)
	}
	if entry.CreatedAt.IsZero() {
		t.Errorf("sidecar createdAt is zero")
	}
}

func TestCreateSnapshot_ContainsExpectedFiles(t *testing.T) {
	cfg := newTestConfig(t)

	id, err := createSnapshot(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createSnapshot failed: %v", err)
	}

	zipPath := filepath.Join(cfg.BackupsDir, id+".zip")
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(r.Close, t)

	got := map[string]bool{}
	for _, f := range r.File {
		got[f.Name] = true
	}

	for _, want := range []string{"root/page.md", "assets/image.png", "users.db", "branding.json", "backup-meta.json"} {
		if !got[want] {
			t.Errorf("zip missing expected entry %q; got entries: %v", want, got)
		}
	}
}

func TestCreateSnapshot_BrandingConfigFileIsOptional(t *testing.T) {
	// An instance that has never touched branding settings has no
	// branding.json on disk yet — createSnapshot must not fail over that
	// (matches addFileToZip's existing "missing optional source" behavior).
	cfg := newTestConfig(t)
	cfg.BrandingConfigFile = filepath.Join(t.TempDir(), "does-not-exist", "branding.json")

	id, err := createSnapshot(context.Background(), cfg)
	if err != nil {
		t.Fatalf("createSnapshot failed: %v", err)
	}

	zipPath := filepath.Join(cfg.BackupsDir, id+".zip")
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(r.Close, t)

	for _, f := range r.File {
		if f.Name == "branding.json" {
			t.Error("expected no branding.json entry when the source file doesn't exist")
		}
	}
}

func TestCreateSnapshot_SameSecondCallsDoNotCollide(t *testing.T) {
	cfg := newTestConfig(t)

	id1, err := createSnapshot(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first createSnapshot failed: %v", err)
	}
	id2, err := createSnapshot(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second createSnapshot failed: %v", err)
	}

	if id1 == id2 {
		t.Fatalf("expected distinct IDs for two createSnapshot calls, got %q twice", id1)
	}

	for _, id := range []string{id1, id2} {
		if _, err := os.Stat(filepath.Join(cfg.BackupsDir, id+".zip")); err != nil {
			t.Errorf("expected zip for %s to still exist: %v", id, err)
		}
		if _, err := os.Stat(filepath.Join(cfg.BackupsDir, id+".json")); err != nil {
			t.Errorf("expected sidecar for %s to still exist: %v", id, err)
		}
	}
}

func TestUniqueSnapshotID_ReturnsSameIDWhenNoCollision(t *testing.T) {
	backupsDir := t.TempDir()
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	id, err := uniqueSnapshotID(backupsDir, ts)
	if err != nil {
		t.Fatalf("uniqueSnapshotID failed: %v", err)
	}
	want := "snapshot-20260101-120000"
	if id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

func TestUniqueSnapshotID_AppendsSuffixOnCollision(t *testing.T) {
	backupsDir := t.TempDir()
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	base := "snapshot-20260101-120000"

	if err := os.WriteFile(filepath.Join(backupsDir, base+".zip"), []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	id, err := uniqueSnapshotID(backupsDir, ts)
	if err != nil {
		t.Fatalf("uniqueSnapshotID failed: %v", err)
	}
	want := base + "-2"
	if id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

func TestVacuumUsersDB_WaitsForBusyWriter(t *testing.T) {
	base := t.TempDir()
	srcPath := filepath.Join(base, "users.db")
	createTestUsersDB(t, srcPath)

	// _txlock=exclusive makes Begin() issue "BEGIN EXCLUSIVE", grabbing the
	// database lock immediately instead of lazily on first write.
	holderDB, err := sql.Open("sqlite", srcPath+"?_txlock=exclusive")
	if err != nil {
		t.Fatalf("failed to open holder connection: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(holderDB.Close, t)

	tx, err := holderDB.Begin()
	if err != nil {
		t.Fatalf("failed to begin exclusive transaction: %v", err)
	}

	committed := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := tx.Commit(); err != nil {
			t.Errorf("failed to commit holder transaction: %v", err)
		}
		close(committed)
	}()

	dstPath := filepath.Join(base, "users-copy.db")
	if err := vacuumUsersDB(context.Background(), srcPath, dstPath); err != nil {
		t.Fatalf("vacuumUsersDB failed while a writer briefly held the database: %v", err)
	}

	<-committed

	if _, err := os.Stat(dstPath); err != nil {
		t.Fatalf("expected vacuum output at %s: %v", dstPath, err)
	}
}

func TestManager_ErrAlreadyRunning(t *testing.T) {
	cfg := newTestConfig(t)
	m := NewManager(cfg)

	if err := m.TriggerSnapshot(); err != nil {
		t.Fatalf("first TriggerSnapshot failed: %v", err)
	}

	assertLocalizedErrorCode(t, m.TriggerSnapshot(), "snapshot_already_running")

	waitForSnapshotDone(t, m)
}

func waitForSnapshotDone(t *testing.T, m *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !m.Status().IsRunning {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("snapshot did not finish within deadline")
}

func TestManager_List(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir})

	older := SnapshotEntry{ID: "snapshot-20260101-000000", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), SizeBytes: 100}
	newer := SnapshotEntry{ID: "snapshot-20260201-000000", CreatedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), SizeBytes: 200}

	writeSidecarFixture(t, backupsDir, older)
	writeSidecarFixture(t, backupsDir, newer)

	entries, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(entries))
	}
	if entries[0].ID != newer.ID || entries[1].ID != older.ID {
		t.Errorf("List order = [%s, %s], want [%s, %s]", entries[0].ID, entries[1].ID, newer.ID, older.ID)
	}
}

func TestManager_List_EmptyWhenBackupsDirMissing(t *testing.T) {
	m := NewManager(Config{BackupsDir: filepath.Join(t.TempDir(), "does-not-exist")})

	entries, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List returned %d entries, want 0", len(entries))
	}
}

func writeSidecarFixture(t *testing.T, backupsDir string, entry SnapshotEntry) {
	t.Helper()
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal fixture: %v", err)
	}
	path := filepath.Join(backupsDir, entry.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
}

func TestManager_Delete_NotFound(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir})

	err := m.Delete("snapshot-20260101-000000")
	assertLocalizedErrorCode(t, err, "snapshot_not_found")
}

func TestManager_Delete_RemovesFiles(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir})

	entry := SnapshotEntry{ID: "snapshot-20260101-000000", CreatedAt: time.Now().UTC(), SizeBytes: 42}
	writeSidecarFixture(t, backupsDir, entry)
	zipPath := filepath.Join(backupsDir, entry.ID+".zip")
	if err := os.WriteFile(zipPath, []byte("fake-zip"), 0o644); err != nil {
		t.Fatalf("failed to write fixture zip: %v", err)
	}

	if err := m.Delete(entry.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Errorf("expected zip file to be removed")
	}
	if _, err := os.Stat(filepath.Join(backupsDir, entry.ID+".json")); !os.IsNotExist(err) {
		t.Errorf("expected sidecar file to be removed")
	}
}

func TestManager_RunOnce_ErrAlreadyRunning(t *testing.T) {
	cfg := newTestConfig(t)
	m := NewManager(cfg)

	if !m.status.TryStart() {
		t.Fatal("expected TryStart to succeed")
	}
	defer m.status.SetSuccess(time.Now().UTC(), "")

	assertLocalizedErrorCode(t, m.RunOnce(context.Background()), "snapshot_already_running")
}

func TestManager_PruneOldSnapshots_KeepsNewestOnly(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir, RetentionCount: 2})

	entries := []SnapshotEntry{
		{ID: "snapshot-20260101-000000", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), SizeBytes: 1},
		{ID: "snapshot-20260102-000000", CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), SizeBytes: 1},
		{ID: "snapshot-20260103-000000", CreatedAt: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), SizeBytes: 1},
	}
	for _, e := range entries {
		writeSidecarFixture(t, backupsDir, e)
	}

	if err := m.pruneOldSnapshots(); err != nil {
		t.Fatalf("pruneOldSnapshots failed: %v", err)
	}

	remaining, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining snapshots, got %d", len(remaining))
	}
	want := map[string]bool{"snapshot-20260103-000000": true, "snapshot-20260102-000000": true}
	for _, r := range remaining {
		if !want[r.ID] {
			t.Errorf("unexpected snapshot retained: %s", r.ID)
		}
	}
}

func TestManager_PruneOldSnapshots_UnlimitedWhenRetentionNotPositive(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir, RetentionCount: 0})

	writeSidecarFixture(t, backupsDir, SnapshotEntry{ID: "snapshot-20260101-000000", CreatedAt: time.Now().UTC(), SizeBytes: 1})
	writeSidecarFixture(t, backupsDir, SnapshotEntry{ID: "snapshot-20260102-000000", CreatedAt: time.Now().UTC(), SizeBytes: 1})

	if err := m.pruneOldSnapshots(); err != nil {
		t.Fatalf("pruneOldSnapshots failed: %v", err)
	}

	remaining, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Errorf("expected no pruning with RetentionCount=0, got %d remaining", len(remaining))
	}
}

func TestManager_RunOnce_PrunesAfterSuccess(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.RetentionCount = 2
	m := NewManager(cfg)

	for i := 0; i < 3; i++ {
		if err := m.RunOnce(context.Background()); err != nil {
			t.Fatalf("RunOnce #%d failed: %v", i, err)
		}
	}

	remaining, err := m.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected retention to keep 2 snapshots, got %d", len(remaining))
	}
}

func TestManager_Delete_InvalidID(t *testing.T) {
	backupsDir := t.TempDir()
	m := NewManager(Config{BackupsDir: backupsDir})

	for _, id := range []string{"../etc/passwd", "not-a-snapshot", "snapshot-../evil"} {
		assertLocalizedErrorCode(t, m.Delete(id), "snapshot_invalid_id")
	}
}

func assertLocalizedErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error, got %T (%v)", err, err)
	}
	if localized.Code != wantCode {
		t.Fatalf("localized error code = %q, want %q", localized.Code, wantCode)
	}
}
