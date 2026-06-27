package backup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gossh "golang.org/x/crypto/ssh"
)

// testSSHKeyPEM holds a throwaway Ed25519 key generated at test startup.
// Local file:// remotes don't use SSH auth, but buildSSHAuth requires a parseable key.
var testSSHKeyPEM string

func TestMain(m *testing.M) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic("failed to generate test SSH key: " + err.Error())
	}
	block, err := gossh.MarshalPrivateKey(priv, "")
	if err != nil {
		panic("failed to marshal test SSH key: " + err.Error())
	}
	testSSHKeyPEM = string(pem.EncodeToMemory(block))
	os.Exit(m.Run())
}

// initBareRemote creates a bare git repo that acts as a remote.
func initBareRemote(t *testing.T) string {
	t.Helper()
	bareDir := t.TempDir()
	if _, err := gogit.PlainInit(bareDir, true); err != nil {
		t.Fatalf("initBareRemote: PlainInit failed: %v", err)
	}
	return bareDir
}

// newRepoWithRemote creates a local Repository with a file in root/, makes an
// initial RunBackup (which pushes to the bare remote), and returns the repo
// and its rootDir so tests can write more files.
func newRepoWithRemote(t *testing.T, bareDir string) (*Repository, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("newRepoWithRemote: MkdirAll root: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("newRepoWithRemote: MkdirAll assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("newRepoWithRemote: WriteFile: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		RemoteURL:   "file://" + bareDir,
		SSHKey:      testSSHKeyPEM,
	}
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("newRepoWithRemote: Init failed: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("newRepoWithRemote: initial RunBackup failed: %v", err)
	}
	return repo, rootDir
}

// commitToRemote simulates an external client pushing a commit directly to the
// bare remote (e.g. an edit via the GitHub UI).
func commitToRemote(t *testing.T, bareDir, filename, content string) {
	t.Helper()
	cloneDir := t.TempDir()
	cloned, err := gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
		URL:           "file://" + bareDir,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		Auth:          nil,
	})
	if err != nil {
		t.Fatalf("commitToRemote: PlainClone failed: %v", err)
	}
	wt, err := cloned.Worktree()
	if err != nil {
		t.Fatalf("commitToRemote: Worktree failed: %v", err)
	}
	path := filepath.Join(cloneDir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("commitToRemote: MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("commitToRemote: WriteFile: %v", err)
	}
	if _, err := wt.Add(filename); err != nil {
		t.Fatalf("commitToRemote: Add failed: %v", err)
	}
	if _, err := wt.Commit("external commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "External",
			Email: "ext@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("commitToRemote: Commit failed: %v", err)
	}
	remote, err := cloned.Remote("origin")
	if err != nil {
		t.Fatalf("commitToRemote: Remote failed: %v", err)
	}
	if err := remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"},
		Auth:     nil,
	}); err != nil {
		t.Fatalf("commitToRemote: Push failed: %v", err)
	}
}
