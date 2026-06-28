package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// --- Init edge cases ---

func TestInit_EmptyRootDir_NoInitialCommit(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	}
	_, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// No files → no initial commit → HEAD must not exist
	r, _ := gogit.PlainOpen(tmpDir)
	_, err = r.Head()
	if err == nil {
		t.Error("expected no HEAD when both dirs are empty")
	}
}

func TestInit_OnlyAssetsDirHasFiles_InitialCommitCreated(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "img.png"), []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile img.png: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	}
	_, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, _ := gogit.PlainOpen(tmpDir)
	head, err := r.Head()
	if err != nil {
		t.Fatalf("expected initial commit, got no HEAD: %v", err)
	}
	commit, _ := r.CommitObject(head.Hash())
	tree, _ := commit.Tree()
	if _, err := tree.File("assets/img.png"); err != nil {
		t.Error("expected assets/img.png in initial commit tree")
	}
}

func TestInit_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}

	r1, err := Init(cfg)
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	r2, err := Init(cfg)
	if err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	if r1 == nil || r2 == nil {
		t.Fatal("expected non-nil repos")
	}

	// Commit count must not have changed
	repo, _ := gogit.PlainOpen(tmpDir)
	head, _ := repo.Head()
	commit, _ := repo.CommitObject(head.Hash())
	if commit.NumParents() != 0 {
		t.Error("expected exactly one commit after two Init calls on same dir")
	}
}

func TestInit_ReconcileRemote_UpdatesURL(t *testing.T) {
	bareDir1 := initBareRemote(t)
	bareDir2 := initBareRemote(t)

	// newRepoWithRemote does Init + RunBackup, which pushes to bareDir1 and
	// creates the "origin" remote in the local repo.
	repo, _ := newRepoWithRemote(t, bareDir1)
	tmpDir := repo.repoDir

	// Re-init pointing at a different remote — reconcileRemote should update origin.
	cfg := repo.cfg
	cfg.RemoteURL = "file://" + bareDir2
	if _, err := Init(cfg); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	gitRepo, _ := gogit.PlainOpen(tmpDir)
	remote, err := gitRepo.Remote("origin")
	if err != nil {
		t.Fatalf("remote 'origin' not found: %v", err)
	}
	if urls := remote.Config().URLs; len(urls) == 0 || urls[0] != "file://"+bareDir2 {
		t.Errorf("expected remote URL %q, got %v", "file://"+bareDir2, urls)
	}
}

// --- RunBackup edge cases ---

func TestRunBackup_DeletedFile_IsCommitted(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "delete-me.md"), []byte("bye\n"), 0644); err != nil {
		t.Fatalf("WriteFile delete-me.md: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)
	if err := repo.RunBackup(); err != nil { // commit the file first
		t.Fatalf("initial RunBackup failed: %v", err)
	}

	// Now delete the file and run backup again
	if err := os.Remove(filepath.Join(rootDir, "delete-me.md")); err != nil {
		t.Fatalf("Remove delete-me.md: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after deletion failed: %v", err)
	}

	r, _ := gogit.PlainOpen(tmpDir)
	head, _ := r.Head()
	commit, _ := r.CommitObject(head.Hash())
	tree, _ := commit.Tree()
	if _, err := tree.File("root/delete-me.md"); err == nil {
		t.Error("expected delete-me.md to be absent from latest commit tree")
	}
}

func TestRunBackup_SecondRunWithoutChanges_SameHeadHash(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("initial RunBackup failed: %v", err)
	}

	r, _ := gogit.PlainOpen(tmpDir)
	head1, _ := r.Head()

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("second RunBackup failed: %v", err)
	}

	head2, _ := r.Head()
	if head1.Hash() != head2.Hash() {
		t.Error("expected no new commit when working tree is unchanged")
	}
}

func TestRunBackup_CommitMessageFormat(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)

	before := time.Now().Truncate(time.Second)
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}
	after := time.Now().Add(time.Second)

	r, _ := gogit.PlainOpen(tmpDir)
	head, _ := r.Head()
	commit, _ := r.CommitObject(head.Hash())

	if !strings.HasPrefix(commit.Message, "backup: ") {
		t.Errorf("expected commit message to start with 'backup: ', got %q", commit.Message)
	}
	ts, err := time.Parse(time.RFC3339, strings.TrimPrefix(commit.Message, "backup: "))
	if err != nil {
		t.Fatalf("commit message timestamp not RFC3339: %q", commit.Message)
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("commit timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

func TestRunBackup_StatusLastBackupAtSet(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)

	before := time.Now().Add(-time.Second)
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	snap := repo.Status()
	if snap.LastBackupAt == nil {
		t.Fatal("expected LastBackupAt to be set after RunBackup")
	}
	if snap.LastBackupAt.Before(before) {
		t.Errorf("LastBackupAt %v is before RunBackup was called", snap.LastBackupAt)
	}
	if snap.LastError != "" {
		t.Errorf("expected no error, got %q", snap.LastError)
	}
}

func TestRunBackup_AssetsFileCommitted(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)

	if err := os.WriteFile(filepath.Join(assetsDir, "img.png"), []byte("imgdata"), 0644); err != nil {
		t.Fatalf("WriteFile img.png: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	r, _ := gogit.PlainOpen(tmpDir)
	head, _ := r.Head()
	commit, _ := r.CommitObject(head.Hash())
	tree, _ := commit.Tree()
	if _, err := tree.File("assets/img.png"); err != nil {
		t.Error("expected assets/img.png to be in commit tree")
	}
}

func TestRunBackup_UntrackedFileOutsideDirs_NotCommitted(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}

	// File sits in tmpDir itself (the repo root), outside root/ and assets/
	if err := os.WriteFile(filepath.Join(tmpDir, "app.db"), []byte("database"), 0644); err != nil {
		t.Fatalf("WriteFile app.db: %v", err)
	}

	cfg := Config{RootDir: rootDir, AssetsDir: assetsDir, AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main"}
	repo, _ := Init(cfg)

	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# P\n"), 0644); err != nil {
		t.Fatalf("WriteFile page.md: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	r, _ := gogit.PlainOpen(tmpDir)
	head, _ := r.Head()
	commit, _ := r.CommitObject(head.Hash())
	tree, _ := commit.Tree()
	if _, err := tree.File("app.db"); err == nil {
		t.Error("expected app.db (outside root/ and assets/) to NOT be in commit tree")
	}
}

// commitDirectlyOnRepo makes a git commit bypassing RunBackup — used to create
// locally diverged state in conflict tests.
func commitDirectlyOnRepo(t *testing.T, repo *Repository, rootDir, filename, content string) {
	t.Helper()
	p := filepath.Join(rootDir, filename)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("commitDirectlyOnRepo: WriteFile: %v", err)
	}
	wt, err := repo.repo.Worktree()
	if err != nil {
		t.Fatalf("commitDirectlyOnRepo: Worktree: %v", err)
	}
	rel, err := filepath.Rel(repo.repoDir, p)
	if err != nil {
		t.Fatalf("commitDirectlyOnRepo: Rel: %v", err)
	}
	if _, err := wt.Add(filepath.ToSlash(rel)); err != nil {
		t.Fatalf("commitDirectlyOnRepo: Add: %v", err)
	}
	if _, err := wt.Commit("local diverged commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@t.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commitDirectlyOnRepo: Commit: %v", err)
	}
}
