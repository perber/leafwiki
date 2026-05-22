package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func TestInit_InitializesNewRepo(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	// Create root and assets directories
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	// Create a test file in root
	testFile := filepath.Join(rootDir, "test.md")
	err = os.WriteFile(testFile, []byte("# Test\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if repo == nil {
		t.Fatal("Init returned nil repo")
	}

	// Verify the repo exists
	r, err := git.PlainOpen(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// Verify there's at least one commit
	head, err := r.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	if head.Type() != plumbing.HashReference {
		t.Errorf("expected HEAD to be a hash reference (branch), got %v", head.Type())
	}
	// go-git may create "master" or "main" depending on version/config
	// Just verify we have a branch head
	branchName := head.Name().Short()
	if branchName != "main" && branchName != "master" {
		t.Errorf("expected HEAD to be main or master branch, got %s", branchName)
	}
}

func TestInit_OpensExistingRepo(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	// Create directories
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	// Initialize a git repo manually
	r, err := git.PlainInit(tmpDir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	_ = r

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if repo == nil {
		t.Fatal("Init returned nil repo")
	}

	// Verify repo is still valid
	_, err = git.PlainOpen(tmpDir)
	if err != nil {
		t.Fatalf("failed to open existing repo: %v", err)
	}
}

func TestRunBackup_NothingToCommit(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	// Create directories
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Run backup on clean working tree
	err = repo.RunBackup()
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	// Verify status is clean
	status := repo.Status()
	if status.LastError != "" {
		t.Errorf("expected no error, got %s", status.LastError)
	}
}

func TestRunBackup_StagesAndCommits(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	// Create directories
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write a file to root/
	testFile := filepath.Join(rootDir, "new.md")
	err = os.WriteFile(testFile, []byte("# New File\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run backup
	err = repo.RunBackup()
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	// Verify the file was committed
	r, err := git.PlainOpen(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	head, err := r.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	if commit.Author.Name != "Test Author" {
		t.Errorf("expected author name 'Test Author', got %s", commit.Author.Name)
	}
}

func TestRunBackup_OnlyStagedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	otherDir := filepath.Join(tmpDir, "other")

	// Create directories
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}
	err = os.MkdirAll(otherDir, 0755)
	if err != nil {
		t.Fatalf("failed to create other dir: %v", err)
	}

	// Write a file outside root/ and assets/
	otherFile := filepath.Join(otherDir, "outside.txt")
	err = os.WriteFile(otherFile, []byte("should not be committed\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write other file: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write a file to root/
	testFile := filepath.Join(rootDir, "new.md")
	err = os.WriteFile(testFile, []byte("# New File\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Run backup
	err = repo.RunBackup()
	if err != nil {
		t.Fatalf("RunBackup failed: %v", err)
	}

	// Verify the commit only contains files from root/ and assets/
	r, err := git.PlainOpen(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	head, err := r.Head()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}

	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("failed to get commit: %v", err)
	}

	// Check that "outside.txt" is not in the commit
	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("failed to get tree: %v", err)
	}

	_, err = tree.File("other/outside.txt")
	if err == nil {
		t.Error("expected 'other/outside.txt' to not be in commit tree")
	}

	// Verify the file from root/ is present
	_, err = tree.File("root/new.md")
	if err != nil {
		t.Errorf("expected 'root/new.md' to be in commit tree: %v", err)
	}
}

func TestInit_RequiresRootDir(t *testing.T) {
	cfg := Config{
		RootDir:   "",
		AssetsDir: "/some/path",
	}
	_, err := Init(cfg)
	if err == nil {
		t.Error("expected error for empty RootDir")
	}
}

func TestInit_RequiresAssetsDir(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}

	cfg := Config{
		RootDir:   rootDir,
		AssetsDir: "",
	}
	_, err = Init(cfg)
	if err == nil {
		t.Error("expected error for empty AssetsDir")
	}
}

func TestInit_RequiresAuthorName(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "",
		AuthorEmail: "test@example.com",
	}
	_, err = Init(cfg)
	if err == nil {
		t.Error("expected error for empty AuthorName")
	}
}

func TestInit_RequiresAuthorEmail(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "",
	}
	_, err = Init(cfg)
	if err == nil {
		t.Error("expected error for empty AuthorEmail")
	}
}