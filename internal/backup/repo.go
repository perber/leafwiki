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
		r.repo = repo
		return r, nil
	}

	// Initialize new repo
	repo, err = gogit.PlainInit(repoDir, false)
	if err != nil {
		return nil, fmt.Errorf("failed to init repo: %w", err)
	}
	r.repo = repo

	// Create initial commit with root/ and assets/ if they exist
	if err := r.makeInitialCommit(); err != nil {
		return nil, fmt.Errorf("failed to make initial commit: %w", err)
	}

	return r, nil
}

// makeInitialCommit creates the first commit with root/ and assets/ directories.
func (r *Repository) makeInitialCommit() error {
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

	// Stage root/ and assets/ directories using relative paths
	// Track if we actually staged any content (files within directories)
	stagedFiles := false
	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		if _, err := wt.Add(rootRel); err != nil {
			return fmt.Errorf("failed to stage root dir: %w", err)
		}
		// Check if root has any files
		if hasFiles(r.cfg.RootDir) {
			stagedFiles = true
		}
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		if _, err := wt.Add(assetsRel); err != nil {
			return fmt.Errorf("failed to stage assets dir: %w", err)
		}
		// Check if assets has any files
		if hasFiles(r.cfg.AssetsDir) {
			stagedFiles = true
		}
	}

	// If no files were found in root/assets, skip initial commit
	// The first RunBackup will create the commit when there's actual content
	if !stagedFiles {
		return nil
	}

	// Check if there's anything to commit
	status, err := wt.Status()
	if err != nil {
		return err
	}
	if status.IsClean() {
		return nil // Nothing to commit
	}

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

	// Push to remote if configured
	if r.cfg.RemoteURL != "" {
		if err := r.push(commit.String()); err != nil {
			r.status.SetError(err.Error())
			// Don't return error - initial commit succeeded
		}
	}

	return nil
}

// hasFiles returns true if the directory contains any files (non-recursive).
func hasFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
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
	wt, err := r.repo.Worktree()
	if err != nil {
		errMsg := fmt.Errorf("failed to get worktree: %w", err).Error()
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now()) 
		return nil // Never propagate
	}

	// Compute relative paths from repo root
	repoDir := filepath.Dir(r.cfg.RootDir)
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

	// Stage root/ and assets/ directories using relative paths
	if _, err := os.Stat(r.cfg.RootDir); err == nil {
		if _, err := wt.Add(rootRel); err != nil {
			r.status.SetError(fmt.Errorf("failed to stage root dir: %w", err).Error())
			r.status.SetSuccess(time.Now()) 
			return nil
		}
	}
	if _, err := os.Stat(r.cfg.AssetsDir); err == nil {
		if _, err := wt.Add(assetsRel); err != nil {
			r.status.SetError(fmt.Errorf("failed to stage assets dir: %w", err).Error())
			r.status.SetSuccess(time.Now()) 
			return nil
		}
	}

	// Check working tree status
	status, err := wt.Status()
	if err != nil {
		errMsg := fmt.Errorf("failed to get status: %w", err).Error()
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now()) 
		return nil // Never propagate
	}

	if status.IsClean() {
		return nil // Nothing to commit - don't update LastBackupAt
	}

	// Commit changes
	commitMsg := fmt.Sprintf("backup: %s", time.Now().Format(time.RFC3339))
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
			return nil
		}
		errMsg := fmt.Errorf("failed to commit: %w", err).Error()
		r.status.SetError(errMsg)
		r.status.SetSuccess(time.Now()) 
		return nil // Never propagate
	}

	// Push to remote
	if r.cfg.RemoteURL != "" {
		if err := r.push(commit.String()); err != nil {
			r.status.SetError(err.Error())
			r.status.SetSuccess(time.Now()) 
			return nil // Never propagate
		}
	}

	r.status.SetSuccess(time.Now())
	slog.Default().Info("backup pushed to remote")
	return nil
}

// push pushes the given commit hash to the configured remote.
func (r *Repository) push(commitHash string) error {
	// Build SSH auth
	auth, err := r.buildSSHAuth()
	if err != nil {
		return fmt.Errorf("failed to build SSH auth: %w", err)
	}

	// Get remote - use r.repo directly since we're using the repo instance
	remote, err := r.repo.Remote("origin")
	if err != nil {
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
	}

	// Push HEAD to the configured remote branch
	refSpec := config.RefSpec("HEAD:refs/heads/" + r.cfg.Branch)
	err = remote.Push(&gogit.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		// "already up-to-date" means the remote already has this commit - not an error
		if strings.Contains(strings.ToLower(err.Error()), "already up-to-date") {
			slog.Default().Info("backup skipped - already up-to-date")
			return nil
		}
		slog.Default().Error("git push failed", "error", err, "remote", r.cfg.RemoteURL)
		return fmt.Errorf("failed to push: %w", err)
	}
	slog.Default().Info("git push succeeded")
	return nil
}

// buildSSHAuth builds SSH authentication from config.
func (r *Repository) buildSSHAuth() (ssh.AuthMethod, error) {
	var privateKey []byte
	var err error

	// Try SSHKey string first
	if r.cfg.SSHKey != "" {
		privateKey = []byte(r.cfg.SSHKey)
	} else if r.cfg.SSHKeyPath != "" {
		privateKey, err = os.ReadFile(r.cfg.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
	} else {
		return nil, fmt.Errorf("no SSH key provided")
	}

	// Parse the private key using x/crypto/ssh
	signer, err := sshcrypto.ParsePrivateKey(privateKey)
	if err != nil {
		slog.Default().Error("failed to parse SSH key", "error", err, "path", r.cfg.SSHKeyPath)
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	// Use InsecureIgnoreHostKey since we don't have a known_hosts file in the container
	auth := &ssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}
	auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
	return auth, nil
}

// Status returns a snapshot of the last backup time and any error.
func (r *Repository) Status() StatusSnapshot {
	return r.status.Snapshot()
}

// getRepoDir returns the parent directory of RootDir (i.e., the git repo root).
func (r *Repository) getRepoDir() string {
	return filepath.Dir(r.cfg.RootDir)
}

// stripPrefix strips the repoDir prefix from a full path to get a relative path.
func stripPrefix(fullPath, repoDir string) string {
	rel, _ := filepath.Rel(repoDir, fullPath)
	// If rel starts with "..", something is wrong - use just the base name
	if strings.HasPrefix(rel, "..") {
		return filepath.Base(fullPath)
	}
	return rel
}