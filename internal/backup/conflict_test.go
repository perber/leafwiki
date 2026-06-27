package backup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConflict_DivergedHistory tests that RunBackup sets NeedsIntervention when
// local and remote histories have diverged (ErrNonFastForwardUpdate).
//
// Setup:
//  1. Local repo A: commit + push → Remote = HEAD1
//  2. External client: commit + push → Remote = HEAD2 (parent HEAD1)
//  3. Local repo A: direct commit (no push) → Local = HEAD3 (parent HEAD1, diverges from HEAD2)
//  4. RunBackup → pull detects non-fast-forward → NeedsIntervention
func TestConflict_DivergedHistory(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	// External commit advances the remote
	commitToRemote(t, bareDir, "root/remote-only.md", "from remote\n")

	// Local diverged commit (bypass RunBackup so it never pushes)
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")

	err := repo.RunBackup()
	if err == nil {
		t.Fatal("expected RunBackup to return an error on diverged history")
	}

	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after diverged history")
	}
	if snap.ConflictDetails == "" {
		t.Error("expected ConflictDetails to be set")
	}
}

// TestConflict_FileConflict tests that RunBackup sets NeedsIntervention when
// the same file is modified on the remote and is dirty on disk (ErrUnstagedChanges).
//
// Setup:
//  1. Local repo A: page.md = "A" committed + pushed
//  2. External client: page.md = "B" committed + pushed to remote
//  3. Local on disk: page.md = "C" (not staged, not committed)
//  4. RunBackup → pull would overwrite "C" with "B" → ErrUnstagedChanges → NeedsIntervention
func TestConflict_FileConflict(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	// Remote gets an update to the same file
	commitToRemote(t, bareDir, "root/page.md", "version B from remote\n")

	// Local disk has an uncommitted modification to the same file
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := repo.RunBackup()
	if err == nil {
		t.Fatal("expected RunBackup to return an error on file conflict")
	}

	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after file conflict")
	}
}

// TestConflict_FirstPush_NoError tests that RunBackup succeeds when the remote
// branch does not exist yet (ErrReferenceNotFound treated as first push).
func TestConflict_FirstPush_NoError(t *testing.T) {
	bareDir := initBareRemote(t)

	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	}
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("expected first RunBackup to succeed (first push), got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = false on first push")
	}
	if snap.LastError != "" {
		t.Errorf("expected no error, got %q", snap.LastError)
	}
}

// TestConflict_RemoteAhead_FastForwardMerge tests that RunBackup pulls a
// fast-forward from the remote when local is behind, without conflict.
func TestConflict_RemoteAhead_FastForwardMerge(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	// Remote gets a new file; local is clean (no new local commits)
	commitToRemote(t, bareDir, "root/from-remote.md", "remote content\n")

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("expected RunBackup to succeed after fast-forward, got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected no intervention needed after clean fast-forward")
	}
	if snap.LastError != "" {
		t.Errorf("expected no error, got %q", snap.LastError)
	}
}

// TestConflict_RemoteAheadPlusLocalChanges tests that RunBackup pulls from
// remote (fast-forward) AND commits local changes in the same cycle.
func TestConflict_RemoteAheadPlusLocalChanges(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	// Remote gets a new file
	commitToRemote(t, bareDir, "root/from-remote.md", "remote content\n")

	// Local has a new file too (different name, no conflict)
	if err := os.WriteFile(filepath.Join(rootDir, "local-new.md"), []byte("local new\n"), 0644); err != nil {
		t.Fatalf("WriteFile local-new.md: %v", err)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("expected RunBackup to succeed, got: %v", err)
	}

	snap := repo.Status()
	if snap.NeedsIntervention {
		t.Error("expected no intervention needed")
	}
	if snap.LastError != "" {
		t.Errorf("expected no error, got %q", snap.LastError)
	}
	if snap.LastBackupAt == nil {
		t.Error("expected LastBackupAt to be set")
	}
}
