package backup

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
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
	if snap.ConflictKind != ConflictKindDiverged {
		t.Errorf("ConflictKind = %q, want %q", snap.ConflictKind, ConflictKindDiverged)
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
	if snap.ConflictKind != ConflictKindFileConflict {
		t.Errorf("ConflictKind = %q, want %q", snap.ConflictKind, ConflictKindFileConflict)
	}
	if !strings.Contains(snap.ConflictDetails, "root/page.md") {
		t.Errorf("ConflictDetails = %q, want it to name root/page.md", snap.ConflictDetails)
	}
}

// TestConflict_LocalDeleteRemoteModify tests that RunBackup surfaces a
// conflict (NeedsIntervention) instead of silently resurrecting a file that
// was deleted locally, when the remote independently modified that same file
// before the delete was backed up.
//
// Setup:
//  1. Local repo A: page.md = "# Page\n" committed + pushed
//  2. Local on disk: page.md deleted (not committed)
//  3. External client: page.md = "version B" committed + pushed to remote
//  4. RunBackup → materializeContent must not silently rewrite page.md with
//     the remote content; the local deletion must not be reverted without the
//     operator being told about the conflict.
func TestConflict_LocalDeleteRemoteModify(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	pagePath := filepath.Join(rootDir, "page.md")

	if err := os.Remove(pagePath); err != nil {
		t.Fatalf("Remove page.md: %v", err)
	}

	commitToRemote(t, bareDir, "root/page.md", "version B from remote\n")

	err := repo.RunBackup()
	if err == nil {
		t.Fatal("expected RunBackup to return an error on a delete/modify conflict")
	}

	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after a delete/modify conflict")
	}
	if _, statErr := os.Stat(pagePath); !os.IsNotExist(statErr) {
		t.Errorf("expected the local deletion of page.md to be preserved, stat err = %v", statErr)
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
		RemoteURL:   test_utils.FileURL(bareDir),
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

// incomingPages are pages an external client adds to the remote in the same
// pull as a conflicting edit. Several of them, because the pull visits paths
// in map order: with only one, a half-applying pull could pass by luck.
var incomingPages = map[string]string{
	"root/a.md": "a\n",
	"root/b.md": "b\n",
	"root/c.md": "c\n",
	"root/d.md": "d\n",
	"root/e.md": "e\n",
}

// TestConflict_FileConflict_LeavesLiveDirUntouched tests that a file conflict
// stops the pull before it writes anything: none of the remote's other changes
// land on disk and the local branch stays where it was, so a later Force Push
// cannot mistake half-applied pages for local content.
func TestConflict_FileConflict_LeavesLiveDirUntouched(t *testing.T) {
	for i := range 10 {
		bareDir := initBareRemote(t)
		repo, rootDir := newRepoWithRemote(t, bareDir)
		headBefore := headCommit(t, repo).Hash

		incoming := map[string]string{"root/page.md": "version B from remote\n"}
		for p, c := range incomingPages {
			incoming[p] = c
		}
		commitFilesToRemote(t, bareDir, incoming)
		writeFileHelper(t, rootDir, "page.md", "version C local\n")

		err := repo.RunBackup()
		if !errors.Is(err, errPullConflict) {
			t.Fatalf("run %d: RunBackup err = %v, want errPullConflict", i, err)
		}
		if !repo.Status().NeedsIntervention {
			t.Errorf("run %d: expected NeedsIntervention = true", i)
		}
		for p := range incomingPages {
			if _, statErr := os.Stat(filepath.Join(filepath.Dir(rootDir), filepath.FromSlash(p))); !os.IsNotExist(statErr) {
				t.Errorf("run %d: %s was written despite the conflict (stat err = %v)", i, p, statErr)
			}
		}
		readFileEqual(t, filepath.Join(rootDir, "page.md"), "version C local\n")
		if got := headCommit(t, repo).Hash; got != headBefore {
			t.Errorf("run %d: HEAD moved to %s, want it to stay at %s", i, got, headBefore)
		}
	}
}

// TestConflict_ConflictOnlyInAssets_LeavesRootUntouched tests that the pull
// checks every content directory before it writes to any of them: a conflict
// under assets/ also keeps the remote's root/ changes off the disk.
func TestConflict_ConflictOnlyInAssets_LeavesRootUntouched(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	assetsDir := repo.cfg.AssetsDir
	writeFileHelper(t, assetsDir, "logo.svg", "<svg/>\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup (commit asset): %v", err)
	}

	commitFilesToRemote(t, bareDir, map[string]string{
		"root/from-remote.md": "remote page\n",
		"assets/logo.svg":     "<svg>remote</svg>\n",
	})
	writeFileHelper(t, assetsDir, "logo.svg", "<svg>local</svg>\n")

	if err := repo.RunBackup(); !errors.Is(err, errPullConflict) {
		t.Fatalf("RunBackup err = %v, want errPullConflict", err)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "from-remote.md")); !os.IsNotExist(err) {
		t.Errorf("root/from-remote.md was written despite the assets conflict (stat err = %v)", err)
	}
	readFileEqual(t, filepath.Join(assetsDir, "logo.svg"), "<svg>local</svg>\n")
}

// TestConflict_ResolvedByMatchingRemote_Clears tests that a conflict clears
// once the live file equals the remote version: a file that already holds
// what the remote has is applied, not a conflict, so the next cycle pulls the
// rest and pushes.
func TestConflict_ResolvedByMatchingRemote_Clears(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitFilesToRemote(t, bareDir, map[string]string{
		"root/page.md":        "version B from remote\n",
		"root/from-remote.md": "remote page\n",
	})
	writeFileHelper(t, rootDir, "page.md", "version C local\n")
	if err := repo.RunBackup(); !errors.Is(err, errPullConflict) {
		t.Fatalf("first RunBackup err = %v, want errPullConflict", err)
	}

	// The admin resolves the conflict by adopting the remote version.
	writeFileHelper(t, rootDir, "page.md", "version B from remote\n")

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after adopting the remote version: %v", err)
	}
	if snap := repo.Status(); snap.NeedsIntervention || snap.LastError != "" {
		t.Errorf("expected a clean status, got NeedsIntervention=%v LastError=%q", snap.NeedsIntervention, snap.LastError)
	}
	readFileEqual(t, filepath.Join(rootDir, "from-remote.md"), "remote page\n")
	remote := remoteHeadCommit(t, bareDir)
	assertFileInCommit(t, remote, "root/page.md", "version B from remote\n")
	assertFileInCommit(t, remote, "root/from-remote.md", "remote page\n")
}

// TestConflict_RemoteAhead_RemoteTreeKeepsIncomingPages tests that a pull of
// remote-only pages next to an unrelated local edit keeps both on the remote:
// the backup commit must not drop the pages it just pulled.
func TestConflict_RemoteAhead_RemoteTreeKeepsIncomingPages(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitFilesToRemote(t, bareDir, incomingPages)
	writeFileHelper(t, rootDir, "page.md", "local edit\n")

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	remote := remoteHeadCommit(t, bareDir)
	for p, want := range incomingPages {
		assertFileInCommit(t, remote, p, want)
	}
	assertFileInCommit(t, remote, "root/page.md", "local edit\n")
}

// TestConflict_MixedDeletesAndWrites_ConflictElsewhere_LeavesLiveDirUntouched
// tests that a conflict on one file also holds back the remote's deletes and
// its writes into nested and Unicode-named directories, and that once the
// local edit is reverted the next cycle applies all of them at once.
func TestConflict_MixedDeletesAndWrites_ConflictElsewhere_LeavesLiveDirUntouched(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	writeFileHelper(t, rootDir, "gone.md", "to be deleted\n")
	writeFileHelper(t, rootDir, "sec/nested.md", "nested base\n")
	writeFileHelper(t, rootDir, "sec/only/child.md", "only child\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup (seed): %v", err)
	}
	headBefore := headCommit(t, repo).Hash

	commitChangesToRemote(t, bareDir, map[string]string{
		"root/page.md":        "remote page\n",
		"root/sec/nested.md":  "nested remote\n",
		"root/über/日本/新しい.md": "unicode remote\n",
	}, []string{"root/gone.md", "root/sec/only/child.md"})
	writeFileHelper(t, rootDir, "page.md", "local page\n")

	err := repo.RunBackup()
	var conflict *pullConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RunBackup err = %v, want *pullConflictError", err)
	}
	if want := []string{"root/page.md"}; !reflect.DeepEqual(conflict.Paths, want) {
		t.Errorf("conflict paths = %v, want %v", conflict.Paths, want)
	}
	readFileEqual(t, filepath.Join(rootDir, "gone.md"), "to be deleted\n")
	readFileEqual(t, filepath.Join(rootDir, "sec", "nested.md"), "nested base\n")
	readFileEqual(t, filepath.Join(rootDir, "sec", "only", "child.md"), "only child\n")
	if _, statErr := os.Stat(filepath.Join(rootDir, "über")); !os.IsNotExist(statErr) {
		t.Errorf("über/ was created despite the conflict (stat err = %v)", statErr)
	}
	if got := headCommit(t, repo).Hash; got != headBefore {
		t.Errorf("HEAD moved to %s, want it to stay at %s", got, headBefore)
	}

	// Revert the local edit; the next cycle applies everything.
	writeFileHelper(t, rootDir, "page.md", "# Page\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after reverting the local edit: %v", err)
	}
	readFileEqual(t, filepath.Join(rootDir, "page.md"), "remote page\n")
	readFileEqual(t, filepath.Join(rootDir, "sec", "nested.md"), "nested remote\n")
	readFileEqual(t, filepath.Join(rootDir, "über", "日本", "新しい.md"), "unicode remote\n")
	if _, statErr := os.Stat(filepath.Join(rootDir, "gone.md")); !os.IsNotExist(statErr) {
		t.Errorf("gone.md still present after the pull (stat err = %v)", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(rootDir, "sec", "only")); !os.IsNotExist(statErr) {
		t.Errorf("sec/only/ should be removed once its last file is deleted (stat err = %v)", statErr)
	}
	if snap := repo.Status(); snap.NeedsIntervention || snap.LastError != "" {
		t.Errorf("expected a clean status, got NeedsIntervention=%v LastError=%q", snap.NeedsIntervention, snap.LastError)
	}
}

// TestConflict_SeveralFilesInBothTargets_AllReported tests that the conflict
// error names every conflicting file in root/ and assets/, not just the first
// one the pull happens to visit.
func TestConflict_SeveralFilesInBothTargets_AllReported(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	assetsDir := repo.cfg.AssetsDir
	writeFileHelper(t, rootDir, "other.md", "other base\n")
	writeFileHelper(t, assetsDir, "logo.svg", "<svg/>\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup (seed): %v", err)
	}

	commitFilesToRemote(t, bareDir, map[string]string{
		"root/page.md":    "remote page\n",
		"root/other.md":   "remote other\n",
		"assets/logo.svg": "<svg>remote</svg>\n",
	})
	writeFileHelper(t, rootDir, "page.md", "local page\n")
	writeFileHelper(t, rootDir, "other.md", "local other\n")
	writeFileHelper(t, assetsDir, "logo.svg", "<svg>local</svg>\n")

	err := repo.RunBackup()
	var conflict *pullConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("RunBackup err = %v, want *pullConflictError", err)
	}
	got := append([]string(nil), conflict.Paths...)
	sort.Strings(got)
	want := []string{"assets/logo.svg", "root/other.md", "root/page.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("conflict paths = %v, want %v", got, want)
	}
	if !repo.Status().NeedsIntervention {
		t.Error("expected NeedsIntervention = true")
	}
}
