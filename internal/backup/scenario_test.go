package backup

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScenario_FreshInstanceSyncsInsteadOfWiping_WithPath is the prefix
// variant: content synced from <path>/root must land in the live root dir.
func TestScenario_FreshInstanceSyncsInsteadOfWiping_WithPath(t *testing.T) {
	bareDir := initBareRemote(t)

	repoA, rootA := newRepoWithRemotePath(t, bareDir, "docs")
	writeFileHelper(t, rootA, "guide.md", "guide\n")
	if err := repoA.RunBackup(); err != nil {
		t.Fatalf("repoA.RunBackup: %v", err)
	}

	tmpDir := t.TempDir()
	rootB := filepath.Join(tmpDir, "root")
	assetsB := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assetsB, 0755); err != nil {
		t.Fatal(err)
	}
	writeFileHelper(t, rootB, "welcome-to-leafwiki.md", "welcome\n")

	repoB, err := Init(Config{
		RootDir:     rootB,
		AssetsDir:   assetsB,
		Path:        "docs",
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init B: %v", err)
	}

	// Init synced docs/root/* into the live root dir.
	readFileEqual(t, filepath.Join(rootB, "guide.md"), "guide\n")
	readFileEqual(t, filepath.Join(rootB, "page.md"), "# Page\n")

	if err := repoB.RunBackup(); err != nil {
		t.Fatalf("repoB.RunBackup: %v", err)
	}

	commit := headCommit(t, repoB)
	assertFileInCommit(t, commit, "docs/root/guide.md", "guide\n")
	assertFileInCommit(t, commit, "docs/root/page.md", "# Page\n")
	assertFileInCommit(t, commit, "docs/root/welcome-to-leafwiki.md", "welcome\n")
}

// TestScenario_FreshInstanceSyncsInsteadOfWiping seeds a remote with a prior
// backup (several documents) and then starts a fresh instance whose live content
// is just the default welcome page, pointing at the same remote.
//
// The first Init syncs the remote content locally, so RunBackup must not delete
// the documents that only exist on the remote, and local files are never
// overwritten.
func TestScenario_FreshInstanceSyncsInsteadOfWiping(t *testing.T) {
	bareDir := initBareRemote(t)

	// Instance A backs up two documents.
	repoA, rootA := newRepoWithRemote(t, bareDir)
	if err := os.WriteFile(filepath.Join(rootA, "guide.md"), []byte("guide\n"), 0644); err != nil {
		t.Fatalf("WriteFile guide.md: %v", err)
	}
	if err := repoA.RunBackup(); err != nil {
		t.Fatalf("repoA.RunBackup: %v", err)
	}

	// Instance B: fresh data dir with the default welcome page, plus a local edit
	// to a document that also exists on the remote (must win locally).
	tmpDir := t.TempDir()
	rootB := filepath.Join(tmpDir, "root")
	assetsB := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(assetsB, 0755); err != nil {
		t.Fatal(err)
	}
	writeFileHelper(t, rootB, "welcome-to-leafwiki.md", "welcome\n")
	writeFileHelper(t, rootB, "page.md", "local wins\n")

	repoB, err := Init(Config{
		RootDir:     rootB,
		AssetsDir:   assetsB,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init B: %v", err)
	}

	// Init syncs the remote content locally (without clobbering local files).
	readFileEqual(t, filepath.Join(rootB, "guide.md"), "guide\n")
	readFileEqual(t, filepath.Join(rootB, "page.md"), "local wins\n")

	if err := repoB.RunBackup(); err != nil {
		t.Fatalf("repoB.RunBackup: %v", err)
	}

	commit := headCommit(t, repoB)
	// Remote-only documents were synced, not deleted.
	assertFileInCommit(t, commit, "root/guide.md", "guide\n")
	// The local edit is preserved.
	assertFileInCommit(t, commit, "root/page.md", "local wins\n")
	// The instance's own file is kept.
	assertFileInCommit(t, commit, "root/welcome-to-leafwiki.md", "welcome\n")
}

func readFileEqual(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s = %q, want %q", path, string(got), want)
	}
}
