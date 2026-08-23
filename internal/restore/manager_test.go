package restore

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/auth"
	coreshared "github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/favorites"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
	"github.com/perber/wiki/internal/usersettings"
)

// managerFixture wires a restore.Manager against a real live dataDir, a real
// AuthService/BrandingService pointed at that same dataDir, and a real
// snapshot to restore from — mirroring how cmd/leafwiki/main.go wires these
// together, so the test exercises the same integration points production does.
type managerFixture struct {
	manager     *Manager
	dataDir     string
	snapshotID  string
	authService *auth.AuthService
	branding    *branding.BrandingService
	resyncCalls int
}

func newManagerFixture(t *testing.T, wikiVersion string) *managerFixture {
	t.Helper()
	return newManagerFixtureWithBranding(t, wikiVersion, `{"siteName":"Snapshot Site"}`)
}

func newManagerFixtureWithBranding(t *testing.T, wikiVersion, brandingJSON string) *managerFixture {
	t.Helper()

	snapshotMgr, snapshotID := fixtureSnapshotWithBranding(t, wikiVersion, brandingJSON)

	dataDir := t.TempDir()
	// Seed different "live" content so a successful restore is observable.
	test_utils.WriteFile(t, dataDir, "root/live-page.md", "# Live content before restore\n")
	createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")

	sessionStore, err := auth.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)

	brandingService, err := branding.NewBrandingService(dataDir)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	f := &managerFixture{dataDir: dataDir, snapshotID: snapshotID, authService: authService, branding: brandingService}
	f.manager = NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     wikiVersion,
		WriteGate:       NewWriteGate(),
		AuthService:     authService,
		BrandingService: brandingService,
		TriggerResync:   func() { f.resyncCalls++ },
	})

	t.Cleanup(func() { _ = authService.Close() })

	return f
}

func waitForRestoreDone(t *testing.T, m *Manager) JobStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s := m.Status()
		if s.Done {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("restore did not finish within deadline")
	return JobStatus{}
}

func TestManager_Restore_HappyPath(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}

	status := waitForRestoreDone(t, f.manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}
	if status.NeedsIntervention {
		t.Fatal("expected NeedsIntervention = false on a successful restore")
	}

	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "welcome.md")); err != nil {
		t.Errorf("expected restored root/welcome.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "live-page.md")); !os.IsNotExist(err) {
		t.Errorf("expected pre-restore live content to be gone, got err=%v", err)
	}

	if f.resyncCalls != 1 {
		t.Errorf("expected resync to be triggered exactly once, got %d", f.resyncCalls)
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("expected write gate to be disengaged after a successful restore")
	}

	// AuthService was hot-swapped to the restored users.db: the live-only
	// admin is gone, the snapshot's admin can log in.
	if _, err := f.authService.Login("live-admin", "live-password-123"); err == nil {
		t.Fatal("expected the live-only user to no longer exist after restore")
	}
	if _, err := f.authService.Login("snapshot-admin", "snapshot-password-123"); err != nil {
		t.Fatalf("expected the snapshot's user to be able to log in after restore: %v", err)
	}

	brandingCfg, err := f.branding.GetBranding()
	if err != nil {
		t.Fatalf("GetBranding failed: %v", err)
	}
	if brandingCfg.SiteName != "Snapshot Site" {
		t.Errorf("expected branding reloaded from the restored branding.json, got SiteName=%q", brandingCfg.SiteName)
	}
}

// TestManager_Restore_ByID_DoesNotEnforceUploadExtractionCaps is a
// regression test for the zip-bomb DoS fix's by-id/by-upload distinction: a
// snapshot the server itself created is trusted content, not
// attacker-controlled input, so restoring it by id must not be rejected by
// the same DefaultZipExtractionLimits an untrusted upload goes through — a
// legitimately large or highly-compressible (e.g. repetitive markdown, which
// is exactly what a real wiki page often is) wiki must still be restorable.
func TestManager_Restore_ByID_DoesNotEnforceUploadExtractionCaps(t *testing.T) {
	src := t.TempDir()
	rootDir := filepath.Join(src, "root")
	// Highly repetitive content, past DefaultZipExtractionLimits' 1 MiB
	// ratio floor, compresses at a ratio the by-upload path would flag as a
	// decompression bomb (well over its 100:1 threshold).
	test_utils.WriteFile(t, rootDir, "bomb-shaped.md", strings.Repeat("A", 2*1024*1024))
	createRealUsersDB(t, src, "snap-admin", "snap-admin@example.com", "snap-password-123")

	snapshotMgr := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  filepath.Join(src, "backups"),
		RootDir:     rootDir,
		AssetsDir:   filepath.Join(src, "assets"),
		BrandingDir: filepath.Join(src, "branding"),
		UsersDBPath: filepath.Join(src, "users.db"),
		WikiVersion: "v1.0.0",
	})
	if err := snapshotMgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotMgr.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	snapshotID := entries[0].ID
	zipPath, err := snapshotMgr.SnapshotZipPath(snapshotID)
	if err != nil {
		t.Fatalf("SnapshotZipPath failed: %v", err)
	}

	// Sanity check: the same content, run through the capped (by-upload)
	// extraction path directly, really is rejected as ratio-bomb-shaped —
	// otherwise this test wouldn't be proving anything about the by-id path
	// being different.
	if _, _, err := extractAndValidate(zipPath, t.TempDir()); !errors.Is(err, coreshared.ErrDecompressionRatioTooHigh) {
		t.Fatalf("expected the capped (by-upload) extraction path to reject this fixture as a ratio bomb, got: %v", err)
	}

	dataDir := t.TempDir()
	createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")
	sessionStore, err := auth.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })
	brandingService, err := branding.NewBrandingService(dataDir)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	m := NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     "v1.0.0",
		WriteGate:       NewWriteGate(),
		AuthService:     authService,
		BrandingService: brandingService,
		TriggerResync:   func() {},
	})

	if err := m.TriggerRestore(snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, m)
	if status.Error != "" {
		t.Fatalf("expected by-id restore to succeed despite ratio-bomb-shaped content, got: %s", status.Error)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "root", "bomb-shaped.md")); err != nil {
		t.Errorf("expected restored root/bomb-shaped.md: %v", err)
	}
}

func TestManager_Restore_InvalidatesSessions(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	if err := f.authService.RevokeAllUserSessions("some-user-id"); err != nil {
		t.Fatalf("sanity RevokeAllUserSessions failed: %v", err)
	}

	// Seed an active session directly (independent of Login/bcrypt) via a
	// second handle onto the same sessions.db.
	probe, err := auth.NewSessionStore(f.dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore (probe) failed: %v", err)
	}
	t.Cleanup(func() { _ = probe.Close() })

	expiresAt := time.Now().Add(time.Hour)
	if err := probe.CreateSession("jti-1", "user-1", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	active, err := probe.IsActive("jti-1", "user-1", "refresh", time.Now())
	if err != nil || !active {
		t.Fatalf("expected the seeded session to be active before restore (active=%v, err=%v)", active, err)
	}

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}

	active, err = probe.IsActive("jti-1", "user-1", "refresh", time.Now())
	if err != nil {
		t.Fatalf("IsActive failed: %v", err)
	}
	if active {
		t.Error("expected the session to be invalidated by the restore")
	}
}

func TestManager_Restore_UnknownSnapshotID_FailsCleanlyWithoutTouchingFiles(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	if err := f.manager.TriggerRestore("snapshot-does-not-exist"); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)
	if status.Error == "" {
		t.Fatal("expected an error for an unknown snapshot id")
	}
	if status.NeedsIntervention {
		t.Error("an unknown id should fail during validation, before anything is touched — not NeedsIntervention")
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("write gate should never have been engaged for a validation failure")
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "live-page.md")); err != nil {
		t.Errorf("expected live content to be completely untouched: %v", err)
	}
}

func TestManager_Restore_ErrAlreadyRunning(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("first TriggerRestore failed: %v", err)
	}

	err := f.manager.TriggerRestore(f.snapshotID)
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok || loc.Code != "restore_already_running" {
		t.Fatalf("expected restore_already_running, got %v", err)
	}

	waitForRestoreDone(t, f.manager)
}

// TestManager_Restore_RejectsNewTriggerWhileNeedsIntervention is the
// regression test for a real gap found in review: TriggerRestore had no
// guard against starting a brand-new restore while a previous one left the
// instance in a NeedsIntervention state (write-gate stuck engaged, possibly
// half-swapped filesystem) — silently clearing that flag and compounding the
// corruption instead of forcing the documented self-restart recovery.
func TestManager_Restore_RejectsNewTriggerWhileNeedsIntervention(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	// Force the job directly into NeedsIntervention without going through a
	// real failure sequence — Job.Start()/FinishNeedsIntervention() are
	// exercised in isolation by job_test.go; here we only need Manager to
	// observe that state.
	f.manager.job.Start()
	f.manager.job.FinishNeedsIntervention(errTest("rollback also failed"))

	err := f.manager.TriggerRestore(f.snapshotID)
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok || loc.Code != "restore_needs_intervention" {
		t.Fatalf("expected restore_needs_intervention, got %v", err)
	}

	// And Start() itself must not have been called — status should still
	// reflect the stuck NeedsIntervention state, not a freshly running job.
	status := f.manager.Status()
	if status.Running {
		t.Error("TriggerRestore must not start a new run while NeedsIntervention is set")
	}
	if !status.NeedsIntervention {
		t.Error("expected NeedsIntervention to remain set after a rejected trigger")
	}
}

func TestManager_Restore_RollsBackOnBrandingReloadFailure(t *testing.T) {
	// The snapshot's branding.json is deliberately invalid JSON: the file
	// swap itself succeeds (SwapAll doesn't parse content), but
	// BrandingService.Reload() then fails to unmarshal it — exercising the
	// "roll back everything, including the already-succeeded file swap and
	// auth reopen" path (refinement over the original plan doc, which only
	// kept pre-restore copies until the rename step, not the whole sequence).
	f := newManagerFixtureWithBranding(t, "v1.0.0", `not valid json {{{`)

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)

	if status.Error == "" {
		t.Fatal("expected the restore to fail when branding reload fails")
	}
	if status.NeedsIntervention {
		t.Fatal("rollback should have succeeded here, so this should be a plain failure, not NeedsIntervention")
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("expected write gate to be disengaged after a successful rollback")
	}

	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "welcome.md")); !os.IsNotExist(err) {
		t.Errorf("expected the restored snapshot content to have been rolled back, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "live-page.md")); err != nil {
		t.Errorf("expected the original live content to be back in place after rollback: %v", err)
	}

	// AuthService.ReplaceUserStore already succeeded before the branding
	// phase failed — the rollback must re-sync AuthService's in-memory
	// handle back to the restored-to-original users.db, not just the file on
	// disk, otherwise it would keep silently serving the rolled-back-away
	// content through an orphaned (unlinked but still open) file handle.
	if _, err := f.authService.Login("live-admin", "live-password-123"); err != nil {
		t.Fatalf("expected the original live user to be able to log in again after rollback: %v", err)
	}
	if _, err := f.authService.Login("snapshot-admin", "snapshot-password-123"); err == nil {
		t.Fatal("expected the snapshot's user to no longer exist after rollback")
	}
}

func TestManager_MaxUploadSizeBytes_DefaultsWhenUnset(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	if got := f.manager.MaxUploadSizeBytes(); got != DefaultMaxUploadSizeBytes {
		t.Errorf("expected default %d, got %d", DefaultMaxUploadSizeBytes, got)
	}
}

func TestManager_MaxUploadSizeBytes_UsesConfiguredValue(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")
	f.manager.cfg.MaxUploadSizeBytes = 12345

	if got := f.manager.MaxUploadSizeBytes(); got != 12345 {
		t.Errorf("expected configured value 12345, got %d", got)
	}
}

func TestManager_RestoreFromUpload_HappyPath(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	upload, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = upload.Close() }()

	if err := f.manager.TriggerRestoreFromUpload(upload); err != nil {
		t.Fatalf("TriggerRestoreFromUpload failed: %v", err)
	}

	status := waitForRestoreDone(t, f.manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}
	if status.NeedsIntervention {
		t.Fatal("expected NeedsIntervention = false on a successful restore")
	}

	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "welcome.md")); err != nil {
		t.Errorf("expected restored root/welcome.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "live-page.md")); !os.IsNotExist(err) {
		t.Errorf("expected pre-restore live content to be gone, got err=%v", err)
	}

	if f.resyncCalls != 1 {
		t.Errorf("expected resync to be triggered exactly once, got %d", f.resyncCalls)
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("expected write gate to be disengaged after a successful restore")
	}

	if _, err := f.authService.Login("snapshot-admin", "snapshot-password-123"); err != nil {
		t.Fatalf("expected the snapshot's user to be able to log in after restore: %v", err)
	}
}

func TestManager_RestoreFromUpload_InvalidZip_FailsCleanlyWithoutTouchingFiles(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	// A zip with no entries at all is missing both required entries
	// (backup-meta.json, users.db) — extractAndValidate must reject it before
	// anything on disk is touched.
	badZipPath := filepath.Join(t.TempDir(), "bad.zip")
	zf, err := os.Create(badZipPath)
	if err != nil {
		t.Fatalf("failed to create bad zip file: %v", err)
	}
	zw := zip.NewWriter(zf)
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close empty zip writer: %v", err)
	}
	if err := zf.Close(); err != nil {
		t.Fatalf("failed to close bad zip file: %v", err)
	}

	upload, err := os.Open(badZipPath)
	if err != nil {
		t.Fatalf("failed to open bad zip: %v", err)
	}
	defer func() { _ = upload.Close() }()

	if err := f.manager.TriggerRestoreFromUpload(upload); err != nil {
		t.Fatalf("TriggerRestoreFromUpload failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)
	if status.Error == "" {
		t.Fatal("expected an error for an invalid zip")
	}
	if status.NeedsIntervention {
		t.Error("an invalid zip should fail during validation, before anything is touched — not NeedsIntervention")
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("write gate should never have been engaged for a validation failure")
	}
	if _, err := os.Stat(filepath.Join(f.dataDir, "root", "live-page.md")); err != nil {
		t.Errorf("expected live content to be completely untouched: %v", err)
	}
}

// TestManager_RestoreFromUpload_StageUploadFailure_ReturnsLocalizedError is
// the regression test for a review finding: stageUpload used to return plain
// fmt.Errorf, which respondWithRestoreError can't render specifically (it
// only recognizes *sharederrors.LocalizedError), so any staging failure
// (disk full, permission error, ...) surfaced to the admin as a generic
// "Restore request failed" instead of a real message — unlike every other
// synchronous error this endpoint can return.
func TestManager_RestoreFromUpload_StageUploadFailure_ReturnsLocalizedError(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")
	// A DataDir whose parent doesn't exist makes os.CreateTemp fail
	// deterministically, without relying on real disk-full/permission errors.
	f.manager.cfg.DataDir = filepath.Join(f.dataDir, "does-not-exist")

	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	upload, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = upload.Close() }()

	err = f.manager.TriggerRestoreFromUpload(upload)
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok || loc.Code != "restore_upload_staging_failed" {
		t.Fatalf("expected restore_upload_staging_failed, got %v", err)
	}
}

func TestManager_RestoreFromUpload_ErrAlreadyRunning(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	upload1, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = upload1.Close() }()

	if err := f.manager.TriggerRestoreFromUpload(upload1); err != nil {
		t.Fatalf("first TriggerRestoreFromUpload failed: %v", err)
	}

	upload2, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = upload2.Close() }()

	err = f.manager.TriggerRestoreFromUpload(upload2)
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok || loc.Code != "restore_already_running" {
		t.Fatalf("expected restore_already_running, got %v", err)
	}

	waitForRestoreDone(t, f.manager)
}

func TestManager_RestoreFromUpload_RejectsWhileNeedsIntervention(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	f.manager.job.Start()
	f.manager.job.FinishNeedsIntervention(errTest("rollback also failed"))

	zipPath := buildFixtureSnapshot(t, "v1.0.0")
	upload, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("failed to open fixture zip: %v", err)
	}
	defer func() { _ = upload.Close() }()

	err = f.manager.TriggerRestoreFromUpload(upload)
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok || loc.Code != "restore_needs_intervention" {
		t.Fatalf("expected restore_needs_intervention, got %v", err)
	}

	status := f.manager.Status()
	if status.Running {
		t.Error("TriggerRestoreFromUpload must not start a new run while NeedsIntervention is set")
	}
	if !status.NeedsIntervention {
		t.Error("expected NeedsIntervention to remain set after a rejected trigger")
	}
}

// TestManager_RestoreFromUpload_CleansUpTempFileAfterCompletion covers both a
// successful and a failed upload restore: in either case, the temp file
// stageUpload wrote the upload into (inside dataDir) must be gone once the
// job finishes — it's a private implementation detail, not a snapshot to
// retain.
func TestManager_RestoreFromUpload_CleansUpTempFileAfterCompletion(t *testing.T) {
	tempGlob := func(dataDir string) []string {
		matches, err := filepath.Glob(filepath.Join(dataDir, ".leafwiki-restore-upload-*.zip"))
		if err != nil {
			t.Fatalf("Glob failed: %v", err)
		}
		return matches
	}

	t.Run("success", func(t *testing.T) {
		f := newManagerFixture(t, "v1.0.0")
		zipPath := buildFixtureSnapshot(t, "v1.0.0")
		upload, err := os.Open(zipPath)
		if err != nil {
			t.Fatalf("failed to open fixture zip: %v", err)
		}
		defer func() { _ = upload.Close() }()

		if err := f.manager.TriggerRestoreFromUpload(upload); err != nil {
			t.Fatalf("TriggerRestoreFromUpload failed: %v", err)
		}
		waitForRestoreDone(t, f.manager)

		if matches := tempGlob(f.dataDir); len(matches) != 0 {
			t.Errorf("expected upload temp file to be cleaned up, found: %v", matches)
		}
	})

	t.Run("failure", func(t *testing.T) {
		f := newManagerFixture(t, "v1.0.0")
		badZipPath := filepath.Join(t.TempDir(), "bad.zip")
		zf, err := os.Create(badZipPath)
		if err != nil {
			t.Fatalf("failed to create bad zip file: %v", err)
		}
		zw := zip.NewWriter(zf)
		if err := zw.Close(); err != nil {
			t.Fatalf("failed to close empty zip writer: %v", err)
		}
		if err := zf.Close(); err != nil {
			t.Fatalf("failed to close bad zip file: %v", err)
		}

		upload, err := os.Open(badZipPath)
		if err != nil {
			t.Fatalf("failed to open bad zip: %v", err)
		}
		defer func() { _ = upload.Close() }()

		if err := f.manager.TriggerRestoreFromUpload(upload); err != nil {
			t.Fatalf("TriggerRestoreFromUpload failed: %v", err)
		}
		status := waitForRestoreDone(t, f.manager)
		if status.Error == "" {
			t.Fatal("expected the invalid zip to fail validation")
		}

		if matches := tempGlob(f.dataDir); len(matches) != 0 {
			t.Errorf("expected upload temp file to be cleaned up even on failure, found: %v", matches)
		}
	})
}

// TestManager_Restore_BlocksConcurrentWritesDuringSwap runs real goroutines
// racing WriteGate.TryEnter against a real Manager.runLocked (file swap, auth
// reopen, branding reload) — closing the gap between the gate primitive's own
// race coverage (writegate_test.go) and the HTTP middleware's gate coverage
// (internal/http/middleware/maintenance/writegate_test.go), neither of which
// exercises an actual in-flight restore.
func TestManager_Restore_BlocksConcurrentWritesDuringSwap(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")

	var admitted, rejected int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if leave, ok := f.manager.cfg.WriteGate.TryEnter(); ok {
					atomic.AddInt64(&admitted, 1)
					time.Sleep(time.Millisecond)
					leave()
				} else {
					atomic.AddInt64(&rejected, 1)
				}
			}
		}()
	}

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)
	close(stop)
	wg.Wait()

	if status.Error != "" {
		t.Fatalf("expected successful restore, got: %s", status.Error)
	}
	if atomic.LoadInt64(&rejected) == 0 {
		t.Error("expected at least some concurrent TryEnter calls to be rejected while the gate was engaged")
	}
	if f.manager.cfg.WriteGate.Engaged() {
		t.Error("expected write gate to be disengaged after restore completes")
	}
}

// TestManager_Restore_HappyPath_PreservesAPIKeys is the regression test for
// the bug where a full backup/restore cycle silently dropped api_keys.db: a
// key minted before the snapshot was taken (against the snapshot-source
// dataDir) must still resolve after restore, and a key minted only against
// the live dataDir (after the snapshot, so never captured) must be gone —
// exactly mirroring how TestManager_Restore_HappyPath already asserts this
// for users.db content.
func TestManager_Restore_HappyPath_PreservesAPIKeys(t *testing.T) {
	src := t.TempDir()
	test_utils.WriteFile(t, src, "root/welcome.md", "# Snapshot content\n")
	snapshotOwnerID := createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")
	snapshotToken := createRealAPIKeysDB(t, src, snapshotOwnerID, "snapshot-key")

	snapshotMgr := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:    filepath.Join(src, "backups"),
		RootDir:       filepath.Join(src, "root"),
		UsersDBPath:   filepath.Join(src, "users.db"),
		APIKeysDBPath: filepath.Join(src, "api_keys.db"),
		WikiVersion:   "v1.0.0",
	})
	if err := snapshotMgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotMgr.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	snapshotID := entries[0].ID

	dataDir := t.TempDir()
	liveOwnerID := createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")
	liveToken := createRealAPIKeysDB(t, dataDir, liveOwnerID, "live-key")

	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessionStore, err := auth.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })

	apiKeyStore, err := auth.NewAPIKeyStore(dataDir)
	if err != nil {
		t.Fatalf("NewAPIKeyStore failed: %v", err)
	}
	apiKeyService := auth.NewAPIKeyService(apiKeyStore, authService)
	t.Cleanup(func() { _ = apiKeyService.Close() })

	brandingService, err := branding.NewBrandingService(dataDir)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	manager := NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     "v1.0.0",
		WriteGate:       NewWriteGate(),
		AuthService:     authService,
		APIKeyService:   apiKeyService,
		BrandingService: brandingService,
	})

	if err := manager.TriggerRestore(snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}

	if _, err := apiKeyService.Resolve(snapshotToken); err != nil {
		t.Errorf("expected the snapshot's api key to still resolve after restore, got: %v", err)
	}
	if _, err := apiKeyService.Resolve(liveToken); err == nil {
		t.Error("expected the live-only api key (created after the snapshot) to no longer resolve after restore")
	}
}

// TestManager_Restore_HappyPath_PreservesFavoritesAndUserSettings is the
// regression test for the bug where a full backup/restore cycle silently
// dropped favorites.db and usersettings.db: a favorite/setting saved before
// the snapshot was taken (against the snapshot-source dataDir) must still be
// present after restore, and a favorite/setting saved only against the live
// dataDir (after the snapshot, so never captured) must be gone — mirroring
// TestManager_Restore_HappyPath_PreservesAPIKeys's shape. This is also the
// test that actually proves the in-place Replace fix works (the running
// process's Favorites/UserSettings pointers must reflect the restored data,
// not a stale open file handle from before the swap).
func TestManager_Restore_HappyPath_PreservesFavoritesAndUserSettings(t *testing.T) {
	src := t.TempDir()
	test_utils.WriteFile(t, src, "root/welcome.md", "# Snapshot content\n")
	createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")
	createRealFavoritesDB(t, src, "user-1", "snapshot-page")
	createRealUserSettingsDB(t, src, "user-1", "de")

	snapshotMgr := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:         filepath.Join(src, "backups"),
		RootDir:            filepath.Join(src, "root"),
		UsersDBPath:        filepath.Join(src, "users.db"),
		FavoritesDBPath:    filepath.Join(src, "favorites.db"),
		UserSettingsDBPath: filepath.Join(src, "usersettings.db"),
		WikiVersion:        "v1.0.0",
	})
	if err := snapshotMgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotMgr.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	snapshotID := entries[0].ID

	dataDir := t.TempDir()
	createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")
	createRealFavoritesDB(t, dataDir, "user-1", "live-only-page")
	createRealUserSettingsDB(t, dataDir, "user-1", "en")

	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessionStore, err := auth.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })

	favoritesStore, err := favorites.NewFavoritesStore(dataDir, nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore failed: %v", err)
	}
	t.Cleanup(func() { _ = favoritesStore.Close() })

	userSettingsStore, err := usersettings.NewUserSettingsStore(dataDir, nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore failed: %v", err)
	}
	userSettingsService := usersettings.NewUserSettingsService(userSettingsStore)
	t.Cleanup(func() { _ = userSettingsService.Close() })

	brandingService, err := branding.NewBrandingService(dataDir)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	manager := NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     "v1.0.0",
		WriteGate:       NewWriteGate(),
		AuthService:     authService,
		Favorites:       favoritesStore,
		UserSettings:    userSettingsService,
		BrandingService: brandingService,
	})

	if err := manager.TriggerRestore(snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}

	pageIDs, err := favoritesStore.ListPageIDsForUser("user-1")
	if err != nil {
		t.Fatalf("ListPageIDsForUser failed: %v", err)
	}
	if len(pageIDs) != 1 || pageIDs[0] != "snapshot-page" {
		t.Errorf("expected only the snapshot's favorite (snapshot-page) to survive restore, got %v", pageIDs)
	}

	settings, err := userSettingsService.Get("user-1")
	if err != nil {
		t.Fatalf("Get user settings failed: %v", err)
	}
	if settings.Language != "de" {
		t.Errorf("expected the snapshot's language (de) to survive restore, got %q", settings.Language)
	}
}

// TestManager_Restore_HappyPath_ReloadsUserResolverCache is a regression test
// for the UserResolver half of "User-Management Routes Go Stale After Live
// Restore": UserResolver's own in-memory author-label cache is a separate
// thing from AuthService's live UserService pointer — even with that pointer
// fixed, a label resolved before the restore stays cached (and wrong) unless
// something calls UserResolver.Reload() after the swap. This mirrors
// TestManager_Restore_HappyPath_PreservesAPIKeys's shape.
func TestManager_Restore_HappyPath_ReloadsUserResolverCache(t *testing.T) {
	src := t.TempDir()
	test_utils.WriteFile(t, src, "root/welcome.md", "# Snapshot content\n")
	snapshotOwnerID := createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")

	snapshotMgr := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:  filepath.Join(src, "backups"),
		RootDir:     filepath.Join(src, "root"),
		UsersDBPath: filepath.Join(src, "users.db"),
		WikiVersion: "v1.0.0",
	})
	if err := snapshotMgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotMgr.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}
	snapshotID := entries[0].ID

	dataDir := t.TempDir()
	liveOwnerID := createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")

	userStore, err := auth.NewUserStore(dataDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	sessionStore, err := auth.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	sessions := auth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authService := auth.NewAuthService(auth.NewUserService(userStore), sessions, nil)
	t.Cleanup(func() { _ = authService.Close() })

	userResolver, err := auth.NewUserResolver(authService.UserService)
	if err != nil {
		t.Fatalf("NewUserResolver failed: %v", err)
	}

	// Populate the resolver's cache for the live-only admin before the
	// restore — this is the entry that must NOT survive the restore.
	if _, err := userResolver.ResolveUserLabel(liveOwnerID); err != nil {
		t.Fatalf("failed to pre-populate resolver cache for live admin: %v", err)
	}

	brandingService, err := branding.NewBrandingService(dataDir)
	if err != nil {
		t.Fatalf("NewBrandingService failed: %v", err)
	}

	manager := NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     "v1.0.0",
		WriteGate:       NewWriteGate(),
		AuthService:     authService,
		BrandingService: brandingService,
		UserResolver:    userResolver,
	})

	if err := manager.TriggerRestore(snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}

	if label, err := userResolver.ResolveUserLabel(snapshotOwnerID); err != nil || label == nil || label.Username != "snapshot-admin" {
		t.Errorf("expected the resolver to resolve the restored snapshot admin, got label=%+v err=%v", label, err)
	}
	if label, err := userResolver.ResolveUserLabel(liveOwnerID); err == nil {
		t.Errorf("expected the pre-restore cached label for the live-only admin to be gone after restore, got stale label=%+v", label)
	}
}

// TestManager_Restore_APIKeyManagementDisabled_DoesNotError covers the common
// case (API key management is off by default): cfg.APIKeyService is nil and
// no api_keys.db was ever created, so restore must complete normally — the
// new APIKeyService nil-guards in runFromZipPath/rollbackOrIntervene must not
// panic, and swapNames' "skip items absent from the staging dir" behavior
// must leave the missing api_keys.db alone.
func TestManager_Restore_APIKeyManagementDisabled_DoesNotError(t *testing.T) {
	f := newManagerFixture(t, "v1.0.0")
	f.manager.cfg.APIKeyService = nil

	if err := f.manager.TriggerRestore(f.snapshotID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	status := waitForRestoreDone(t, f.manager)
	if status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}
	if status.NeedsIntervention {
		t.Fatal("expected NeedsIntervention = false on a successful restore")
	}
}
