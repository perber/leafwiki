package backup

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
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
	commitFilesToRemote(t, bareDir, map[string]string{filename: content})
}

// commitFilesToRemote is commitToRemote for several files in one commit; files
// maps repository-relative paths to their new content.
func commitFilesToRemote(t *testing.T, bareDir string, files map[string]string) {
	t.Helper()
	commitChangesToRemote(t, bareDir, files, nil)
}

// commitChangesToRemote pushes one external commit that writes files (paths
// relative to the repository, mapped to their new content) and removes the
// paths in deletes, so a single pull sees edits and deletes together.
func commitChangesToRemote(t *testing.T, bareDir string, files map[string]string, deletes []string) {
	t.Helper()
	cloneDir := t.TempDir()
	cloned, err := gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
		URL:           "file://" + bareDir,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		Auth:          nil,
	})
	if err != nil {
		t.Fatalf("commitChangesToRemote: PlainClone failed: %v", err)
	}
	wt, err := cloned.Worktree()
	if err != nil {
		t.Fatalf("commitChangesToRemote: Worktree failed: %v", err)
	}
	for filename, content := range files {
		path := filepath.Join(cloneDir, filename)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("commitChangesToRemote: MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("commitChangesToRemote: WriteFile: %v", err)
		}
		if _, err := wt.Add(filename); err != nil {
			t.Fatalf("commitChangesToRemote: Add failed: %v", err)
		}
	}
	for _, filename := range deletes {
		if _, err := wt.Remove(filename); err != nil {
			t.Fatalf("commitChangesToRemote: Remove %s failed: %v", filename, err)
		}
	}
	if _, err := wt.Commit("external commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "External",
			Email: "ext@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("commitChangesToRemote: Commit failed: %v", err)
	}
	remote, err := cloned.Remote("origin")
	if err != nil {
		t.Fatalf("commitChangesToRemote: Remote failed: %v", err)
	}
	if err := remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"},
		Auth:     nil,
	}); err != nil {
		t.Fatalf("commitChangesToRemote: Push failed: %v", err)
	}
}

// pushFirstCommitToRemote creates the very first commit on bareDir's branch
// directly, without going through Init/RunBackup — simulating an external
// client (e.g. another instance, or an admin) creating the remote branch out
// of band, before any local instance has ever synced with it.
func pushFirstCommitToRemote(t *testing.T, bareDir, branch, filename, content string) {
	t.Helper()
	pushNewHistoryToRemote(t, bareDir, branch, filename, content, "external initial commit", false)
}

// forcePushRewrite creates a brand-new, unrelated commit history (no shared
// ancestry with whatever the remote branch currently points to) and
// force-pushes it, simulating an external history rewrite.
func forcePushRewrite(t *testing.T, bareDir, branch, filename, content string) {
	t.Helper()
	pushNewHistoryToRemote(t, bareDir, branch, filename, content, "rewritten history", true)
}

// pushNewHistoryToRemote creates a fresh single-commit repo containing
// filename/content and pushes it to bareDir's branch, simulating an external
// client acting directly on the remote. force selects a normal push (for a
// branch that doesn't exist on the remote yet) or a forced one (to rewrite an
// existing, unrelated branch).
func pushNewHistoryToRemote(t *testing.T, bareDir, branch, filename, content, commitMessage string, force bool) {
	t.Helper()
	cloneDir := t.TempDir()
	repo, err := gogit.PlainInitWithOptions(cloneDir, &gogit.PlainInitOptions{
		InitOptions: gogit.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName(branch)},
	})
	if err != nil {
		t.Fatalf("pushNewHistoryToRemote: PlainInitWithOptions: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("pushNewHistoryToRemote: Worktree: %v", err)
	}
	path := filepath.Join(cloneDir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("pushNewHistoryToRemote: MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("pushNewHistoryToRemote: WriteFile: %v", err)
	}
	if _, err := wt.Add(filename); err != nil {
		t.Fatalf("pushNewHistoryToRemote: Add: %v", err)
	}
	if _, err := wt.Commit(commitMessage, &gogit.CommitOptions{
		Author: &object.Signature{Name: "External", Email: "ext@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("pushNewHistoryToRemote: Commit: %v", err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"file://" + bareDir},
	}); err != nil {
		t.Fatalf("pushNewHistoryToRemote: CreateRemote: %v", err)
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		t.Fatalf("pushNewHistoryToRemote: Remote: %v", err)
	}
	branchRef := plumbing.NewBranchReferenceName(branch)
	refSpec := config.RefSpec(branchRef + ":" + branchRef)
	if force {
		refSpec = config.RefSpec("+" + branchRef + ":" + branchRef)
	}
	if err := remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
		Force:    force,
	}); err != nil {
		t.Fatalf("pushNewHistoryToRemote: Push: %v", err)
	}
}

// filesystemIsCaseInsensitive reports whether t.TempDir()'s filesystem folds
// case (the default on Windows and macOS). Tests that depend on this should
// skip on a case-sensitive filesystem (the default on Linux/CI) rather than
// fail, since the scenario they exercise cannot occur there.
func filesystemIsCaseInsensitive(t *testing.T) bool {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "case-probe"), []byte("x"), 0644); err != nil {
		t.Fatalf("filesystemIsCaseInsensitive: WriteFile: %v", err)
	}
	_, err := os.Stat(filepath.Join(dir, "CASE-PROBE"))
	return err == nil
}

// deleteFromRemote simulates an external client deleting a file on the remote.
func deleteFromRemote(t *testing.T, bareDir, filename string) {
	t.Helper()
	cloneDir := t.TempDir()
	cloned, err := gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{
		URL:           "file://" + bareDir,
		ReferenceName: plumbing.NewBranchReferenceName("main"),
	})
	if err != nil {
		t.Fatalf("deleteFromRemote: PlainClone failed: %v", err)
	}
	wt, err := cloned.Worktree()
	if err != nil {
		t.Fatalf("deleteFromRemote: Worktree failed: %v", err)
	}
	if _, err := wt.Remove(filename); err != nil {
		t.Fatalf("deleteFromRemote: Remove failed: %v", err)
	}
	if _, err := wt.Commit("external delete", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "External",
			Email: "ext@example.com",
			When:  time.Now(),
		},
	}); err != nil {
		t.Fatalf("deleteFromRemote: Commit failed: %v", err)
	}
	remote, err := cloned.Remote("origin")
	if err != nil {
		t.Fatalf("deleteFromRemote: Remote failed: %v", err)
	}
	if err := remote.Push(&gogit.PushOptions{
		RefSpecs: []config.RefSpec{"refs/heads/main:refs/heads/main"},
	}); err != nil {
		t.Fatalf("deleteFromRemote: Push failed: %v", err)
	}
}

// remoteHeadCommit returns the commit the bare remote's main branch points at.
func remoteHeadCommit(t *testing.T, bareDir string) *object.Commit {
	t.Helper()
	bare, err := gogit.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("remoteHeadCommit: PlainOpen: %v", err)
	}
	ref, err := bare.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatalf("remoteHeadCommit: remote has no main branch: %v", err)
	}
	commit, err := bare.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("remoteHeadCommit: CommitObject: %v", err)
	}
	return commit
}

// pushRawRootEntryToRemote commits an entry with the raw name name under root/
// on the bare remote, written with git plumbing so none of go-git's worktree
// name checks apply. With dir set the entry sits in a root/<dir>/
// directory. With files nil the entry is a file holding "outside"; otherwise
// it is a directory holding files.
func pushRawRootEntryToRemote(t *testing.T, bareDir, dir, name string, files map[string]string) {
	t.Helper()
	bare, err := gogit.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("pushRawRootEntryToRemote: PlainOpen: %v", err)
	}
	s := bare.Storer
	blob := func(content string) plumbing.Hash {
		obj := s.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		w, err := obj.Writer()
		if err != nil {
			t.Fatalf("pushRawRootEntryToRemote: blob writer: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("pushRawRootEntryToRemote: write blob: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("pushRawRootEntryToRemote: close blob: %v", err)
		}
		h, err := s.SetEncodedObject(obj)
		if err != nil {
			t.Fatalf("pushRawRootEntryToRemote: store blob: %v", err)
		}
		return h
	}
	tree := func(entries []object.TreeEntry) plumbing.Hash {
		sort.Sort(object.TreeEntrySorter(entries))
		obj := s.NewEncodedObject()
		if err := (&object.Tree{Entries: entries}).Encode(obj); err != nil {
			t.Fatalf("pushRawRootEntryToRemote: encode tree: %v", err)
		}
		h, err := s.SetEncodedObject(obj)
		if err != nil {
			t.Fatalf("pushRawRootEntryToRemote: store tree: %v", err)
		}
		return h
	}

	entry := object.TreeEntry{Name: name, Mode: filemode.Regular, Hash: blob("outside")}
	if files != nil {
		var children []object.TreeEntry
		for fileName, content := range files {
			children = append(children, object.TreeEntry{Name: fileName, Mode: filemode.Regular, Hash: blob(content)})
		}
		entry = object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: tree(children)}
	}
	if dir != "" {
		entry = object.TreeEntry{Name: dir, Mode: filemode.Dir, Hash: tree([]object.TreeEntry{entry})}
	}

	parent := remoteHeadCommit(t, bareDir)
	top, err := parent.Tree()
	if err != nil {
		t.Fatalf("pushRawRootEntryToRemote: Tree: %v", err)
	}
	root, err := top.Tree("root")
	if err != nil {
		t.Fatalf("pushRawRootEntryToRemote: remote has no root/: %v", err)
	}
	newRoot := tree(append(append([]object.TreeEntry{}, root.Entries...), entry))
	var topEntries []object.TreeEntry
	for _, e := range top.Entries {
		if e.Name == "root" {
			e.Hash = newRoot
		}
		topEntries = append(topEntries, e)
	}
	sig := object.Signature{Name: "External", Email: "ext@example.com", When: time.Now()}
	commit := &object.Commit{
		Author: sig, Committer: sig, Message: "raw entry",
		TreeHash: tree(topEntries), ParentHashes: []plumbing.Hash{parent.Hash},
	}
	obj := s.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		t.Fatalf("pushRawRootEntryToRemote: encode commit: %v", err)
	}
	commitHash, err := s.SetEncodedObject(obj)
	if err != nil {
		t.Fatalf("pushRawRootEntryToRemote: store commit: %v", err)
	}
	if err := s.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("main"), commitHash)); err != nil {
		t.Fatalf("pushRawRootEntryToRemote: SetReference: %v", err)
	}
}
