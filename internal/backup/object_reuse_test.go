package backup

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

func countLooseObjects(t *testing.T, r *Repository) int {
	t.Helper()
	los, ok := r.repo.Storer.(storer.LooseObjectStorer)
	if !ok {
		t.Fatal("storer does not support loose object enumeration")
	}
	n := 0
	if err := los.ForEachObjectHash(func(plumbing.Hash) error {
		n++
		return nil
	}); err != nil {
		t.Fatalf("ForEachObjectHash: %v", err)
	}
	return n
}

// An unchanged backup run must not write any git objects again: everything
// it hashes already exists (here: packed by gc), so re-writing it as loose
// objects only costs CPU and disk writes on every interval.
func TestRunBackup_UnchangedContent_WritesNoNewObjects(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	writeFileHelper(t, rootDir, "second.md", "second\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	if !repo.gc() {
		t.Fatal("gc failed")
	}
	if n := countLooseObjects(t, repo); n != 0 {
		t.Fatalf("expected no loose objects after gc, got %d", n)
	}

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup (unchanged): %v", err)
	}
	if n := countLooseObjects(t, repo); n != 0 {
		t.Errorf("unchanged RunBackup wrote %d loose objects, want 0", n)
	}
}

// A changed file still produces exactly the new objects it needs.
func TestRunBackup_ChangedFile_WritesOnlyNewObjects(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	writeFileHelper(t, rootDir, "untouched.md", "untouched\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	if !repo.gc() {
		t.Fatal("gc failed")
	}

	writeFileHelper(t, rootDir, "page.md", "# Page\nedited\n")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}
	// new blob + root/ tree + top-level tree + commit
	if n := countLooseObjects(t, repo); n != 4 {
		t.Errorf("expected 4 new loose objects for one edited file, got %d", n)
	}
	assertFileInCommit(t, headCommit(t, repo), "root/page.md", "# Page\nedited\n")
}
