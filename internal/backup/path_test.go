package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestValidateBackupPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"", false},
		{"docs/wiki", false},
		{"docs\\wiki", false},
		{"/abs", true},
		{"../up", true},
		{"docs/../wiki", true},
		{".", true},
	}
	for _, tc := range tests {
		err := validateBackupPath(tc.path)
		if tc.wantErr && err == nil {
			t.Fatalf("validateBackupPath(%q) expected error", tc.path)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("validateBackupPath(%q) unexpected error: %v", tc.path, err)
		}
	}
}

func TestInit_WithPath_UsesGitBackupDirAndNestedTree(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "a.txt"), []byte("asset"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := Init(Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		Path:        "docs/wiki",
		AuthorName:  "T",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	wantRepoDir := filepath.Join(tmpDir, ".git-backup")
	if repo.repoDir != wantRepoDir {
		t.Fatalf("repoDir = %q, want %q", repo.repoDir, wantRepoDir)
	}
	if _, err := os.Stat(filepath.Join(wantRepoDir, "docs", "wiki", "root", "page.md")); err != nil {
		t.Fatalf("expected mirrored page: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected no in-place .git under data dir, err=%v", err)
	}

	head, err := repo.repo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	commit, err := repo.repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("CommitObject: %v", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	found := false
	_ = tree.Files().ForEach(func(f *object.File) error {
		if strings.ReplaceAll(f.Name, "\\", "/") == "docs/wiki/root/page.md" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("expected docs/wiki/root/page.md in commit tree")
	}
}

func TestRunBackup_WithPath_PreservesSiblingMonorepoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, ".git-backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "docs", "wiki", "root"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, "README.md"), []byte("monorepo"), 0o644); err != nil {
		t.Fatal(err)
	}

	gRepo, err := gogit.PlainInit(backupRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := gRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("README.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Commit("seed", &gogit.CommitOptions{
		Author: &object.Signature{Name: "T", Email: "t@t.com"},
	}); err != nil {
		t.Fatal(err)
	}

	repo, err := Init(Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		Path:        "docs/wiki",
		AuthorName:  "T",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(backupRoot, "README.md")); err != nil {
		t.Fatalf("expected monorepo README to remain: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(backupRoot, "docs", "wiki", "root", "page.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "v1" {
		t.Fatalf("mirrored page = %q, want v1", string(b))
	}
}
