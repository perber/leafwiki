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
	cfg     Config
	repoDir string
	repo    *gogit.Repository
	status  *Status
}

// Init opens an existing repo at repoDir or initialises a new one.
// On first init, stages root/ and assets/ and makes an initial commit.
func Init(cfg Config) (*Repository, error) {
	if cfg.RootDir == "" {
		return nil, fmt.Errorf("RootDir is required")
	}
	if cfg.AssetsDir == "" {
		return nil, fmt.Errorf("AssetsDir is required")
	}
	if cfg.AuthorName == "" {
		return nil, fmt.Errorf("AuthorName is required")
	}
	if cfg.AuthorEmail == "" {
		return nil, fmt.Errorf("AuthorEmail is required")
	}

	repoDir := filepath.Dir(filepath.Clean(cfg.RootDir))
	slog.Debug("backup init started", "repoDir", repoDir, "rootDir", cfg.RootDir, "assetsDir", cfg.AssetsDir, "remote", cfg.RemoteURL, "branch", cfg.Branch)

	// Ensure parent directory exists
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo directory: %w", err)
	}

	r := &Repository{
		cfg:     cfg,
		repoDir: repoDir,
		status:  &Status{},
	}

	// Try to open existing repo
	repo, err := gogit.PlainOpen(repoDir)
	if err == nil {
		slog.Debug("opened existing git repo", "repoDir", repoDir)
		r.repo = repo
		return r, nil
	}

	slog.Debug("no existing repo found, initialising new one", "repoDir", repoDir, "openErr", err)

	// Initialize new repo
	repo, err = gogit.PlainInit(repoDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to init repo: %w", err)
	}
	slog.Debug("new git repo initialised", "repoDir", repoDir)
	r.repo = repo

	if err := EnsureGitignore(repoDir); err != nil {
		return nil, fmt.Errorf("failed to write .gitignore: %w", err)
	}

	// Create initial commit with root/ and assets/ if they exist
	if err := r.makeInitialCommit(); err != nil {
		return nil, fmt.Errorf("failed to make initial commit: %w", err)
	}

	return r, nil
}

// makeInitialCommit creates the first commit with root/ and assets/ directories.
func (r *Repository) makeInitialCommit() error {
	slog.Debug("makeInitialCommit: starting")

	wt, err := r.repo.Worktree()
	if err != nil {
		return err
	}

	// Compute relative paths from repo root
	rootRel, err := filepath.Rel(r.repoDir, r.cfg.RootDir)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for root: %w", err)
	}
	assetsRel, err := filepath.Rel(r.repoDir, r.cfg.AssetsDir)
	if err != nil {
		return fmt.Errorf("failed to compute relative path for assets: %w", err)
	}
	slog.Debug("makeInitialCommit: resolved relative paths", "rootRel", rootRel, "assetsRel", assetsRel)

	// Stage root/ and assets/ directories using relative paths
	// Track if we actually staged any content (files within directories)
	stagedFiles := false
	rootDirMissing := false
	assetsDirMissing := false

	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		slog.Debug("makeInitialCommit: staging root dir", "path", rootRel)
		if _, err := wt.Add(rootRel); err != nil {
			return fmt.Errorf("failed to stage root dir: %w", err)
		}
		// Check if root has any files
		if hasFilesFlag, err := hasFiles(r.cfg.RootDir); err == nil && hasFilesFlag {
			stagedFiles = true
			slog.Debug("makeInitialCommit: root dir has files, will commit")
		} else if err != nil {
			slog.Debug("makeInitialCommit: root dir read error, skipping", "path", r.cfg.RootDir, "err", err)
		} else {
			slog.Debug("makeInitialCommit: root dir is empty, skipping")
		}
	} else {
		rootDirMissing = true
		slog.Debug("makeInitialCommit: root dir does not exist, skipping", "path", r.cfg.RootDir, "err", err)
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		slog.Debug("makeInitialCommit: staging assets dir", "path", assetsRel)
		if _, err := wt.Add(assetsRel); err != nil {
			return fmt.Errorf("failed to stage assets dir: %w", err)
		}
		// Check if assets has any files
		if hasFilesFlag, err := hasFiles(r.cfg.AssetsDir); err == nil && hasFilesFlag {
			stagedFiles = true
			slog.Debug("makeInitialCommit: assets dir has files, will commit")
		} else if err != nil {
			slog.Debug("makeInitialCommit: assets dir read error, skipping", "path", r.cfg.AssetsDir, "err", err)
		} else {
			slog.Debug("makeInitialCommit: assets dir is empty, skipping")
		}
	} else {
		assetsDirMissing = true
		slog.Debug("makeInitialCommit: assets dir does not exist, skipping", "path", r.cfg.AssetsDir, "err", err)
	}

	// Warn if both directories are missing
	if rootDirMissing && assetsDirMissing {
		slog.Warn("makeInitialCommit: both root and assets directories are missing")
	}

	// If no files were found in root/assets, skip initial commit
	// The first RunBackup will create the commit when there's actual content
	if !stagedFiles {
		slog.Debug("makeInitialCommit: no files found in root or assets, skipping initial commit")
		return nil
	}

	// Check if there's anything to commit
	status, err := wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		slog.Debug("makeInitialCommit: working tree is clean after staging, nothing to commit")
		return nil // Nothing to commit
	}
	slog.Debug("makeInitialCommit: staged file count", "count", len(status))

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
	slog.Debug("makeInitialCommit: initial commit created", "hash", commit.String())

	// Push to remote if configured
	if r.cfg.RemoteURL != "" {
		slog.Debug("makeInitialCommit: scheduling initial commit push to remote (scheduler will push on next cycle)", "remote", r.cfg.RemoteURL)
	} else {
		slog.Debug("makeInitialCommit: no remote configured, skipping push")
	}

	return nil
}

// hasFiles returns true if the directory contains any files (recursive).
// Returns an error if the directory cannot be read.
func hasFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		slog.Debug("hasFiles: failed to read directory", "dir", dir, "error", err)
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return true, nil
		}
		// Check subdirectory contents recursively
		if hasFilesRecursive, err := hasFiles(filepath.Join(dir, entry.Name())); hasFilesRecursive {
			return true, nil
		} else if err != nil {
			return false, err
		}
	}
	return false, nil
}

// hasStagedChanges returns true if the status map contains any entry where the
// staging area (index) has an actual change: added, modified, deleted, or renamed.
// Untracked files (Staging == '?') are intentionally ignored — they represent
// content outside the directories we back up and should not trigger a commit.
func hasStagedChanges(status gogit.Status) bool {
	for _, fileStatus := range status {
		switch fileStatus.Staging {
		case gogit.Added, gogit.Modified, gogit.Deleted, gogit.Renamed:
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
	slog.Debug("RunBackup: starting backup cycle")

	wt, err := r.repo.Worktree()
	if err != nil {
		errMsg := fmt.Errorf("failed to get worktree: %w", err).Error()
		slog.Debug("RunBackup: failed to get worktree", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	rootRel, err := filepath.Rel(r.repoDir, r.cfg.RootDir)
	if err != nil {
		errMsg := fmt.Errorf("failed to compute relative path for root: %w", err).Error()
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to compute relative path for root: %w", err)
	}
	assetsRel, err := filepath.Rel(r.repoDir, r.cfg.AssetsDir)
	if err != nil {
		errMsg := fmt.Errorf("failed to compute relative path for assets: %w", err).Error()
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to compute relative path for assets: %w", err)
	}
	slog.Debug("RunBackup: staging content directories", "rootRel", rootRel, "assetsRel", assetsRel)

	rootDirMissing := false
	assetsDirMissing := false

	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		if _, err := wt.Add(rootRel); err != nil {
			errMsg := fmt.Errorf("failed to stage root dir: %w", err).Error()
			slog.Debug("RunBackup: failed to stage root dir", "error", errMsg)
			r.status.SetError(errMsg)
			return fmt.Errorf("failed to stage root dir: %w", err)
		}
		slog.Debug("RunBackup: staged root dir", "path", rootRel)
	} else {
		rootDirMissing = true
		slog.Debug("RunBackup: root dir not found, skipping", "path", r.cfg.RootDir)
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		if _, err := wt.Add(assetsRel); err != nil {
			errMsg := fmt.Errorf("failed to stage assets dir: %w", err).Error()
			slog.Debug("RunBackup: failed to stage assets dir", "error", errMsg)
			r.status.SetError(errMsg)
			return fmt.Errorf("failed to stage assets dir: %w", err)
		}
		slog.Debug("RunBackup: staged assets dir", "path", assetsRel)
	} else {
		assetsDirMissing = true
		slog.Debug("RunBackup: assets dir not found, skipping", "path", r.cfg.AssetsDir)
	}

	// Warn if both directories are missing
	if rootDirMissing && assetsDirMissing {
		slog.Warn("RunBackup: both root and assets directories are missing")
	}

	// Check working tree status
	status, err := wt.Status()
	if err != nil {
		errMsg := fmt.Errorf("failed to get status: %w", err).Error()
		slog.Debug("RunBackup: failed to get working tree status", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to get status: %w", err)
	}

	// hasStagedChanges checks only the staging area (index), ignoring untracked files.
	// status.IsClean() returns false for ANY entry — including untracked files outside
	// root/ and assets/ — which would cause empty commits every cycle. We only care
	// whether the content we explicitly staged above has changed.
	staged := hasStagedChanges(status)
	slog.Debug("RunBackup: working tree status checked", "hasStagedChanges", staged, "totalStatusEntries", len(status))

	if !staged {
		slog.Info("backup skipped - no staged changes in content directories")
		r.status.SetSuccess(time.Now())
		return nil
	}

	// Log only the staged files (skip untracked noise from other app directories)
	for path, fileStatus := range status {
		if fileStatus.Staging != gogit.Untracked {
			slog.Debug("RunBackup: staged file", "path", path, "staging", string(fileStatus.Staging), "worktree", string(fileStatus.Worktree))
		}
	}

	// Commit changes
	commitMsg := fmt.Sprintf("backup: %s", time.Now().Format(time.RFC3339))
	slog.Debug("RunBackup: committing changes", "message", commitMsg, "author", r.cfg.AuthorName, "email", r.cfg.AuthorEmail)
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
			slog.Debug("RunBackup: commit skipped - empty tree")
			r.status.SetSuccess(time.Now())
			return nil
		}
		errMsg := fmt.Errorf("failed to commit: %w", err).Error()
		slog.Debug("RunBackup: commit failed", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to commit: %w", err)
	}
	slog.Debug("RunBackup: commit created", "hash", commit.String(), "message", commitMsg)

	// Push to remote
	if r.cfg.RemoteURL != "" {
		slog.Debug("RunBackup: pushing to remote", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commit.String())
		if err := r.push(commit.String()); err != nil {
			slog.Debug("RunBackup: push failed", "error", err)
			r.status.SetError(err.Error())
			return fmt.Errorf("push failed: %w", err)
		}
	} else {
		slog.Debug("RunBackup: no remote configured, skipping push")
	}

	if r.cfg.RemoteURL != "" {
		slog.Info("backup committed and pushed to remote")
	} else {
		slog.Info("backup committed locally (no remote configured)")
	}
	r.status.SetSuccess(time.Now())
	return nil
}

// push pushes the given commit hash to the configured remote.
func (r *Repository) push(commitHash string) error {
	slog.Debug("push: starting", "commit", commitHash, "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch)

	// Build SSH auth
	slog.Debug("push: building SSH auth", "sshKeyPath", r.cfg.SSHKeyPath, "hasInlineKey", r.cfg.SSHKey != "")
	auth, err := r.buildSSHAuth()
	if err != nil {
		slog.Debug("push: SSH auth build failed", "error", err)
		return fmt.Errorf("failed to build SSH auth: %w", err)
	}
	slog.Debug("push: SSH auth built successfully")

	// Get remote - use r.repo directly since we're using the repo instance
	remote, err := r.repo.Remote("origin")
	if err != nil {
		slog.Debug("push: remote 'origin' not found, creating it", "url", r.cfg.RemoteURL)
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
		slog.Debug("push: remote 'origin' created", "url", r.cfg.RemoteURL)
	} else {
		remoteURLs := remote.Config().URLs
		slog.Debug("push: remote 'origin' found", "urls", remoteURLs)
	}

	// Resolve local HEAD to verify what we are about to push.
	localHead, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to resolve local HEAD: %w", err)
	}
	slog.Debug("push: local HEAD resolved", "hash", localHead.Hash().String(), "branch", localHead.Name().Short())

	// Delete the local remote-tracking ref before pushing.
	// go-git compares local HEAD against refs/remotes/origin/<branch> (the cached
	// tracking ref written by previous pushes). If the remote was reset or recreated
	// since the last push, the tracking ref still points to a commit the live remote
	// no longer has — causing go-git to short-circuit with ErrAlreadyUpToDate before
	// even attempting to send the pack. Removing it forces a clean push.
	trackingRef := plumbing.NewRemoteReferenceName("origin", r.cfg.Branch)
	if rmErr := r.repo.Storer.RemoveReference(trackingRef); rmErr != nil && rmErr != plumbing.ErrReferenceNotFound {
		slog.Debug("push: could not remove stale remote tracking ref", "ref", trackingRef.String(), "error", rmErr)
	} else {
		slog.Debug("push: cleared remote tracking ref", "ref", trackingRef.String())
	}

	// Use the resolved branch ref explicitly rather than HEAD.
	// Symbolic HEAD in a force refspec can confuse go-git when the local branch
	// name differs from the configured remote branch (e.g. local=master, remote=main).
	localBranchRef := localHead.Name().String() // e.g. refs/heads/master
	refSpec := config.RefSpec(localBranchRef + ":refs/heads/" + r.cfg.Branch)
	slog.Debug("push: pushing", "refSpec", string(refSpec), "localBranch", localBranchRef, "remoteBranch", r.cfg.Branch)
	err = remote.Push(&gogit.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already up-to-date") {
			// Genuine up-to-date: remote caught up between our List call and Push.
			slog.Info("backup skipped - already up-to-date on " + r.cfg.Branch + " at remote URL: " + r.cfg.RemoteURL)
			return nil
		}
		slog.Error("git push failed", "error", err, "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "refSpec", string(refSpec))
		return fmt.Errorf("failed to push: %w", err)
	}
	slog.Info("git push succeeded", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commitHash)
	return nil
}

// buildSSHAuth builds SSH authentication from config.
func (r *Repository) buildSSHAuth() (ssh.AuthMethod, error) {
	var privateKey []byte
	var err error

	// Try SSHKey string first
	if r.cfg.SSHKey != "" {
		slog.Debug("buildSSHAuth: using inline SSH key")
		privateKey = []byte(r.cfg.SSHKey)
	} else if r.cfg.SSHKeyPath != "" {
		slog.Debug("buildSSHAuth: reading SSH key from file", "path", r.cfg.SSHKeyPath)
		privateKey, err = os.ReadFile(r.cfg.SSHKeyPath)
		if err != nil {
			slog.Debug("buildSSHAuth: failed to read SSH key file", "path", r.cfg.SSHKeyPath, "error", err)
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
		slog.Debug("buildSSHAuth: SSH key file read successfully", "path", r.cfg.SSHKeyPath, "size", len(privateKey))
	} else {
		slog.Debug("buildSSHAuth: no SSH key configured (neither inline nor path)")
		return nil, fmt.Errorf("no SSH key provided")
	}

	// Parse the private key using x/crypto/ssh
	signer, err := sshcrypto.ParsePrivateKey(privateKey)
	if err != nil {
		slog.Error("failed to parse SSH key", "error", err, "path", r.cfg.SSHKeyPath)
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}
	slog.Debug("buildSSHAuth: SSH key parsed successfully", "keyType", signer.PublicKey().Type())

	auth := &ssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}

	// Use known hosts for MITM protection if provided.
	// NewKnownHostsCallback expects a file path, so we write the raw content to a temp file.
	if r.cfg.SSHKnownHosts != "" {
		tmpFile, tmpErr := os.CreateTemp("", "known_hosts_*")
		if tmpErr != nil {
			slog.Warn("buildSSHAuth: failed to create temp file for SSHKnownHosts, falling back to insecure mode", "error", tmpErr)
			auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
		} else {
			defer os.Remove(tmpFile.Name())
			if _, writeErr := tmpFile.WriteString(r.cfg.SSHKnownHosts); writeErr != nil {
				tmpFile.Close()
				slog.Warn("buildSSHAuth: failed to write SSHKnownHosts to temp file, falling back to insecure mode", "error", writeErr)
				auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
			} else {
				tmpFile.Close()
				knownHostsCallback, err := ssh.NewKnownHostsCallback(tmpFile.Name())
				if err != nil {
					slog.Warn("buildSSHAuth: failed to parse SSHKnownHosts, falling back to insecure mode", "error", err)
					auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
				} else {
					auth.HostKeyCallback = knownHostsCallback
					slog.Debug("buildSSHAuth: SSH auth configured with known hosts callback")
				}
			}
		}
	} else {
		slog.Warn("buildSSHAuth: no SSHKnownHosts provided, connection will be insecure (no MITM protection)")
		auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
	}
	return auth, nil
}

// Status returns a snapshot of the last backup time and any error.
func (r *Repository) Status() StatusSnapshot {
	return r.status.Snapshot()
}
