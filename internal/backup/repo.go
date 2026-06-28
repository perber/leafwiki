package backup

import (
	"context"
	"errors"
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
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	sshcrypto "golang.org/x/crypto/ssh"
)

// gcLooseThreshold is the number of loose objects that triggers a gc() run.
// git itself defaults to 6700; we use a lower value because the backup repo
// accumulates objects predictably and we prefer smaller, more frequent packs.
const gcLooseThreshold = 500

// networkTimeout caps how long a single SSH pull or push may block the
// scheduler goroutine. A TCP-blackholed remote would otherwise stall it forever.
const networkTimeout = 2 * time.Minute

// Repository wraps a git repository with backup-specific state.
type Repository struct {
	cfg              Config
	repoDir          string
	repo             *gogit.Repository
	status           *Status
	looseObjsSinceGC int           // loose objects created since last gc
	lastPushedHash   plumbing.Hash  // hash of the last commit successfully pushed; zero = never pushed

	// OnRemoteChanges is called (in the scheduler goroutine) after a pull that
	// brought in new commits from the remote. Wire it to wiki.TriggerResyncAsync
	// so the SQLite index is updated automatically after a remote push.
	OnRemoteChanges func()
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
	slog.Info("backup: initializing", "repoDir", repoDir, "remote", cfg.RemoteURL, "branch", cfg.Branch, "interval", cfg.Interval)

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
		if err := r.migrateBranchName(); err != nil {
			return nil, err
		}
		if cfg.RemoteURL != "" {
			if err := r.reconcileRemote(); err != nil {
				return nil, err
			}
		}
		// Ensure .gitignore exists even for repos created before this feature
		// was added, or if it was manually deleted.
		if err := EnsureGitignore(repoDir); err != nil {
			return nil, fmt.Errorf("failed to write .gitignore: %w", err)
		}
		return r, nil
	}

	slog.Debug("no existing repo found, initialising new one", "repoDir", repoDir, "openErr", err)

	// Initialize new repo with the configured branch name so local and remote
	// branch names always match — go-git's PlainInit defaults to "master".
	targetBranch := plumbing.NewBranchReferenceName(cfg.Branch)
	repo, err = gogit.PlainInitWithOptions(repoDir, &gogit.PlainInitOptions{
		InitOptions: gogit.InitOptions{DefaultBranch: targetBranch},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init repo: %w", err)
	}
	slog.Debug("new git repo initialised", "repoDir", repoDir, "branch", cfg.Branch)
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

// reconcileRemote ensures the stored 'origin' remote URL matches cfg.RemoteURL.
// Called when opening an existing repo so that a changed --git-backup-remote
// takes effect immediately rather than silently pushing to the old destination.
func (r *Repository) reconcileRemote() error {
	remote, err := r.repo.Remote("origin")
	if err != nil {
		// Remote doesn't exist yet; push() will create it on first use.
		return nil
	}
	if urls := remote.Config().URLs; len(urls) > 0 && urls[0] == r.cfg.RemoteURL {
		return nil // already up to date
	}
	if err := r.repo.DeleteRemote("origin"); err != nil {
		return fmt.Errorf("failed to remove stale remote: %w", err)
	}
	if _, err := r.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.cfg.RemoteURL},
	}); err != nil {
		return fmt.Errorf("failed to update remote URL: %w", err)
	}
	slog.Info("backup: updated 'origin' remote URL", "url", r.cfg.RemoteURL)
	return nil
}

// migrateBranchName renames the local branch to cfg.Branch when the repo was
// previously initialised with a different default (e.g. go-git's "master").
// This fixes the refSpec mismatch (refs/heads/master:refs/heads/main) that
// caused every pull to return nil instead of NoErrAlreadyUpToDate, and
// non-fast-forward push failures when the remote advanced between cycles.
func (r *Repository) migrateBranchName() error {
	head, err := r.repo.Head()
	if err != nil {
		// Empty repo or detached HEAD — nothing to rename yet.
		return nil
	}
	if !head.Name().IsBranch() {
		return nil
	}
	current := head.Name().Short()
	if current == r.cfg.Branch {
		return nil // already on the right branch
	}
	target := plumbing.NewBranchReferenceName(r.cfg.Branch)

	// Create target branch ref pointing at the same commit.
	if err := r.repo.Storer.SetReference(plumbing.NewHashReference(target, head.Hash())); err != nil {
		return fmt.Errorf("migrateBranchName: failed to create %s ref: %w", r.cfg.Branch, err)
	}
	// Repoint HEAD to the new branch.
	if err := r.repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, target)); err != nil {
		return fmt.Errorf("migrateBranchName: failed to update HEAD: %w", err)
	}
	// Remove old branch ref.
	if err := r.repo.Storer.RemoveReference(head.Name()); err != nil {
		slog.Warn("migrateBranchName: could not remove old branch ref", "branch", current, "error", err)
	}
	slog.Info("backup: renamed local branch", "from", current, "to", r.cfg.Branch)
	return nil
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
		if _, err := wt.Add(filepath.ToSlash(rootRel)); err != nil {
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
		if _, err := wt.Add(filepath.ToSlash(assetsRel)); err != nil {
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

// RunBackup pulls from the remote (fast-forward only) to integrate any external
// commits, then stages all changes in root/ and assets/, commits if anything
// changed, and pushes to the configured remote.
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

	// Pull remote changes before staging so our subsequent push is always a
	// fast-forward. This handles the case where the remote was modified externally
	// (e.g. a README committed via the GitHub UI).
	if r.cfg.RemoteURL != "" {
		if err := r.pullBeforeBackup(wt); err != nil {
			return err
		}
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
		if _, err := wt.Add(filepath.ToSlash(rootRel)); err != nil {
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
		if _, err := wt.Add(filepath.ToSlash(assetsRel)); err != nil {
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
		// Push only if there are genuinely unpushed local commits (e.g. the initial
		// commit from Init() that was never pushed yet). After a successful pull or
		// push, lastPushedHash equals local HEAD so this is a no-op — prevents the
		// spurious non-fast-forward push errors when the remote advances between cycles.
		if r.cfg.RemoteURL != "" {
			localHead, err := r.repo.Head()
			if err == nil && localHead.Hash() != r.lastPushedHash {
				slog.Debug("RunBackup: pushing unpushed local commit", "commit", localHead.Hash().String())
				if err := r.push(localHead.Hash().String()); err != nil {
					r.status.SetError(err.Error())
					return fmt.Errorf("push failed: %w", err)
				}
			}
		}
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

	// Each commit adds several loose objects; track them and GC when warranted.
	// A typical commit touches ~3–5 objects (tree + blobs); use 10 as a
	// conservative per-commit estimate so GC fires after ~50 commits.
	r.looseObjsSinceGC += 10
	r.maybeGC()

	// Push to remote
	if r.cfg.RemoteURL != "" {
		slog.Debug("RunBackup: pushing to remote", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commit.String())
		if err := r.push(commit.String()); err != nil {
			slog.Debug("RunBackup: push failed", "error", err)
			r.status.SetError(err.Error())
			return fmt.Errorf("push failed: %w", err)
		}
	} else {
		slog.Info("backup: committed locally (no remote configured)", "commit", commit.String())
	}
	r.status.SetSuccess(time.Now())
	return nil
}

// maybeGC runs gc() once the estimated loose-object count exceeds the
// threshold. The count is an approximation (10 objects per commit); a real
// count would require iterating the object store on every cycle, which is
// expensive. An occasional unnecessary GC is harmless.
// The counter is only reset on success so that a persistent repack failure
// retries after the next threshold is reached rather than silently never GC-ing.
func (r *Repository) maybeGC() {
	if r.looseObjsSinceGC < gcLooseThreshold {
		return
	}
	if r.gc() {
		r.looseObjsSinceGC = 0
	}
}

// gc packs all loose objects into a single packfile and then deletes the loose
// files. This is the go-git equivalent of `git gc --auto`.
// Errors are logged but not propagated: a failed GC is not a backup failure.
// Returns true on success so maybeGC can decide whether to reset the counter.
func (r *Repository) gc() bool {
	slog.Info("backup gc: starting — repacking loose objects")

	los, ok := r.repo.Storer.(storer.LooseObjectStorer)
	if !ok {
		slog.Debug("backup gc: storer does not support loose object enumeration, skipping prune")
		return true
	}

	// Enumerate loose objects BEFORE calling RepackObjects so the set we pack
	// and the set we prune are exactly the same. Objects added during a
	// concurrent operation (shouldn't happen on the single scheduler goroutine,
	// but defensive) won't appear in toDelete and won't be touched.
	var toDelete []plumbing.Hash
	if err := los.ForEachObjectHash(func(h plumbing.Hash) error {
		toDelete = append(toDelete, h)
		return nil
	}); err != nil {
		slog.Warn("backup gc: failed to enumerate loose objects", "error", err)
		return false
	}

	if len(toDelete) == 0 {
		slog.Debug("backup gc: no loose objects to repack")
		return true
	}

	if err := r.repo.RepackObjects(&gogit.RepackConfig{}); err != nil {
		slog.Warn("backup gc: repack failed", "error", err)
		return false
	}

	deleted := 0
	for _, h := range toDelete {
		if err := los.DeleteLooseObject(h); err != nil {
			slog.Warn("backup gc: failed to delete loose object", "hash", h, "error", err)
		} else {
			deleted++
		}
	}
	slog.Info("backup gc: completed", "packed_and_pruned", deleted)
	return true
}

// pullBeforeBackup fetches from the remote and fast-forward merges any new
// commits before we stage and commit wiki changes. This ensures our subsequent
// push is always a fast-forward, even when the remote was modified externally.
//
// Error semantics:
//   - nil / NoErrAlreadyUpToDate → proceed normally
//   - ErrNonFastForwardUpdate    → local has unpushed commits AND remote diverged → NeedsIntervention
//   - ErrUnstagedChanges         → same file changed on remote and in local wiki  → NeedsIntervention
//   - ErrReferenceNotFound       → remote branch does not exist yet (first push)  → skip pull
//   - other                      → transient error (network / auth)               → SetError, return
func (r *Repository) pullBeforeBackup(wt *gogit.Worktree) error {
	// Ensure "origin" remote exists before calling Pull.
	if _, err := r.repo.Remote("origin"); err != nil {
		if _, err2 := r.repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{r.cfg.RemoteURL},
		}); err2 != nil {
			r.status.SetError(fmt.Sprintf("failed to create remote before pull: %v", err2))
			return fmt.Errorf("failed to create remote before pull: %w", err2)
		}
		slog.Debug("pullBeforeBackup: created remote 'origin'", "url", r.cfg.RemoteURL)
	}

	auth, err := r.buildSSHAuth()
	if err != nil {
		r.status.SetError(fmt.Sprintf("failed to build SSH auth for pre-backup pull: %v", err))
		return fmt.Errorf("failed to build SSH auth for pre-backup pull: %w", err)
	}

	pullCtx, pullCancel := context.WithTimeout(context.Background(), networkTimeout)
	defer pullCancel()

	slog.Debug("pullBeforeBackup: pulling from remote", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch)
	pullErr := wt.PullContext(pullCtx, &gogit.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(r.cfg.Branch),
		Auth:          auth,
	})

	switch {
	case pullErr == nil:
		if head, err := r.repo.Head(); err == nil {
			slog.Info("pullBeforeBackup: pulled remote changes", "head", head.Hash().String())
			// Remote is now at this HEAD — no push needed until we make new local commits.
			r.lastPushedHash = head.Hash()
		} else {
			slog.Info("pullBeforeBackup: pulled remote changes")
		}
		if r.OnRemoteChanges != nil {
			r.OnRemoteChanges()
		}
		return nil

	case errors.Is(pullErr, gogit.NoErrAlreadyUpToDate):
		slog.Debug("pullBeforeBackup: already up-to-date, no pull needed")
		return nil

	case errors.Is(pullErr, plumbing.ErrReferenceNotFound),
		errors.Is(pullErr, transport.ErrEmptyRemoteRepository):
		// Remote branch (or entire repo) does not exist yet — this is the first push. Skip pull.
		slog.Debug("pullBeforeBackup: remote has no commits yet, skipping pull (first push)", "branch", r.cfg.Branch)
		return nil

	case errors.Is(pullErr, gogit.ErrNonFastForwardUpdate):
		// The remote has commits that cannot be fast-forwarded onto local history.
		// The local backup repo is authoritative (it holds all wiki commits), so the
		// correct recovery is to overwrite the remote manually:
		//   git -C <repoDir> push --force origin HEAD:<branch>
		msg := "remote has diverged from local backup history; " +
			"to recover, run: git -C " + r.repoDir + " push --force origin HEAD:" + r.cfg.Branch
		slog.Error("pullBeforeBackup: "+msg, "remote", r.cfg.RemoteURL)
		r.status.SetNeedsIntervention(msg)
		return fmt.Errorf("%s", msg)

	case errors.Is(pullErr, gogit.ErrUnstagedChanges):
		// A file was modified both on the remote and in the local wiki (disk has
		// a dirty version that pull cannot safely overwrite). The next backup cycle
		// will retry; if the remote change should be discarded, reset that file on
		// the remote repo and trigger a new backup.
		msg := "pull conflict: a wiki file has been modified both on the remote and locally; " +
			"reset the conflicting file on the remote or wait for the next backup cycle to retry"
		slog.Error("pullBeforeBackup: "+msg, "remote", r.cfg.RemoteURL)
		r.status.SetNeedsIntervention(msg)
		return fmt.Errorf("%s", msg)

	default:
		errMsg := fmt.Sprintf("failed to pull from remote before backup: %v", pullErr)
		slog.Error(errMsg, "remote", r.cfg.RemoteURL)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to pull from remote: %w", pullErr)
	}
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
	pushCtx, pushCancel := context.WithTimeout(context.Background(), networkTimeout)
	defer pushCancel()
	err = remote.PushContext(pushCtx, &gogit.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already up-to-date") {
			slog.Debug("push: remote already up-to-date", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch)
			r.lastPushedHash = localHead.Hash()
			return nil
		}
		slog.Error("git push failed", "error", err, "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "refSpec", string(refSpec))
		return fmt.Errorf("failed to push: %w", err)
	}
	r.lastPushedHash = localHead.Hash()
	slog.Info("backup: pushed to remote", "remote", r.cfg.RemoteURL, "branch", r.cfg.Branch, "commit", commitHash)
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

	// Use known hosts file for MITM protection.
	// Priority: explicit config path → ~/.ssh/known_hosts → InsecureIgnoreHostKey (warn).
	// If the configured path is unreadable we fail hard rather than silently downgrading.
	if r.cfg.SSHKnownHostsPath != "" {
		cb, err := ssh.NewKnownHostsCallback(r.cfg.SSHKnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts file %q: %w", r.cfg.SSHKnownHostsPath, err)
		}
		auth.HostKeyCallback = cb
		slog.Debug("buildSSHAuth: using configured known_hosts", "path", r.cfg.SSHKnownHostsPath)
	} else if defaultPath, ok := defaultKnownHostsPath(); ok {
		cb, err := ssh.NewKnownHostsCallback(defaultPath)
		if err != nil {
			slog.Warn("buildSSHAuth: default known_hosts found but could not be loaded; SSH host key verification disabled (MITM risk)",
				"path", defaultPath, "error", err)
			auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
		} else {
			auth.HostKeyCallback = cb
			slog.Info("buildSSHAuth: using default known_hosts for SSH host key verification", "path", defaultPath)
		}
	} else {
		slog.Warn("buildSSHAuth: no known_hosts file configured or found at ~/.ssh/known_hosts; SSH connections will not verify host keys (MITM risk) — set --git-backup-ssh-known-hosts to fix")
		auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
	}
	return auth, nil
}

// defaultKnownHostsPath returns the path to the user's default known_hosts file
// and whether it actually exists on disk.
func defaultKnownHostsPath() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	p := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// Status returns a snapshot of the last backup time and any error.
func (r *Repository) Status() StatusSnapshot {
	return r.status.Snapshot()
}
