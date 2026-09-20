package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestNormalizeBackupPath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"docs/wiki", "docs/wiki", false},
		{"docs\\wiki", "docs/wiki", false},
		{"  docs/wiki  ", "docs/wiki", false},
		{"docs//wiki", "docs/wiki", false},
		{"docs/./wiki", "docs/wiki", false},
		{"/abs", "", true},
		{"../up", "", true},
		{"docs/../wiki", "", true},
		{".", "", true},
	}
	for _, tc := range cases {
		got, err := normalizeBackupPath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("normalizeBackupPath(%q) expected error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("normalizeBackupPath(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeBackupPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// newRepoWithRemotePath is newRepoWithRemote with a monorepo prefix configured.
func newRepoWithRemotePath(t *testing.T, bareDir, prefix string) (*Repository, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("newRepoWithRemotePath: MkdirAll root: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("newRepoWithRemotePath: MkdirAll assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("newRepoWithRemotePath: WriteFile: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		Path:        prefix,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	}
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("newRepoWithRemotePath: Init failed: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("newRepoWithRemotePath: initial RunBackup failed: %v", err)
	}
	return repo, rootDir
}

func headCommit(t *testing.T, repo *Repository) *object.Commit {
	t.Helper()
	head, err := repo.repo.Head()
	if err != nil {
		t.Fatalf("headCommit: Head: %v", err)
	}
	commit, err := repo.repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("headCommit: CommitObject: %v", err)
	}
	return commit
}

func assertFileInCommit(t *testing.T, commit *object.Commit, path, want string) {
	t.Helper()
	file, err := commit.File(path)
	if err != nil {
		t.Fatalf("expected %q in commit tree: %v", path, err)
	}
	content, err := file.Contents()
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if content != want {
		t.Fatalf("%s = %q, want %q", path, content, want)
	}
}

// TestInit_WithPath_CommitsUnderPrefix checks that with a prefix set the content
// lands under <prefix>/root and <prefix>/assets, while the live files stay in
// the data directory and nothing is written at the repository top level.
func TestInit_WithPath_CommitsUnderPrefix(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	if err := os.WriteFile(filepath.Join(repo.cfg.AssetsDir, "img.png"), []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile asset: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	commit := headCommit(t, repo)
	assertFileInCommit(t, commit, "docs/wiki/root/page.md", "# Page\n")
	assertFileInCommit(t, commit, "docs/wiki/assets/img.png", "data")

	// The live files are untouched at the data dir root.
	if _, err := os.Stat(filepath.Join(rootDir, "page.md")); err != nil {
		t.Fatalf("expected live page to remain at %s: %v", rootDir, err)
	}

	// Nothing is committed at the repository top level.
	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if findTreeEntry(tree.Entries, "root") != nil {
		t.Error("did not expect a top-level root/ entry when a prefix is configured")
	}
	if findTreeEntry(tree.Entries, "assets") != nil {
		t.Error("did not expect a top-level assets/ entry when a prefix is configured")
	}
}

// TestInit_WithPath_SingleSegment checks that a single-segment path puts the
// content directories directly under it: path "docs" → docs/root + docs/assets.
func TestInit_WithPath_SingleSegment(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, _ := newRepoWithRemotePath(t, bareDir, "docs")

	writeFileHelper(t, repo.cfg.AssetsDir, "img.png", "data")
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	commit := headCommit(t, repo)
	assertFileInCommit(t, commit, "docs/root/page.md", "# Page\n")
	assertFileInCommit(t, commit, "docs/assets/img.png", "data")
}

// TestRunBackup_WithPath_PreservesSiblingRemoteFiles checks that the plumbing
// splice keeps monorepo siblings committed on the remote intact.
func TestRunBackup_WithPath_PreservesSiblingRemoteFiles(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	// External client adds a sibling file at the repository top level.
	commitToRemote(t, bareDir, "README.md", "monorepo readme\n")

	if err := os.WriteFile(filepath.Join(rootDir, "local.md"), []byte("local\n"), 0644); err != nil {
		t.Fatalf("WriteFile local.md: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	commit := headCommit(t, repo)
	assertFileInCommit(t, commit, "README.md", "monorepo readme\n")
	assertFileInCommit(t, commit, "docs/wiki/root/page.md", "# Page\n")
	assertFileInCommit(t, commit, "docs/wiki/root/local.md", "local\n")
}

// TestPull_WithPath_MaterializesRemoteContentIntoLiveDirs checks that an
// external edit to <prefix>/root is pulled back into the live content dir.
func TestPull_WithPath_MaterializesRemoteContentIntoLiveDirs(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	commitToRemote(t, bareDir, "docs/wiki/root/from-remote.md", "remote content\n")

	if err := repo.Pull(); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(rootDir, "from-remote.md"))
	if err != nil {
		t.Fatalf("expected pulled file at %s: %v", rootDir, err)
	}
	if string(got) != "remote content\n" {
		t.Fatalf("pulled content = %q, want %q", string(got), "remote content\n")
	}
	if snap := repo.Status(); snap.NeedsIntervention {
		t.Error("expected no intervention needed after clean fast-forward")
	}
}

// TestRunBackup_WithPath_ConflictOnDirtyLocalFile checks that a remote change to
// a file that is also dirty on disk is surfaced as NeedsIntervention instead of
// silently overwriting the local edit.
func TestRunBackup_WithPath_ConflictOnDirtyLocalFile(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	commitToRemote(t, bareDir, "docs/wiki/root/page.md", "version B from remote\n")
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on a content conflict")
	}
	snap := repo.Status()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after content conflict")
	}
	if snap.ConflictDetails == "" {
		t.Error("expected ConflictDetails to be set")
	}
}

// TestRunBackup_WithPath_DivergedHistory checks that diverging local and remote
// histories are detected with a prefix configured.
func TestRunBackup_WithPath_DivergedHistory(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	commitToRemote(t, bareDir, "docs/wiki/root/remote-only.md", "from remote\n")
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")

	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on diverged history")
	}
	if snap := repo.Status(); !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true after diverged history")
	}
}

// TestRunBackup_IgnoresGitignoredFiles checks that the plumbing commit honours
// .gitignore rules: the generated root .gitignore (transient atomic-write files
// and runtime files), a user-added root rule, and a nested .gitignore.
func TestRunBackup_IgnoresGitignoredFiles(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	// Transient atomic-write file, matched by the generated repo .gitignore.
	writeFileHelper(t, rootDir, ".tmp-abc123", "partial write")
	// User-added repo-root rule.
	appendFile(t, filepath.Join(repo.repoDir, ".gitignore"), "*.draft\n")
	writeFileHelper(t, rootDir, "note.draft", "draft")
	// Nested .gitignore scoped to the content directory.
	writeFileHelper(t, rootDir, ".gitignore", "*.secret\n")
	writeFileHelper(t, rootDir, "drop.secret", "secret")
	writeFileHelper(t, rootDir, "keep.md", "keep\n")

	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	commit := headCommit(t, repo)
	for _, ignored := range []string{
		"docs/wiki/root/.tmp-abc123",
		"docs/wiki/root/note.draft",
		"docs/wiki/root/drop.secret",
	} {
		if _, err := commit.File(ignored); err == nil {
			t.Errorf("expected %q to be ignored, but it is in the commit tree", ignored)
		}
	}
	assertFileInCommit(t, commit, "docs/wiki/root/keep.md", "keep\n")
}

// TestRunBackup_PathCollidesWithExistingFile checks that RunBackup fails
// loudly instead of silently dropping a pre-existing top-level file when the
// configured content path collides with it (spliceTree must not treat an
// existing non-directory tree entry as an absent subtree to splice into).
//
// Both cases are needed: a single-segment path (the default, no
// --git-backup-path) is a "direct" replacement in spliceTree, while a
// multi-segment path goes through its "groups" branch instead — two
// independent code paths that both need the same guard.
func TestRunBackup_PathCollidesWithExistingFile(t *testing.T) {
	cases := []struct {
		name   string
		prefix string // --git-backup-path; "" exercises spliceTree's direct map
	}{
		{"no prefix", ""},
		{"with prefix", "docs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collidesWith := "root"
			if tc.prefix != "" {
				collidesWith = tc.prefix
			}

			bareDir := initBareRemote(t)
			pushFirstCommitToRemote(t, bareDir, "main", collidesWith, "existing top-level file, not a directory\n")

			tmpDir := t.TempDir()
			rootDir := filepath.Join(tmpDir, "root")
			assetsDir := filepath.Join(tmpDir, "assets")
			if err := os.MkdirAll(rootDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(assetsDir, 0755); err != nil {
				t.Fatal(err)
			}
			writeFileHelper(t, rootDir, "page.md", "# Page\n")

			repo, err := Init(Config{
				RootDir:     rootDir,
				AssetsDir:   assetsDir,
				Path:        tc.prefix,
				AuthorName:  "Test Author",
				AuthorEmail: "test@example.com",
				Branch:      "main",
				RemoteURL:   "file://" + bareDir,
				SSHKey:      testSSHKeyPEM,
			})
			if err != nil {
				t.Fatalf("Init: %v", err)
			}

			if err := repo.RunBackup(); err == nil {
				t.Fatal("expected RunBackup to fail when the content path collides with an existing top-level file")
			}

			commit := headCommit(t, repo)
			assertFileInCommit(t, commit, collidesWith, "existing top-level file, not a directory\n")
		})
	}
}

func writeFileHelper(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("writeFileHelper MkdirAll %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeFileHelper %s: %v", rel, err)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("appendFile open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("appendFile write %s: %v", path, err)
	}
}

// TestRunBackup_WithPath_RemoteDeletionMaterialized checks that a file deleted
// on the remote is removed from the live content dir on pull.
func TestRunBackup_WithPath_RemoteDeletionMaterialized(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemotePath(t, bareDir, "docs/wiki")

	// External client deletes the page.
	deleteFromRemote(t, bareDir, "docs/wiki/root/page.md")

	if err := repo.Pull(); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "page.md")); !os.IsNotExist(err) {
		t.Fatalf("expected live page.md to be removed, stat err = %v", err)
	}
}
