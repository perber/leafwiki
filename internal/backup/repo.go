package backup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	sshcrypto "golang.org/x/crypto/ssh"
)

// Repository wraps a git repository with backup-specific state.
type Repository struct {
	cfg    Config
	repo   *gogit.Repository
	status *Status
}

// Init opens an existing repo at repoDir or initialises a new one.
// On first init, stages root/ and assets/ and makes an initial commit.
func Init(cfg Config) (*Repository, error) {
	repoDir := filepath.Dir(cfg.RootDir)
	slog.Default().Debug("backup init started", "repoDir", repoDir, "rootDir", cfg.RootDir, "assetsDir", cfg.AssetsDir, "remote", cfg.RemoteURL, "branch", cfg.Branch)

	if cfg.RootDir == "" {
		return nil, fmt.Errorf("RootDir is required")
	}
	if cfg.AssetsDir == "" {
		return nil, fmt.Errorf("AssetsDir is required")
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo directory: %w", err)
	}

	r := &Repository{
		cfg:    cfg,
		status: &Status{},
	}

	// Try to open existing repo
	repo, err := gogit.PlainOpen(repoDir)
	if err == nil {
		slog.Default().Debug("opened existing git repo", "repoDir", repoDir)
		r.repo = repo
		return r, nil
	}

	slog.Default().Debug("no existing repo found, initialising new one", "repoDir", repoDir, "openErr", err)

	// Initialize new repo
	repo, err = gogit.PlainInit(repoDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to init repo: %w", err)
	}
	slog.Default().Debug("new git repo initialised", "repoDir", repoDir)
	r.repo = repo

	// Create initial commit with root/ and assets/ if they exist
	if err := r.makeInitialCommit(); err != nil {
		return nil, fmt.Errorf("failed to make initial commit: %w", err)
	}

	return r, nil
}

// makeInitialCommit creates the first commit with root/ and assets/ directories.
func (r *Repository) makeInitialCommit() error {
	slog.Default().Debug("makeInitialCommit: starting")

	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	// Compute relative paths from repo root
	repoDir := filepath.Dir(r.cfg.RootDir)
	rootRel, err := filepath.Rel(repoDir, r.cfg.RootDir)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for root: %w", err)
	}
	assetsRel, err := filepath.Rel(repoDir, r.cfg.AssetsDir)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for assets: %w", err)
	}
	slog.Default().Debug("makeInitialCommit: resolved relative paths", "rootRel", rootRel, "assetsRel", assetsRel)

	// Stage root/ and assets/ directories using relative paths
	// Track if we actually staged any content (files within directories)
	stagedFiles := false
	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		slog.Default().Debug("makeInitialCommit: staging root dir", "path", rootRel)
		if _, err := wt.Add(rootRel); err != nil {
			return fmt.Errorf("failed to stage root dir: %w", err)
		}
		// Check if root has any files
		if hasFiles(r.cfg.RootDir) {
			stagedFiles = true
			slog.Default().Debug("makeInitialCommit: root dir has files, will commit")
		} else {
			slog.Default().Debug("makeInitialCommit: root dir is empty, skipping")
		}
	} else {
		slog.Default().Debug("makeInitialCommit: root dir does not exist, skipping", "path", r.cfg.RootDir, "err", err)
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		slog.Default().Debug("makeInitialCommit: staging assets dir", "path", assetsRel)
		if _, err := wt.Add(assetsRel); err != nil {
			return fmt.Errorf("failed to stage assets dir: %w", err)
		}
		// Check if assets has any files
		if hasFiles(r.cfg.AssetsDir) {
			stagedFiles = true
			slog.Default().Debug("makeInitialCommit: assets dir has files, will commit")
		} else {
			slog.Default().Debug("makeInitialCommit: assets dir is empty, skipping")
		}
	} else {
		slog.Default().Debug("makeInitialCommit: assets dir does not exist, skipping", "path", r.cfg.AssetsDir, "err", err)
	}

	// If no files were found in root/assets, skip initial commit
	// The first RunBackup will create the commit when there's actual content
	if !stagedFiles {
		slog.Default().Debug("makeInitialCommit: no files found in root or assets, skipping initial commit")
		return nil
	}

	// Check if there's anything to commit
	status, err := wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		slog.Default().Debug("makeInitialCommit: working tree is clean after staging, nothing to commit")
		return nil // Nothing to commit
	}
	slog.Default().Debug("makeInitialCommit: staged file count", "count", len(status))

	commit, err := wt.Commit("Initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  r.cfg.AuthorName,
			Email: r.cfg.AuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	slog.Default().Debug("makeInitialCommit: initial commit created", "hash", commit.String())

	// Push to remote if configured
	if r.cfg.RemoteURL != "" {
		slog.Default().Debug("makeInitialCommit: pushing initial commit to remote", "remote", r.cfg.RemoteURL)
		if err := r.push(commit.String()); err != nil {
			r.status.SetError(err.Error())
			slog.Default().Error("initial commit push failed", "error", err, "remote", r.cfg.RemoteURL)
			// Don't return error - initial commit succeeded but push failed
		}
	} else {
		slog.Default().Debug("makeInitialCommit: no remote configured, skipping push")
	}

	return nil
}

// hasFiles returns true if the directory contains any files (recursive).
func hasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return true
		}
		// Check subdirectory contents recursively
		if hasFiles(filepath.Join(dir, entry.Name())) {
			return true
		}
	}
	return false
}

// hasStagedChanges returns true if the status map contains any entry where the
// staging area (index) has an actual change: added, modified, deleted, or renamed.
// Untracked files (Staging == '?') are intentionally ignored — they represent
// content outside the directories we back up and should not trigger a commit.
func hasStagedChanges(status gogit.Status) bool {
	for _, fileStatus := range status {
		switch fileStatus.Staging {
		case gogit.Added, gogit.Modified, gogit.Deleted, gogit.Renamed, gogit.Copied:
			return true
		}
	}
	return false
}

// RunBackup stages all changes in root/ and assets/, commits if anything
// changed, then pushes to the configured remote.
// message format: "backup: <RFC3339 timestamp>"
// Returns nil and skips commit+push if the working tree is clean.
func (r *Repository) RunBackup() error {
	slog.Default().Debug("RunBackup: starting backup cycle")

	wt, err := r.repo.Worktree()
	if err != nil {
		errMsg := fmt.Errorf("failed to get worktree: %w", err).Error()
		slog.Default().Debug("RunBackup: failed to get worktree", "error", errMsg)
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now())
		return nil // Never propagate
	}

	repoDir := filepath.Dir(r.cfg.RootDir)
	slog.Default().Debug("RunBackup: resolved repo dir", "repoDir", repoDir)

	// Compute relative paths for the two content directories we back up.
	// Only root/ (wiki pages) and assets/ (uploaded files) are included.
	// Internal app directories (.leafwiki/, schema.json, search.db-journal, etc.)
	// are intentionally excluded — they are application state, not user content.
	rootRel, err := filepath.Rel(repoDir, r.cfg.RootDir)
	if err != nil {
		r.status.SetError(fmt.Errorf("failed to compute relative path for root: %w", err).Error())
		r.status.SetSuccess(time.Now())
		return nil
	}
	assetsRel, err := filepath.Rel(repoDir, r.cfg.AssetsDir)
	if err != nil {
		r.status.SetError(fmt.Errorf("failed to compute relative path for assets: %w", err).Error())
		r.status.SetSuccess(time.Now())
		return nil
	}
	slog.Default().Debug("RunBackup: staging content directories", "rootRel", rootRel, "assetsRel", assetsRel)

	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		if _, err := wt.Add(rootRel); err != nil {
			errMsg := fmt.Errorf("failed to stage root dir: %w", err).Error()
			slog.Default().Debug("RunBackup: failed to stage root dir", "error", errMsg)
			r.status.SetError(errMsg)
			r.status.SetSuccess(time.Now())
			return nil
		}
		slog.Default().Debug("RunBackup: staged root dir", "path", rootRel)
	} else {
		slog.Default().Debug("RunBackup: root dir not found, skipping", "path", r.cfg.RootDir)
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		if _, err := wt.Add(assetsRel); err != nil {
			errMsg := fmt.Errorf("failed to stage assets dir: %w", err).Error()
			slog.Default().Debug("RunBackup: failed to stage assets dir", "error", errMsg)
			r.status.SetError(errMsg)
			r.status.SetSuccess(time.Now())
			return nil
		}
		slog.Default().Debug("RunBackup: staged assets dir", "path", assetsRel)
	} else {
		slog.Default().Debug("RunBackup: assets dir not found, skipping", "path", r.cfg.AssetsDir)
	}

	// Check working tree status
	status, err := wt.Status()
	if err != nil {
		errMsg := fmt.Errorf("failed to get status: %w", err).Error()
		slog.Default().Debug("RunBackup: failed to get working tree status", "error", errMsg)
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now())
		return nil // Never propagate
	}

	// hasStagedChanges checks only the staging area (index), ignoring untracked files.
	// status.IsClean() returns false for ANY entry — including untracked files outside
	// root/ and assets/ — which would cause empty commits every cycle. We only care
	// whether the content we explicitly staged above has changed.
	staged := hasStagedChanges(status)
	slog.Default().Debug("RunBackup: working tree status checked", "hasStagedChanges", staged, "totalStatusEntries", len(status))

	if !staged {
		slog.Default().Info("backup skipped - no staged changes in content directories")
		r.status.SetSuccess(time.Now())
		return nil
	}

	// Log only the staged files (skip untracked noise from other app directories)
	for path, fileStatus := range status {
		if fileStatus.Staging != gogit.Untracked {
			slog.Default().Debug("RunBackup: staged file", "path", path, "staging", string(fileStatus.Staging), "worktree", string(fileStatus.Worktree))
		}
	}

	// Commit changes
	commitMsg := fmt.Sprintf("backup: %s", time.Now().Format(time.RFC3339))
	slog.Default().Debug("RunBackup: committing changes", "message", commitMsg, "author", r.cfg.AuthorName, "email", r.cfg.AuthorEmail)
	commit, err := wt.Commit(commitMsg, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  r.cfg.AuthorName,
			Email: r.cfg.AuthorEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		// If it's "nothing to commit" (empty tree), that's fine - just skip
		if strings.Contains(err.Error(), "cannot create empty commit") {
			slog.Default().Debug("RunBackup: commit skipped - empty tree")
			return nil
		}
		errMsg := fmt.Errorf("failed to commit: %w", err).Error()
		slog.Default().Debug("RunBackup: commit failed", "error", errMsg)
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now())
		return nil // Never propagate
	}
	slog.Default().Debug("RunBackup: commit created", "hash", commit.String(), "message", commitMsg)

	// Push to remote
	if r.cfg.RemoteURL != "" {
		slog.Default().Debug("RunBackup: pushing to remote", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commit.String())
		if err := r.push(commit.String()); err != nil {
			slog.Default().Debug("RunBackup: push failed", "error", err)
			r.status.SetError(err.Error())
			return nil // Never propagate
		}
	} else {
		slog.Default().Debug("RunBackup: no remote configured, skipping push")
	}

	r.status.SetSuccess(time.Now())
	slog.Default().Info("backup pushed to remote")
	return nil
}

// push pushes the given commit hash to the configured remote.
func (r *Repository) push(commitHash string) error {
	slog.Default().Debug("push: starting", "commit", commitHash, "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch)

	// Build SSH auth
	slog.Default().Debug("push: building SSH auth", "sshKeyPath", r.cfg.SSHKeyPath, "hasInlineKey", r.cfg.SSHKey != "")
	auth, err := r.buildSSHAuth()
	if err != nil {
		slog.Default().Debug("push: SSH auth build failed", "error", err)
		return fmt.Errorf("failed to build SSH auth: %w", err)
	}
	slog.Default().Debug("push: SSH auth built successfully")

	// Get remote - use r.repo directly since we're using the repo instance
	remote, err := r.repo.Remote("origin")
	if err != nil {
		slog.Default().Debug("push: remote 'origin' not found, creating it", "url", r.cfg.RemoteURL)
		// Remote doesn't exist, create it
		_, err = r.repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{r.cfg.RemoteURL},
		})
		if err != nil {
			return fmt.Errorf("failed to create remote: %w", err)
		}
		remote, err = r.repo.Remote("origin")
		if err != nil {
			return fmt.Errorf("failed to get remote: %w", err)
		}
		slog.Default().Debug("push: remote 'origin' created", "url", r.cfg.RemoteURL)
	} else {
		remoteURLs := remote.Config().URLs
		slog.Default().Debug("push: remote 'origin' found", "urls", remoteURLs)
	}

	// Resolve local HEAD to verify what we are about to push.
	localHead, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to resolve local HEAD: %w", err)
	}
	slog.Default().Debug("push: local HEAD resolved", "hash", localHead.Hash().String(), "branch", localHead.Name().Short())

	// Fetch the current remote ref so we can accurately detect true up-to-date.
	// go-git returns ErrAlreadyUpToDate for empty/fresh remotes (no refs yet),
	// which is a false positive — the remote simply has no branch to compare against.
	// Listing first gives us ground truth before we decide whether to push.
	remoteRefs, fetchErr := remote.List(&gogit.ListOptions{Auth: auth})
	if fetchErr != nil {
		slog.Default().Debug("push: could not list remote refs (remote may be empty)", "error", fetchErr)
	}

	remoteHead := ""
	targetRef := "refs/heads/" + r.cfg.Branch
	for _, ref := range remoteRefs {
		if ref.Name().String() == targetRef {
			remoteHead = ref.Hash().String()
			break
		}
	}
	slog.Default().Debug("push: remote branch state", "branch", r.cfg.Branch, "remoteHead", remoteHead, "localHead", commitHash)

	if remoteHead == commitHash {
		// Remote genuinely already has this exact commit — nothing to do.
		slog.Default().Info("backup skipped - remote already at current commit", "branch", r.cfg.Branch, "commit", commitHash)
		return nil
	}

	// Delete the local remote-tracking ref before pushing.
	// go-git compares local HEAD against refs/remotes/origin/<branch> (the cached
	// tracking ref written by previous pushes). If the remote was reset or recreated
	// since the last push, the tracking ref still points to a commit the live remote
	// no longer has — causing go-git to short-circuit with ErrAlreadyUpToDate before
	// even attempting to send the pack. Removing it forces a clean push.
	trackingRef := plumbing.NewRemoteReferenceName("origin", r.cfg.Branch)
	if rmErr := r.repo.Storer.RemoveReference(trackingRef); rmErr != nil && rmErr != plumbing.ErrReferenceNotFound {
		slog.Default().Debug("push: could not remove stale remote tracking ref", "ref", trackingRef.String(), "error", rmErr)
	} else {
		slog.Default().Debug("push: cleared remote tracking ref", "ref", trackingRef.String())
	}

	// Use the resolved branch ref explicitly rather than HEAD.
	// Symbolic HEAD in a force refspec can confuse go-git when the local branch
	// name differs from the configured remote branch (e.g. local=master, remote=main).
	localBranchRef := localHead.Name().String() // e.g. refs/heads/master
	refSpec := config.RefSpec("+" + localBranchRef + ":refs/heads/" + r.cfg.Branch)
	slog.Default().Debug("push: pushing with force refspec", "refSpec", string(refSpec), "localBranch", localBranchRef, "remoteBranch", r.cfg.Branch)
	err = remote.Push(&gogit.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
		Force:    true,
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already up-to-date") {
			// Genuine up-to-date: remote caught up between our List call and Push.
			slog.Default().Info("backup skipped - already up-to-date on " + r.cfg.Branch + " at remote URL: " + r.cfg.RemoteURL)
			return nil
		}
		slog.Default().Error("git push failed", "error", err, "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "refSpec", string(refSpec))
		return fmt.Errorf("failed to push: %w", err)
	}
	slog.Default().Info("git push succeeded", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commitHash)
	return nil
}

// buildSSHAuth builds SSH authentication from config.
func (r *Repository) buildSSHAuth() (ssh.AuthMethod, error) {
	var privateKey []byte
	var err error

	// Try SSHKey string first
	if r.cfg.SSHKey != "" {
		slog.Default().Debug("buildSSHAuth: using inline SSH key")
		privateKey = []byte(r.cfg.SSHKey)
	} else if r.cfg.SSHKeyPath != "" {
		slog.Default().Debug("buildSSHAuth: reading SSH key from file", "path", r.cfg.SSHKeyPath)
		privateKey, err = os.ReadFile(r.cfg.SSHKeyPath)
		if err != nil {
			slog.Default().Debug("buildSSHAuth: failed to read SSH key file", "path", r.cfg.SSHKeyPath, "error", err)
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
		slog.Default().Debug("buildSSHAuth: SSH key file read successfully", "path", r.cfg.SSHKeyPath, "size", len(privateKey))
	} else {
		slog.Default().Debug("buildSSHAuth: no SSH key configured (neither inline nor path)")
		return nil, fmt.Errorf("no SSH key provided")
	}

	// Parse the private key using x/crypto/ssh
	signer, err := sshcrypto.ParsePrivateKey(privateKey)
	if err != nil {
		slog.Default().Error("failed to parse SSH key", "error", err, "path", r.cfg.SSHKeyPath)
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}
	slog.Default().Debug("buildSSHAuth: SSH key parsed successfully", "keyType", signer.PublicKey().Type())

	// Use InsecureIgnoreHostKey since we don't have a known_hosts file in the container
	auth := &ssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}
	auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
	slog.Default().Debug("buildSSHAuth: SSH auth configured with InsecureIgnoreHostKey")
	return auth, nil
}

// Status returns a snapshot of the last backup time and any error.
func (r *Repository) Status() StatusSnapshot {
	return r.status.Snapshot()
}
