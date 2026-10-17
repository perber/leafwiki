package backup

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
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

// TestPull_UnbornLocalHead_RemoteAdvanced tests that Pull does not silently
// overwrite a local file when the local branch is still unborn (no commit
// yet) but the remote already has history that happens to contain a file at
// the same path with different content.
//
// This reproduces the window where Init() leaves the local branch unborn
// because root/assets were empty AND the remote branch didn't exist yet, but
// the remote gains commits (e.g. from another instance, or an admin) before
// this instance's first pull — a case materializeContent's checkConflicts
// flag (gated on baseCommit != nil) does not cover, unlike the equivalent
// first-contact path in syncContentFromRemote (called from Init when the
// remote already has history at Init time), which is deliberately additive.
func TestPull_UnbornLocalHead_RemoteAdvanced(t *testing.T) {
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

	repo, err := Init(Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if head, herr := repo.headCommit(); herr != nil || head != nil {
		t.Fatalf("test precondition failed: expected an unborn local HEAD, got head=%v err=%v", head, herr)
	}

	// The wiki writes local content before it has ever been committed by a
	// backup cycle (e.g. the seeded welcome page).
	if err := os.WriteFile(filepath.Join(rootDir, "welcome.md"), []byte("local welcome\n"), 0644); err != nil {
		t.Fatalf("WriteFile welcome.md: %v", err)
	}

	// Meanwhile the remote branch is created out of band with a file at the
	// same path but different content.
	pushFirstCommitToRemote(t, bareDir, "main", "root/welcome.md", "remote welcome\n")

	if err := repo.Pull(); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(rootDir, "welcome.md"))
	if err != nil {
		t.Fatalf("ReadFile welcome.md: %v", err)
	}
	if string(got) != "local welcome\n" {
		t.Errorf("welcome.md = %q, want local content preserved (%q)", string(got), "local welcome\n")
	}
}

// TestPull_CaseInsensitiveFilesystem_DoesNotClobberDifferentlyCasedFile tests
// that a live file is not silently overwritten via a differently-cased path
// that the OS treats as the same file. Only meaningful on a case-insensitive
// filesystem (default on Windows/macOS); skipped on a case-sensitive one
// (default on Linux/CI), where this scenario cannot occur.
func TestPull_CaseInsensitiveFilesystem_DoesNotClobberDifferentlyCasedFile(t *testing.T) {
	if !filesystemIsCaseInsensitive(t) {
		t.Skip("filesystem is case-sensitive; this scenario only applies on a case-insensitive filesystem (Windows/macOS)")
	}

	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	// Local rename to a different case. On this filesystem "Page.md" and
	// "page.md" are the same physical file.
	if err := os.Rename(filepath.Join(rootDir, "page.md"), filepath.Join(rootDir, "Page.md")); err != nil {
		t.Fatalf("Rename: %v", err)
	}

	// External client edits the file under its original (lowercase) path.
	commitToRemote(t, bareDir, "root/page.md", "remote content\n")

	err := repo.Pull()
	if err == nil {
		snap := repo.Status()
		if snap.NeedsIntervention {
			t.Fatal("Pull succeeded but left NeedsIntervention=true — inconsistent status")
		}
		t.Fatal("expected Pull to surface a conflict instead of silently resolving a case-only path mismatch")
	}
	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Errorf("expected NeedsIntervention = true, got err=%v status=%+v", err, snap)
	}
}

// TestPull_CaseFoldedLiveLookup_DetectsDivergedContentUnderDifferentCase
// forces the case-insensitive-filesystem code path (liveFSCaseInsensitive)
// directly, so this regression runs on any CI platform regardless of the
// actual host filesystem's case behaviour. It renames AND edits the local
// file so the assertion only passes if the folded lookup actually found the
// renamed file's *current* content (liveHash from the fold match) rather
// than, say, coincidentally treating the plain lookup miss as "absent" (which
// would also produce a conflict, just not for the reason under test — see the
// end-to-end variant above for the case where content is unchanged, which can
// only be exercised on a filesystem that genuinely folds case).
func TestPull_CaseFoldedLiveLookup_DetectsDivergedContentUnderDifferentCase(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	repo.liveFSCaseInsensitive = true // simulate Windows/macOS regardless of host

	// Local rename to a different case, with an accompanying local edit.
	renamed := filepath.Join(rootDir, "Page.md")
	if err := os.Rename(filepath.Join(rootDir, "page.md"), renamed); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := os.WriteFile(renamed, []byte("local edit under new case\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	commitToRemote(t, bareDir, "root/page.md", "remote content\n")

	err := repo.Pull()
	if err == nil {
		t.Fatal("expected Pull to surface a conflict")
	}
	if snap := repo.Status(); !snap.NeedsIntervention {
		t.Errorf("expected NeedsIntervention = true, got err=%v", err)
	}
	// The local edit must survive untouched.
	got, rerr := os.ReadFile(renamed)
	if rerr != nil {
		t.Fatalf("ReadFile: %v", rerr)
	}
	if string(got) != "local edit under new case\n" {
		t.Errorf("Page.md = %q, want local edit preserved", string(got))
	}
}

// TestPull_CaseFoldedLiveLookup_AcceptsRemoteVersionUnderDifferentCase tests
// that the "already applied" check goes through the same case-folded lookup as
// the conflict check: a file renamed locally to a different case that already
// holds the remote version is not a conflict and keeps that content.
func TestPull_CaseFoldedLiveLookup_AcceptsRemoteVersionUnderDifferentCase(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	repo.liveFSCaseInsensitive = true // simulate Windows/macOS regardless of host

	renamed := filepath.Join(rootDir, "Page.md")
	if err := os.Rename(filepath.Join(rootDir, "page.md"), renamed); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	writeFileHelper(t, rootDir, "Page.md", "remote content\n")
	commitToRemote(t, bareDir, "root/page.md", "remote content\n")

	if err := repo.Pull(); err != nil {
		t.Fatalf("expected Pull to accept the already-applied remote version, got: %v", err)
	}
	if snap := repo.Status(); snap.NeedsIntervention {
		t.Errorf("expected NeedsIntervention = false, got details %q", snap.ConflictDetails)
	}
	readFileEqual(t, renamed, "remote content\n")
}

// TestPull_RemoteRewrittenBetweenListAndFetch tests that a remote history
// rewrite landing in the narrow window between pullBeforeBackup listing the
// remote branch tip and fetching it produces a clear NeedsIntervention
// (diverged history) rather than a confusing, retry-looking generic error
// ("failed to read fetched remote commit") for a hash that was simply never
// fetched.
func TestPull_RemoteRewrittenBetweenListAndFetch(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bareDir)

	// A pending remote change exists so pullBeforeBackup proceeds past the
	// "already up-to-date" check and reaches the list→fetch window.
	commitToRemote(t, bareDir, "root/other.md", "from another instance\n")

	// Simulate the remote being rewritten by a third party exactly in that
	// window: an unrelated history replaces what was just listed.
	repo.afterListBeforeFetch = func() {
		forcePushRewrite(t, bareDir, "main", "root/rewritten.md", "rewritten history\n")
	}

	err := repo.Pull()
	if err == nil {
		t.Fatal("expected Pull to fail after the remote history was rewritten mid-pull")
	}
	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Errorf("expected a clear NeedsIntervention (diverged history) instead of a generic error; err=%v status=%+v", err, snap)
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

// TestPull_RemoteDeletesFileAlreadyMissingLocally tests that a remote delete of
// a file the wiki has already deleted too counts as applied: no conflict, the
// remote's other changes land, and the backup keeps the file deleted.
func TestPull_RemoteDeletesFileAlreadyMissingLocally(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	writeFileHelper(t, rootDir, "gone.md", "to be deleted\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup (seed): %v", err)
	}

	commitChangesToRemote(t, bareDir, map[string]string{"root/new.md": "remote new\n"}, []string{"root/gone.md"})
	if err := os.Remove(filepath.Join(rootDir, "gone.md")); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	readFileEqual(t, filepath.Join(rootDir, "new.md"), "remote new\n")
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/new.md", "remote new\n")
	if _, err := remote.File("root/gone.md"); err == nil {
		t.Error("root/gone.md is back on the remote after both sides deleted it")
	}
}

// TestPull_RepeatedCyclesConverge tests that once a pull and backup succeeded,
// further cycles neither commit, pull nor raise a conflict.
func TestPull_RepeatedCyclesConverge(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitFilesToRemote(t, bareDir, map[string]string{
		"root/a.md":     "a\n",
		"root/sub/b.md": "b\n",
	})
	writeFileHelper(t, rootDir, "local.md", "local\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("first RunBackup: %v", err)
	}
	settled := remoteHeadCommit(t, bareDir).Hash

	for i := range 3 {
		if err := repo.RunBackup(); err != nil {
			t.Fatalf("cycle %d: RunBackup: %v", i, err)
		}
		if got := remoteHeadCommit(t, bareDir).Hash; got != settled {
			t.Fatalf("cycle %d: remote moved to %s, want it to stay at %s", i, got, settled)
		}
		if got := headCommit(t, repo).Hash; got != settled {
			t.Fatalf("cycle %d: local HEAD = %s, want %s", i, got, settled)
		}
		if snap := repo.Status(); snap.NeedsIntervention || snap.LastError != "" {
			t.Fatalf("cycle %d: NeedsIntervention=%v LastError=%q", i, snap.NeedsIntervention, snap.LastError)
		}
	}
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/a.md", "a\n")
	assertFileInCommit(t, remote, "root/sub/b.md", "b\n")
	assertFileInCommit(t, remote, "root/local.md", "local\n")
}

// TestPull_IOFailureMidApply_NextCycleConverges tests the documented guarantee
// for an I/O failure during the apply pass: the files written before the
// failure stay, HEAD does not move, it is reported as an error rather than a
// conflict, and the next cycle accepts those files as applied and finishes.
func TestPull_IOFailureMidApply_NextCycleConverges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not block file creation on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	headBefore := headCommit(t, repo).Hash

	// Writes run in sorted order: a.md lands, z/b.md fails on the read-only z/.
	commitFilesToRemote(t, bareDir, map[string]string{
		"root/a.md":   "a\n",
		"root/z/b.md": "b\n",
	})
	locked := filepath.Join(rootDir, "z")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	err := repo.RunBackup()
	if err == nil {
		t.Fatal("expected RunBackup to fail while z/ is read-only")
	}
	if errors.Is(err, errPullConflict) {
		t.Fatalf("an I/O failure was reported as a conflict: %v", err)
	}
	snap := repo.Status()
	if snap.NeedsIntervention || snap.LastError == "" {
		t.Errorf("want LastError set and no NeedsIntervention, got NeedsIntervention=%v LastError=%q", snap.NeedsIntervention, snap.LastError)
	}
	readFileEqual(t, filepath.Join(rootDir, "a.md"), "a\n")
	if got := headCommit(t, repo).Hash; got != headBefore {
		t.Errorf("HEAD moved to %s after a failed apply, want %s", got, headBefore)
	}

	if err := os.Chmod(locked, 0o755); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after the I/O problem is gone: %v", err)
	}
	readFileEqual(t, filepath.Join(rootDir, "z", "b.md"), "b\n")
	if snap := repo.Status(); snap.NeedsIntervention || snap.LastError != "" {
		t.Errorf("expected a clean status, got NeedsIntervention=%v LastError=%q", snap.NeedsIntervention, snap.LastError)
	}
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/a.md", "a\n")
	assertFileInCommit(t, remote, "root/z/b.md", "b\n")
}

// TestPull_UnbornLocalHead_WritesMissingFilesAndConverges tests the unborn
// branch path end to end: the pull adds the remote files absent locally, keeps
// the local version of a file present on both sides, and the following backup
// pushes that local version on top, after which cycles are no-ops.
func TestPull_UnbornLocalHead_WritesMissingFilesAndConverges(t *testing.T) {
	bareDir := initBareRemote(t)
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	for _, d := range []string{rootDir, assetsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	repo, err := Init(Config{
		RootDir: rootDir, AssetsDir: assetsDir,
		AuthorName: "Test", AuthorEmail: "t@t.com",
		Branch: "main", RemoteURL: "file://" + bareDir, SSHKey: testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeFileHelper(t, rootDir, "welcome.md", "local welcome\n")
	pushFirstCommitToRemote(t, bareDir, "main", "root/welcome.md", "remote welcome\n")
	commitFilesToRemote(t, bareDir, map[string]string{
		"root/a.md":         "a\n",
		"root/sec/b.md":     "b\n",
		"assets/x/logo.svg": "<svg/>\n",
	})

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	readFileEqual(t, filepath.Join(rootDir, "welcome.md"), "local welcome\n")
	readFileEqual(t, filepath.Join(rootDir, "a.md"), "a\n")
	readFileEqual(t, filepath.Join(rootDir, "sec", "b.md"), "b\n")
	readFileEqual(t, filepath.Join(assetsDir, "x", "logo.svg"), "<svg/>\n")
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/welcome.md", "local welcome\n")
	assertFileInCommit(t, remote, "root/a.md", "a\n")

	settled := remote.Hash
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("second RunBackup: %v", err)
	}
	if got := remoteHeadCommit(t, bareDir).Hash; got != settled {
		t.Errorf("remote moved to %s on an idle cycle, want %s", got, settled)
	}
}

// TestPull_InvalidRemoteTreeEntry_FailsWithoutWriting tests that a remote
// tree entry whose name git's own tree-path rules refuse (a relative
// component, a backslash path, .git) fails the pull before anything is
// written: no file appears inside root/ or next to it, an existing file next
// to root/ keeps its content, HEAD stays, and the failure is logged once with
// the entry name and its parent directory. A valid remote change pulled in
// the same cycle is not written either.
func TestPull_InvalidRemoteTreeEntry_FailsWithoutWriting(t *testing.T) {
	tests := []struct {
		name      string
		top       string // content directory in the repository; "" means root
		parent    string
		entry     string
		files     map[string]string
		validFile string // repository path of a valid file the remote adds first
	}{
		{
			name:  "dot-dot directory",
			entry: "..",
			files: map[string]string{"outside.txt": "new", "existing.txt": "changed"},
		},
		{
			name:   "nested dot-dot directory",
			parent: "sub",
			entry:  "..",
			files:  map[string]string{"outside.txt": "new"},
		},
		{name: "backslash dot-dot file", entry: `..\outside.txt`},
		{name: "backslash dot-dot in the middle", entry: `x\..\..\outside.txt`},
		{name: "dot git directory", entry: ".git", files: map[string]string{"config": "new"}},
		{
			name:   "deeply nested dot-dot directory",
			parent: "a/b",
			entry:  "..",
			files:  map[string]string{"outside.txt": "new"},
		},
		{
			name:  "dot-dot directory under assets",
			top:   "assets",
			entry: "..",
			files: map[string]string{"outside.txt": "new", "existing.txt": "changed"},
		},
		{name: "backslash dot-dot file under assets", top: "assets", entry: `..\outside.txt`},
		{
			name:   "nested dot-dot directory under assets",
			top:    "assets",
			parent: "pageid",
			entry:  "..",
			files:  map[string]string{"outside.txt": "new"},
		},
		{
			name:      "dot-dot directory after a valid remote change",
			entry:     "..",
			files:     map[string]string{"outside.txt": "new"},
			validFile: "root/added.md",
		},
		{
			name:      "dot-dot directory under assets after a valid root change",
			top:       "assets",
			entry:     "..",
			files:     map[string]string{"outside.txt": "new"},
			validFile: "root/added.md",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bareDir := initBareRemote(t)
			repo, rootDir := newRepoWithRemote(t, bareDir)
			dataDir := filepath.Dir(rootDir)
			writeFileHelper(t, dataDir, "existing.txt", "original")
			headBefore := headCommit(t, repo).Hash
			if tc.validFile != "" {
				commitToRemote(t, bareDir, tc.validFile, "valid\n")
			}
			top := tc.top
			if top == "" {
				top = "root"
			}
			parent := top
			if tc.parent != "" {
				parent += "/" + tc.parent
			}
			pushRawEntryToRemote(t, bareDir, rawEntry{parent: parent, name: tc.entry, files: tc.files})
			logs := captureLogs(t)

			err := repo.RunBackup()
			if err == nil {
				t.Fatal("expected RunBackup to fail on an invalid remote tree entry")
			}
			if errors.Is(err, errPullConflict) {
				t.Fatalf("an invalid entry was reported as a conflict: %v", err)
			}
			snap := repo.Status()
			wants := []string{strconv.Quote(tc.entry)}
			if tc.parent != "" {
				wants = append(wants, strconv.Quote(tc.parent))
			}
			for _, want := range wants {
				if !strings.Contains(snap.LastError, want) {
					t.Errorf("want LastError naming %s, got NeedsIntervention=%v LastError=%q", want, snap.NeedsIntervention, snap.LastError)
				}
			}
			if got := headCommit(t, repo).Hash; got != headBefore {
				t.Errorf("HEAD moved to %s after a refused pull, want %s", got, headBefore)
			}
			readFileEqual(t, filepath.Join(dataDir, "existing.txt"), "original")
			if _, err := os.Lstat(filepath.Join(dataDir, "outside.txt")); !os.IsNotExist(err) {
				t.Errorf("outside.txt was written next to root/ (err %v)", err)
			}
			entries, err := os.ReadDir(rootDir)
			if err != nil {
				t.Fatalf("ReadDir: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "page.md" {
				t.Errorf("root/ holds %v after a refused pull, want only page.md", entries)
			}
			if entries, err := os.ReadDir(filepath.Join(dataDir, "assets")); err != nil || len(entries) != 0 {
				t.Errorf("assets/ holds %v after a refused pull (err %v), want nothing", entries, err)
			}
			if n := strings.Count(logs.String(), "level=ERROR"); n != 1 || !strings.Contains(logs.String(), "remote=") {
				t.Errorf("want exactly one error log with the remote, got %d:\n%s", n, logs.String())
			}
		})
	}
}

// TestInit_FirstContactInvalidRemoteTreeEntry_FailsWithoutWriting tests the
// first-contact sync (syncContentFromRemote) against the same kind of entry:
// Init fails and nothing lands next to root/.
func TestInit_FirstContactInvalidRemoteTreeEntry_FailsWithoutWriting(t *testing.T) {
	bareDir := initBareRemote(t)
	pushFirstCommitToRemote(t, bareDir, "main", "root/page.md", "# Page\n")
	pushRawRootEntryToRemote(t, bareDir, "", "..", map[string]string{"outside.txt": "new", "existing.txt": "changed"})

	dataDir := t.TempDir()
	rootDir := filepath.Join(dataDir, "root")
	assetsDir := filepath.Join(dataDir, "assets")
	writeFileHelper(t, dataDir, "existing.txt", "original")

	_, err := Init(Config{
		RootDir: rootDir, AssetsDir: assetsDir,
		AuthorName: "Test", AuthorEmail: "t@t.com",
		Branch: "main", RemoteURL: "file://" + bareDir, SSHKey: testSSHKeyPEM,
	})
	if err == nil {
		t.Fatal("expected Init to fail on an invalid remote tree entry")
	}
	readFileEqual(t, filepath.Join(dataDir, "existing.txt"), "original")
	if _, err := os.Lstat(filepath.Join(dataDir, "outside.txt")); !os.IsNotExist(err) {
		t.Errorf("outside.txt was written next to root/ (err %v)", err)
	}
}

// TestPull_InvalidRemoteTreeEntryResetAway_NextCycleRecovers tests that a
// pull refused for an invalid entry name is not stuck once the remote branch
// is reset to the commit before that entry: the next cycle pushes the local
// edit made meanwhile and clears the error.
//
// Removing the entry in a new commit on top is not enough: go-git refuses to
// push from a history that still holds the entry, so every later push fails.
func TestPull_InvalidRemoteTreeEntryResetAway_NextCycleRecovers(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	before := remoteHeadCommit(t, bareDir).Hash
	pushRawEntryToRemote(t, bareDir, rawEntry{parent: "root/sub", name: "..", files: map[string]string{"outside.txt": "new"}})
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on an invalid remote tree entry")
	}
	if repo.Status().LastError == "" {
		t.Fatal("expected LastError after the refused pull")
	}

	writeFileHelper(t, rootDir, "page.md", "local edit\n")
	bare, err := gogit.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("PlainOpen: %v", err)
	}
	if err := bare.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), before)); err != nil {
		t.Fatalf("reset remote branch: %v", err)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after the remote was reset: %v", err)
	}
	if snap := repo.Status(); snap.NeedsIntervention || snap.LastError != "" {
		t.Errorf("NeedsIntervention=%v LastError=%q after recovery, want both cleared", snap.NeedsIntervention, snap.LastError)
	}
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/page.md", "local edit\n")
	if got := headCommit(t, repo).Hash; got != remote.Hash {
		t.Errorf("local HEAD = %s, want the remote head %s", got, remote.Hash)
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(rootDir), "outside.txt")); !os.IsNotExist(err) {
		t.Errorf("outside.txt was written next to root/ (err %v)", err)
	}
}

// TestPull_SymlinkRemoteTreeEntry_WrittenAsRegularFile tests that a symlink
// entry from the remote lands as a regular file holding the link target, so
// the pull never creates a link that points out of the content directory, and
// that the following cycles keep succeeding.
func TestPull_SymlinkRemoteTreeEntry_WrittenAsRegularFile(t *testing.T) {
	for _, target := range []string{"../outside.txt", "/etc/hostname", "sub/../../outside.txt"} {
		t.Run(target, func(t *testing.T) {
			bareDir := initBareRemote(t)
			repo, rootDir := newRepoWithRemote(t, bareDir)
			dataDir := filepath.Dir(rootDir)
			writeFileHelper(t, dataDir, "outside.txt", "original")
			pushRawEntryToRemote(t, bareDir, rawEntry{parent: "root", name: "link.md", mode: filemode.Symlink, content: target})

			if err := repo.RunBackup(); err != nil {
				t.Fatalf("RunBackup: %v", err)
			}
			info, err := os.Lstat(filepath.Join(rootDir, "link.md"))
			if err != nil {
				t.Fatalf("Lstat link.md: %v", err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("link.md has mode %v, want a regular file", info.Mode())
			}
			readFileEqual(t, filepath.Join(rootDir, "link.md"), target)
			readFileEqual(t, filepath.Join(dataDir, "outside.txt"), "original")

			writeFileHelper(t, rootDir, "page.md", "after\n")
			if err := repo.RunBackup(); err != nil {
				t.Fatalf("second RunBackup: %v", err)
			}
			assertFileInCommit(t, remoteHeadCommit(t, bareDir), "root/page.md", "after\n")
		})
	}
}

// TestPull_LongRemoteEntryNames_PulledOrFailedCleanly tests names at and past
// the usual 255-byte file name limit: a name that fits is pulled, one that
// does not fails the cycle as an error (not a conflict) without moving HEAD,
// and the backup resumes once the remote drops it.
func TestPull_LongRemoteEntryNames_PulledOrFailedCleanly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows counts the limit in UTF-16 units and the full path length")
	}
	tests := []struct {
		name    string
		entry   string
		wantErr bool
	}{
		{name: "255 ASCII bytes", entry: strings.Repeat("a", 252) + ".md"},
		{name: "254 bytes of two-byte runes", entry: strings.Repeat("ü", 125) + ".md"},
		{name: "256 ASCII bytes", entry: strings.Repeat("a", 253) + ".md", wantErr: true},
		{name: "4096 ASCII bytes", entry: strings.Repeat("a", 4093) + ".md", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bareDir := initBareRemote(t)
			repo, rootDir := newRepoWithRemote(t, bareDir)
			headBefore := headCommit(t, repo).Hash
			pushRawEntryToRemote(t, bareDir, rawEntry{parent: "root/sec", name: tc.entry, content: "long\n"})

			err := repo.RunBackup()
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("RunBackup: %v", err)
				}
				readFileEqual(t, filepath.Join(rootDir, "sec", tc.entry), "long\n")
				return
			}
			if err == nil {
				t.Fatal("expected RunBackup to fail on a name the file system cannot hold")
			}
			if errors.Is(err, errPullConflict) {
				t.Fatalf("a too long name was reported as a conflict: %v", err)
			}
			if got := headCommit(t, repo).Hash; got != headBefore {
				t.Errorf("HEAD moved to %s after a failed pull, want %s", got, headBefore)
			}

			pushRawEntryToRemote(t, bareDir, rawEntry{parent: "root/sec", name: tc.entry, remove: true})
			commitToRemote(t, bareDir, "root/ext.md", "ext\n")
			if err := repo.RunBackup(); err != nil {
				t.Fatalf("RunBackup after the remote dropped the name: %v", err)
			}
			readFileEqual(t, filepath.Join(rootDir, "ext.md"), "ext\n")
			if snap := repo.Status(); snap.LastError != "" {
				t.Errorf("LastError = %q after recovery, want it cleared", snap.LastError)
			}
		})
	}
}

// TestPull_UnusualValidRemoteNames_Pulled tests that names close to the
// refused ones, but valid git tree paths, are pulled and backed up unchanged.
func TestPull_UnusualValidRemoteNames_Pulled(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	files := map[string]string{
		"root/..foo":           "a\n",
		"root/foo..bar.md":     "b\n",
		"root/.gitignore":      "*.tmp\n",
		"root/.github/x.md":    "c\n",
		"root/git~2/y.md":      "d\n",
		"root/.gitx":           "e\n",
		"root/über ñ.md":       "f\n",
		"assets/id1/..img.png": "g\n",
	}
	commitFilesToRemote(t, bareDir, files)

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	dataDir := filepath.Dir(rootDir)
	for rel, content := range files {
		readFileEqual(t, filepath.Join(dataDir, filepath.FromSlash(rel)), content)
	}
	writeFileHelper(t, rootDir, "page.md", "after\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("second RunBackup: %v", err)
	}
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/page.md", "after\n")
	for rel, content := range files {
		assertFileInCommit(t, remote, rel, content)
	}
}
