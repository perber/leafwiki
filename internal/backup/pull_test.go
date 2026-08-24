package backup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPull_FastForward tests that Pull merges new remote commits into the
// local working tree without requiring a full backup cycle.
func TestPull_FastForward(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/from-remote.md", "remote content\n")

	if err := repo.Pull(); err != nil {
		t.Fatalf("expected Pull to succeed on fast-forward, got: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, "from-remote.md")); err != nil {
		t.Errorf("expected pulled file to exist on disk, got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected no intervention needed after clean fast-forward")
	}
}

// TestPull_AlreadyUpToDate tests that Pull is a no-op (no error) when there is
// nothing new on the remote.
func TestPull_AlreadyUpToDate(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	if err := repo.Pull(); err != nil {
		t.Fatalf("expected Pull to be a no-op when already up to date, got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected no intervention needed")
	}
}

// TestPull_FileConflict tests that Pull sets NeedsIntervention when the same
// file was changed both on the remote and on disk (uncommitted local change).
func TestPull_FileConflict(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/page.md", "version B from remote\n")

	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := repo.Pull()
	if err == nil {
		t.Fatal("expected Pull to return an error on file conflict")
	}

	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after file conflict")
	}
	if snap.ConflictDetails == "" {
		t.Error("expected ConflictDetails to be set")
	}
}

// TestPull_DivergedHistory tests that Pull sets NeedsIntervention when local
// and remote histories have diverged (ErrNonFastForwardUpdate).
func TestPull_DivergedHistory(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/remote-only.md", "from remote\n")
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")

	err := repo.Pull()
	if err == nil {
		t.Fatal("expected Pull to return an error on diverged history")
	}

	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after diverged history")
	}
}

// TestPull_NoRemoteConfigured tests that Pull is a no-op when no remote URL is
// configured (defensive — the HTTP handler already guards on repo == nil, but
// Pull itself must not panic or error if called on a repo without a remote).
func TestPull_NoRemoteConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	repo, err := Init(Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := repo.Pull(); err != nil {
		t.Fatalf("expected Pull to be a no-op without a remote, got: %v", err)
	}
}

// TestPull_FailsFastWhenBackupRunning tests that Pull mirrors ForcePush and
// fails immediately (rather than blocking) when a backup cycle already holds
// the lock — RunBackup's own network pull/push can each take up to
// networkTimeout, which would otherwise stall the HTTP request behind it.
func TestPull_FailsFastWhenBackupRunning(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	repo.mu.Lock()
	defer repo.mu.Unlock()

	if err := repo.Pull(); err == nil {
		t.Fatal("expected Pull to fail fast when a backup cycle is already running")
	}
}

// TestPull_RecoversFromPriorConflict tests that a successful Pull clears a
// stale NeedsIntervention/LastError left by an earlier failed Pull, without
// bumping LastBackupAt (a pull is not a backup).
func TestPull_RecoversFromPriorConflict(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/page.md", "version B from remote\n")
	pagePath := filepath.Join(rootDir, "page.md")
	if err := os.WriteFile(pagePath, []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := repo.Pull(); err == nil {
		t.Fatal("expected first Pull to fail on file conflict")
	}
	if snap := repo.Status(); !snap.NeedsIntervention {
		t.Fatal("expected NeedsIntervention after conflict")
	}
	lastBackupBefore := *repo.Status().LastBackupAt

	// Resolve the conflict: discard the local edit so the working tree
	// matches HEAD again, allowing the next pull to fast-forward cleanly.
	if err := os.WriteFile(pagePath, []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("WriteFile (revert local change): %v", err)
	}

	if err := repo.Pull(); err != nil {
		t.Fatalf("expected second Pull to succeed after resolving the conflict, got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected NeedsIntervention to be cleared after a successful recovery pull")
	}
	if snap.LastError != "" {
		t.Errorf("expected LastError to be cleared, got %q", snap.LastError)
	}
	if snap.LastBackupAt == nil || !snap.LastBackupAt.Equal(lastBackupBefore) {
		t.Errorf("expected LastBackupAt to be unchanged by a mere pull, before=%v after=%v", lastBackupBefore, snap.LastBackupAt)
	}
}
