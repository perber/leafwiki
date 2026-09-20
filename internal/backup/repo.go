package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// gcLooseThreshold is the number of loose objects that triggers a gc() run.
// git itself defaults to 6700; we use a lower value because the backup repo
// accumulates objects predictably and we prefer smaller, more frequent packs.
const (
	gcLooseThreshold        = 500
	networkTimeout          = 2 * time.Minute
	errWriteGitignoreFailed = "failed to write .gitignore: %w"
	errCommitFailed         = "failed to commit: %w"
)

// errRemoteBranchNotFound is returned by initWithRemoteHistory when the remote
// repository has commits on other branches but not on cfg.Branch. The caller
// (Init) treats it identically to transport.ErrEmptyRemoteRepository: initialise
// a fresh local repo and let the first push create the branch.
var errRemoteBranchNotFound = errors.New("remote branch not found")

// Repository wraps a git repository with backup-specific state.
type Repository struct {
	mu               sync.Mutex // serialises RunBackup and ForcePush so the HTTP handler can't race the scheduler goroutine
	cfg              Config
	repoDir          string
	repo             *gogit.Repository
	status           *Status
	looseObjsSinceGC int
	lastPushedHash   plumbing.Hash // hash of the last commit successfully pushed; zero = never pushed

	// afterListBeforeFetch, when set, runs right after pullBeforeBackup lists the
	// remote branch tip and before it fetches it. It exists only so tests can
	// deterministically simulate a remote history rewrite landing in that window
	// (see TestPull_RemoteRewrittenBetweenListAndFetch); production code never
	// sets it.
	afterListBeforeFetch func()

	// liveFSCaseInsensitive records whether repoDir's filesystem folds case
	// (the default on Windows and macOS), probed once at Init. materialize
	// uses it to avoid treating two differently-cased paths that the OS
	// resolves to the same physical file as unrelated.
	liveFSCaseInsensitive bool

	// syncedContentOnInit records whether this Init() call wrote files to
	// RootDir/AssetsDir via syncContentFromRemote (first contact with a
	// remote that already had history). See SyncedContentOnInit.
	syncedContentOnInit bool
}

// SyncedContentOnInit reports whether Init materialized remote content onto
// disk (first contact with a remote that already had history) without going
// through the normal wiki write path. A caller whose wiki tree/SQLite index
// may already be loaded (i.e. anything other than process startup, before
// wiki.NewWiki runs) must trigger a resync when this is true, or the synced
// pages exist on disk but stay invisible in search/tags/links/nav.
func (r *Repository) SyncedContentOnInit() bool {
	return r.syncedContentOnInit
}

// probeCaseInsensitiveFilesystem reports whether dir's filesystem folds case.
// It writes a throwaway file rather than relying on GOOS, since e.g. macOS can
// be configured case-sensitive. A probe failure (e.g. read-only dir) is
// treated as case-sensitive, the safer default (Linux behaviour).
func probeCaseInsensitiveFilesystem(dir string) bool {
	probe := filepath.Join(dir, ".leafwiki-case-probe")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		return false
	}
	defer func() { _ = os.Remove(probe) }()
	_, err := os.Stat(filepath.Join(dir, ".LEAFWIKI-CASE-PROBE"))
	return err == nil
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

	normalizedPath, err := normalizeBackupPath(cfg.Path)
	if err != nil {
		return nil, err
	}
	cfg.Path = normalizedPath

	repoDir := filepath.Dir(filepath.Clean(cfg.RootDir))
	slog.Info("backup: initializing", "repoDir", repoDir, "path", cfg.Path, "remote", redactRemote(cfg.RemoteURL), "branch", cfg.Branch, "interval", cfg.Interval)

	// Ensure parent directory exists
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo directory: %w", err)
	}

	r := &Repository{
		cfg:                   cfg,
		repoDir:               repoDir,
		status:                &Status{},
		liveFSCaseInsensitive: probeCaseInsensitiveFilesystem(repoDir),
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
			return nil, fmt.Errorf(errWriteGitignoreFailed, err)
		}
		return r, nil
	}

	slog.Debug("no existing repo found, initialising new one", "repoDir", repoDir, "openErr", err)

	// If a remote is configured, fetch the remote history before initialising so
	// the local repo shares ancestry with the remote. This avoids the
	// unrelated-histories / NeedsIntervention trap when the .git folder was deleted
	// while the remote already has content.
	//
	// We deliberately do NOT clone (PlainClone checks out the remote's files,
	// which would overwrite local wiki content with an older remote version).
	// Instead we init + fetch + repoint the local branch, then additively sync
	// the remote content into the live dirs (local files win).
	if cfg.RemoteURL != "" {
		fetched, fetchErr := r.initWithRemoteHistory(repoDir)
		if fetchErr == nil {
			slog.Info("backup: adopted remote history", "remote", redactRemote(cfg.RemoteURL), "branch", cfg.Branch)
			r.repo = fetched
			// Mark remote HEAD as already-pushed; first RunBackup will only push
			// genuinely new local changes on top of the fetched history.
			if head, hErr := fetched.Head(); hErr == nil {
				r.lastPushedHash = head.Hash()
				// First contact with an existing backup: bring its content down before
				// the wiki loads its tree. Otherwise the first backup would treat every
				// remote-only file as a local deletion and wipe the remote, and the
				// wiki would seed a welcome page on top of the restored content.
				synced, err := r.syncContentFromRemote(head.Hash())
				if err != nil {
					return nil, fmt.Errorf("failed to sync content from remote: %w", err)
				}
				// Reconfigure (settings UI, live process) can hit this same path
				// against an already-running wiki whose tree/SQLite index was
				// loaded before these files landed on disk — SyncedContentOnInit
				// lets the caller trigger a resync so the index catches up.
				r.syncedContentOnInit = synced > 0
			}
			if err := EnsureGitignore(repoDir); err != nil {
				return nil, fmt.Errorf(errWriteGitignoreFailed, err)
			}
			return r, nil
		}
		if !errors.Is(fetchErr, transport.ErrEmptyRemoteRepository) && !errors.Is(fetchErr, errRemoteBranchNotFound) {
			return nil, fmt.Errorf("failed to fetch remote history from %s: %w", redactRemote(cfg.RemoteURL), fetchErr)
		}
		// initWithRemoteHistory created a partial .git — remove it before plain init.
		_ = os.RemoveAll(filepath.Join(repoDir, ".git"))
		slog.Info("backup: remote empty or branch missing, initialising local repo", "remote", redactRemote(cfg.RemoteURL))
	}

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
		return nil, fmt.Errorf(errWriteGitignoreFailed, err)
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
	slog.Info("backup: updated 'origin' remote URL", "url", r.remoteForLog())
	return nil
}

// initWithRemoteHistory initialises a fresh git repo at repoDir, fetches the
// remote history into it, and repoints the local branch at the remote HEAD —
// without touching the working tree. Local wiki files are never overwritten.
// Returns transport.ErrEmptyRemoteRepository when the remote has no commits yet.
func (r *Repository) initWithRemoteHistory(repoDir string) (*gogit.Repository, error) {
	gitDir := filepath.Join(repoDir, ".git")

	// Clean up the .git directory on any failure so the caller's fallback path
	// (PlainInitWithOptions) never hits "repository already exists".
	var repo *gogit.Repository
	var returnErr error
	defer func() {
		if returnErr != nil {
			_ = os.RemoveAll(gitDir)
		}
	}()

	targetBranch := plumbing.NewBranchReferenceName(r.cfg.Branch)

	repo, returnErr = gogit.PlainInitWithOptions(repoDir, &gogit.PlainInitOptions{
		InitOptions: gogit.InitOptions{DefaultBranch: targetBranch},
	})
	if returnErr != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: init: %w", returnErr)
		return nil, returnErr
	}

	if _, returnErr = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.cfg.RemoteURL},
	}); returnErr != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: create remote: %w", returnErr)
		return nil, returnErr
	}

	auth, returnErr := r.buildAuth()
	if returnErr != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: auth: %w", returnErr)
		return nil, returnErr
	}

	remote, _ := repo.Remote("origin")
	listCtx, listCancel := context.WithTimeout(context.Background(), networkTimeout)
	defer listCancel()
	refs, err := remote.ListContext(listCtx, &gogit.ListOptions{Auth: auth})
	if err != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: list remote: %w", err)
		return nil, returnErr
	}

	// Find the remote branch HEAD hash.
	var remoteHead plumbing.Hash
	for _, ref := range refs {
		if ref.Name() == targetBranch {
			remoteHead = ref.Hash()
			break
		}
	}
	if remoteHead.IsZero() {
		if len(refs) == 0 {
			returnErr = transport.ErrEmptyRemoteRepository
		} else {
			slog.Info("backup: target branch not found on remote, will be created on first push", "branch", r.cfg.Branch)
			returnErr = fmt.Errorf("%w %q", errRemoteBranchNotFound, r.cfg.Branch)
		}
		return nil, returnErr
	}

	// Fetch only the target branch so we get the full object graph.
	refSpec := config.RefSpec("refs/heads/" + r.cfg.Branch + ":refs/heads/" + r.cfg.Branch)
	fetchCtx, fetchCancel := context.WithTimeout(context.Background(), networkTimeout)
	defer fetchCancel()
	if err := remote.FetchContext(fetchCtx, &gogit.FetchOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
	}); err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		returnErr = fmt.Errorf("initWithRemoteHistory: fetch: %w", err)
		return nil, returnErr
	}

	// Repoint HEAD → local branch → remote HEAD commit (no checkout).
	if err := repo.Storer.SetReference(plumbing.NewHashReference(targetBranch, remoteHead)); err != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: set branch ref: %w", err)
		return nil, returnErr
	}
	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, targetBranch)); err != nil {
		returnErr = fmt.Errorf("initWithRemoteHistory: set HEAD: %w", err)
		return nil, returnErr
	}

	return repo, nil
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

// makeInitialCommit creates the first commit from the live root/ and assets/
// directories when a fresh repository (or one adopted from an empty remote) has
// no history yet. Content is spliced in at Config.ContentTreePaths() rather than
// staged through the working tree.
func (r *Repository) makeInitialCommit() error {
	slog.Debug("makeInitialCommit: starting")

	existing, err := r.headCommit()
	if err != nil {
		return err
	}
	if existing != nil {
		return nil // already has history
	}

	replacements, err := r.buildContentReplacements()
	if err != nil {
		return err
	}
	treeHash, present, err := r.spliceTree(plumbing.ZeroHash, replacements)
	if err != nil {
		return err
	}
	if !present {
		slog.Debug("makeInitialCommit: no files found in root or assets, skipping initial commit")
		return nil
	}

	hash, err := r.commitTree(treeHash, nil, "Initial commit")
	if err != nil {
		return fmt.Errorf(errCommitFailed, err)
	}
	if err := r.setHead(hash); err != nil {
		return err
	}
	slog.Debug("makeInitialCommit: initial commit created", "hash", hash.String())

	if r.cfg.RemoteURL != "" {
		slog.Debug("makeInitialCommit: initial commit will be pushed by the next backup cycle", "remote", r.remoteForLog())
	}
	return nil
}

// Pull fetches from the remote and fast-forward merges any new commits,
// independent of a full backup cycle. Same error/conflict semantics as
// pullBeforeBackup: conflicts and diverged history set r.status.NeedsIntervention,
// transient errors set r.status.LastError. A no-op (nil error) if no remote is
// configured. Fails fast (rather than blocking) if a backup cycle is already
// running, mirroring ForcePush — RunBackup can hold the lock for minutes across
// its own network pull/push, which would otherwise stall the HTTP request.
func (r *Repository) Pull() error {
	if !r.mu.TryLock() {
		return fmt.Errorf("backup is currently running — try again in a moment")
	}
	defer r.mu.Unlock()

	if r.cfg.RemoteURL == "" {
		return nil
	}

	if err := r.pullBeforeBackup(); err != nil {
		return err
	}

	// A successful pull resolves any stale error/conflict state from a prior
	// cycle (e.g. the file conflict that caused it no longer exists on disk).
	// Unlike SetSuccess, this must not touch LastBackupAt — a pull is not a
	// backup, so it shouldn't be reported as one.
	r.status.ClearIntervention()
	return nil
}

// RunBackup integrates remote changes, then commits the live root/ and assets/
// directories into the repository tree (spliced under Config.ContentTreePaths())
// and pushes to the configured remote.
// message format: "backup: <RFC3339 timestamp>"
// Returns nil and skips commit+push if the content is unchanged.
func (r *Repository) RunBackup() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	slog.Debug("RunBackup: starting backup cycle")

	// Integrate remote changes first so our subsequent push is always a
	// fast-forward. This handles the case where the remote was modified
	// externally (e.g. a README or wiki page committed via the GitHub UI).
	if r.cfg.RemoteURL != "" {
		if err := r.pullBeforeBackup(); err != nil {
			return err
		}
	}

	baseCommit, err := r.headCommit()
	if err != nil {
		errMsg := fmt.Errorf("failed to resolve HEAD: %w", err).Error()
		slog.Debug("RunBackup: failed to resolve HEAD", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to resolve HEAD: %w", err)
	}

	var baseTree plumbing.Hash
	var parents []plumbing.Hash
	if baseCommit != nil {
		baseTree = baseCommit.TreeHash
		parents = []plumbing.Hash{baseCommit.Hash}
	}

	replacements, err := r.buildContentReplacements()
	if err != nil {
		errMsg := fmt.Errorf("failed to read content directories: %w", err).Error()
		slog.Debug("RunBackup: failed to read content directories", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to read content directories: %w", err)
	}
	newTree, present, err := r.spliceTree(baseTree, replacements)
	if err != nil {
		errMsg := fmt.Errorf("failed to build content tree: %w", err).Error()
		slog.Debug("RunBackup: failed to build content tree", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to build content tree: %w", err)
	}

	if !present {
		if baseCommit == nil {
			// Fresh repository with no content to back up yet.
			slog.Info("backup skipped - no content in root or assets yet")
			r.status.SetSuccess(time.Now())
			return nil
		}
		// All content was removed: commit an empty tree so the deletion is backed up.
		empty, err := r.emptyTreeHash()
		if err != nil {
			errMsg := fmt.Errorf("failed to build empty tree: %w", err).Error()
			r.status.SetError(errMsg)
			return fmt.Errorf("failed to build empty tree: %w", err)
		}
		newTree = empty
	}

	if baseCommit != nil && newTree == baseTree {
		slog.Info("backup skipped - no changes in content directories")
		// Push only if there are genuinely unpushed local commits (e.g. the initial
		// commit from Init() that was never pushed yet). After a successful pull or
		// push, lastPushedHash equals local HEAD so this is a no-op — prevents the
		// spurious non-fast-forward push errors when the remote advances between cycles.
		if r.cfg.RemoteURL != "" {
			localHead, err := r.repo.Head()
			if err == nil && localHead.Hash() != r.lastPushedHash {
				slog.Debug("RunBackup: pushing unpushed local commit", "commit", localHead.Hash().String())
				if err := r.push(false); err != nil {
					r.status.SetError(err.Error())
					return fmt.Errorf("push failed: %w", err)
				}
			}
		}
		r.status.SetSuccess(time.Now())
		return nil
	}

	commitMsg := fmt.Sprintf("backup: %s", time.Now().Format(time.RFC3339))
	slog.Debug("RunBackup: committing content", "message", commitMsg, "author", r.cfg.AuthorName, "email", r.cfg.AuthorEmail, "path", r.cfg.Path)
	commitHash, err := r.commitTree(newTree, parents, commitMsg)
	if err != nil {
		errMsg := fmt.Errorf(errCommitFailed, err).Error()
		slog.Debug("RunBackup: commit failed", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf(errCommitFailed, err)
	}
	if err := r.setHead(commitHash); err != nil {
		errMsg := fmt.Errorf("failed to update branch ref: %w", err).Error()
		slog.Debug("RunBackup: failed to update branch ref", "error", errMsg)
		r.status.SetError(errMsg)
		return fmt.Errorf("failed to update branch ref: %w", err)
	}
	slog.Debug("RunBackup: commit created", "hash", commitHash.String(), "message", commitMsg)

	// Each commit adds several loose objects; track them and GC when warranted.
	// A typical commit touches several objects (trees + blobs); use 10 as a
	// conservative per-commit estimate so GC fires after ~50 commits.
	r.looseObjsSinceGC += 10
	r.maybeGC()

	if r.cfg.RemoteURL != "" {
		slog.Debug("RunBackup: pushing to remote", "remote", r.remoteForLog(), "branch", r.cfg.Branch, "commit", commitHash.String())
		if err := r.push(false); err != nil {
			slog.Debug("RunBackup: push failed", "error", err)
			r.status.SetError(err.Error())
			return fmt.Errorf("push failed: %w", err)
		}
	} else {
		slog.Info("backup: committed locally (no remote configured)", "commit", commitHash.String())
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

// pullBeforeBackup fetches the remote branch and integrates any new commits
// before we build our own, so the subsequent push is always a fast-forward even
// when the remote was modified externally.
//
// Unlike the historical working-tree merge, this fetches objects and
// fast-forwards the branch ref with git plumbing, then materializes the remote
// content subtree into the live root/ and assets/ directories. Sibling files in
// a monorepo prefix are never written to disk.
//
// Error semantics:
//   - remote branch missing / empty remote        → first push, not an error
//   - already up to date / local ahead            → no-op
//   - diverged history                            → NeedsIntervention
//   - same file changed remotely and dirty locally → NeedsIntervention
//   - other (network / auth)                      → SetError, return
func (r *Repository) pullBeforeBackup() error {
	remote, err := r.ensureRemote()
	if err != nil {
		return err
	}

	auth, err := r.buildAuth()
	if err != nil {
		return r.failWith(fmt.Sprintf("failed to build auth for pre-backup pull: %v", err))
	}

	remoteHead, err := r.listRemoteBranch(remote, auth)
	if err != nil {
		msg := fmt.Sprintf("failed to query remote before backup: %v", err)
		slog.Error(msg, "remote", r.remoteForLog())
		return r.failWith(msg)
	}
	if remoteHead.IsZero() {
		slog.Debug("pullBeforeBackup: remote branch not present yet, skipping pull (first push)", "branch", r.cfg.Branch)
		return nil
	}

	localCommit, err := r.headCommit()
	if err != nil {
		return r.failWith(fmt.Sprintf("failed to resolve local HEAD before pull: %v", err))
	}
	if localCommit != nil && localCommit.Hash == remoteHead {
		slog.Debug("pullBeforeBackup: already up-to-date, no pull needed")
		return nil
	}

	if r.afterListBeforeFetch != nil {
		r.afterListBeforeFetch()
	}

	if err := r.fetchBranch(remote, auth); err != nil {
		msg := fmt.Sprintf("failed to fetch from remote before backup: %v", err)
		slog.Error(msg, "remote", r.remoteForLog())
		return r.failWith(msg)
	}

	// Re-resolve the branch tip from what fetchBranch actually fetched into the
	// tracking ref, rather than trusting the value listed before the fetch: if
	// the remote branch was rewritten concurrently (e.g. a force-push landing
	// between listRemoteBranch and here), the commit remoteHead pointed to may
	// never have been fetched at all, while the tracking ref always reflects
	// whatever this fetch actually brought down.
	tracking := plumbing.NewRemoteReferenceName("origin", r.cfg.Branch)
	trackingRef, err := r.repo.Reference(tracking, true)
	if err != nil {
		return r.failWith(fmt.Sprintf("failed to resolve fetched remote branch: %v", err))
	}
	remoteHead = trackingRef.Hash()

	remoteCommit, err := r.repo.CommitObject(remoteHead)
	if err != nil {
		return r.failWith(fmt.Sprintf("failed to read fetched remote commit: %v", err))
	}

	if localCommit != nil {
		behind, err := localCommit.IsAncestor(remoteCommit)
		if err != nil {
			return r.failWith(fmt.Sprintf("failed to compare local and remote history: %v", err))
		}
		if !behind {
			ahead, aErr := remoteCommit.IsAncestor(localCommit)
			if aErr == nil && ahead {
				slog.Debug("pullBeforeBackup: local history is ahead of remote, nothing to pull")
				return nil
			}
			msg := divergenceMessage(r.repoDir, r.cfg.Branch)
			slog.Error("pullBeforeBackup: "+msg, "remote", r.remoteForLog())
			return r.failNeedsIntervention(msg)
		}
	}

	if err := r.materializeContent(localCommit, remoteCommit); err != nil {
		if errors.Is(err, errPullConflict) {
			return err // materializeContent already set NeedsIntervention
		}
		msg := fmt.Sprintf("failed to update local content from remote: %v", err)
		slog.Error(msg, "remote", r.remoteForLog())
		return r.failWith(msg)
	}

	if err := r.setHead(remoteHead); err != nil {
		return r.failWith(fmt.Sprintf("failed to update local branch after pull: %v", err))
	}
	r.lastPushedHash = remoteHead
	slog.Info("pullBeforeBackup: pulled remote changes", "head", remoteHead.String())
	return nil
}

// ensureRemote returns the "origin" remote, creating it from cfg if absent.
func (r *Repository) ensureRemote() (*gogit.Remote, error) {
	if remote, err := r.repo.Remote("origin"); err == nil {
		return remote, nil
	}
	if _, err := r.repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{r.cfg.RemoteURL},
	}); err != nil {
		return nil, r.failWith(fmt.Sprintf("failed to create remote before pull: %v", err))
	}
	remote, err := r.repo.Remote("origin")
	if err != nil {
		return nil, r.failWith(fmt.Sprintf("failed to get remote before pull: %v", err))
	}
	slog.Debug("pullBeforeBackup: created remote 'origin'", "url", r.remoteForLog())
	return remote, nil
}

// listRemoteBranch returns the remote's hash for the configured branch, or the
// zero hash when the branch (or the whole remote) does not exist yet.
func (r *Repository) listRemoteBranch(remote *gogit.Remote, auth transport.AuthMethod) (plumbing.Hash, error) {
	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()
	refs, err := remote.ListContext(ctx, &gogit.ListOptions{Auth: auth})
	if err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return plumbing.ZeroHash, nil
		}
		return plumbing.ZeroHash, err
	}
	target := plumbing.NewBranchReferenceName(r.cfg.Branch)
	for _, ref := range refs {
		if ref.Name() == target {
			return ref.Hash(), nil
		}
	}
	return plumbing.ZeroHash, nil
}

// fetchBranch fetches the configured branch's objects into a remote-tracking
// ref. The working tree is never touched. A force refspec keeps the tracking ref
// current even if the remote was rewritten; push() removes it before pushing.
func (r *Repository) fetchBranch(remote *gogit.Remote, auth transport.AuthMethod) error {
	tracking := plumbing.NewRemoteReferenceName("origin", r.cfg.Branch)
	refSpec := config.RefSpec("+" + plumbing.NewBranchReferenceName(r.cfg.Branch).String() + ":" + tracking.String())
	ctx, cancel := context.WithTimeout(context.Background(), networkTimeout)
	defer cancel()
	slog.Debug("pullBeforeBackup: fetching", "remote", r.remoteForLog(), "branch", r.cfg.Branch)
	err := remote.FetchContext(ctx, &gogit.FetchOptions{Auth: auth, RefSpecs: []config.RefSpec{refSpec}})
	if err == nil || errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return nil
	}
	if errors.Is(err, plumbing.ErrReferenceNotFound) || errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return nil // branch vanished between list and fetch — nothing to pull
	}
	return err
}

func divergenceMessage(repoDir, branch string) string {
	return "remote has diverged from local backup history; " +
		"to recover, run: git -C " + repoDir + " push --force origin HEAD:" + branch
}

// push pushes the current local HEAD to the configured remote.
// If force is true, a non-fast-forward push is allowed (overwrites remote history).
func (r *Repository) push(force bool) error {
	slog.Debug("push: starting", "force", force, "remote", r.remoteForLog(), "branch", r.cfg.Branch)

	auth, err := r.buildAuth()
	if err != nil {
		return fmt.Errorf("failed to build auth: %w", err)
	}

	remote, err := r.repo.Remote("origin")
	if err != nil {
		slog.Debug("push: remote 'origin' not found, creating it", "url", r.remoteForLog())
		if _, err = r.repo.CreateRemote(&config.RemoteConfig{
			Name: "origin",
			URLs: []string{r.cfg.RemoteURL},
		}); err != nil {
			return fmt.Errorf("failed to create remote: %w", err)
		}
		remote, err = r.repo.Remote("origin")
		if err != nil {
			return fmt.Errorf("failed to get remote: %w", err)
		}
		slog.Debug("push: remote 'origin' created", "url", r.remoteForLog())
	}

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
	if rmErr := r.repo.Storer.RemoveReference(trackingRef); rmErr != nil && !errors.Is(rmErr, plumbing.ErrReferenceNotFound) {
		slog.Warn("push: could not remove stale remote tracking ref", "ref", trackingRef.String(), "error", rmErr)
	}

	// Use the resolved branch ref explicitly rather than HEAD.
	// Symbolic HEAD in a force refspec can confuse go-git when the local branch
	// name differs from the configured remote branch (e.g. local=master, remote=main).
	localBranchRef := localHead.Name().String()
	prefix := ""
	if force {
		prefix = "+"
	}
	refSpec := config.RefSpec(prefix + localBranchRef + ":refs/heads/" + r.cfg.Branch)
	slog.Debug("push: pushing", "refSpec", string(refSpec), "commit", localHead.Hash().String())

	pushCtx, pushCancel := context.WithTimeout(context.Background(), networkTimeout)
	defer pushCancel()
	err = remote.PushContext(pushCtx, &gogit.PushOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
		Force:    force,
	})
	if err != nil {
		if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			slog.Debug("push: remote already up-to-date")
			r.lastPushedHash = localHead.Hash()
			return nil
		}
		slog.Error("git push failed", "error", err, "remote", r.remoteForLog(), "branch", r.cfg.Branch, "refSpec", string(refSpec))
		return fmt.Errorf("failed to push: %w", err)
	}
	r.lastPushedHash = localHead.Hash()
	slog.Info("backup: pushed to remote", "remote", r.remoteForLog(), "branch", r.cfg.Branch, "commit", localHead.Hash().String(), "force", force)
	return nil
}

// redactRemote masks credentials embedded in a remote URL
// (https://user:token@host/repo.git) so the token never reaches a log line or a
// status message shown in the UI. Non-URL remotes (git@host:path) are returned
// unchanged — they carry no secret.
func redactRemote(remoteURL string) string {
	u, err := url.Parse(remoteURL)
	if err != nil || u.User == nil {
		return remoteURL
	}
	return u.Redacted()
}

// remoteForLog returns the configured remote with any embedded credentials
// masked. Use it everywhere the remote is logged or surfaced to the user.
func (r *Repository) remoteForLog() string {
	return redactRemote(r.cfg.RemoteURL)
}

// failWith records msg as the repository's error status and returns it as an
// error, replacing the "SetError then return" tail every pullBeforeBackup/
// ensureRemote/listRemoteBranch failure path repeats. It does not log — call
// sites that already logged before this call keep doing so explicitly, since
// not all of them did (this only collapses the status+return duplication, it
// doesn't change which paths log).
func (r *Repository) failWith(msg string) error {
	r.status.SetError(msg)
	return errors.New(msg)
}

// failNeedsIntervention is failWith's NeedsIntervention counterpart.
func (r *Repository) failNeedsIntervention(msg string) error {
	r.status.SetNeedsIntervention(msg)
	return errors.New(msg)
}

// buildAuth builds the transport authentication for the configured remote.
// The implementation lives in auth.go as package functions so TestRemote can
// reuse it; this method just binds it to the repository's own config.
func (r *Repository) buildAuth() (transport.AuthMethod, error) {
	return buildAuth(r.cfg)
}

// ForcePush overwrites the remote branch with the current local HEAD.
// Used to recover from NeedsIntervention state when the remote has diverged.
// Returns an error immediately if the scheduler goroutine is mid-RunBackup
// (rather than blocking the HTTP handler for up to 4 minutes).
func (r *Repository) ForcePush() error {
	if !r.mu.TryLock() {
		return fmt.Errorf("backup is currently running — try again in a moment")
	}
	defer r.mu.Unlock()

	if r.cfg.RemoteURL == "" {
		return fmt.Errorf("no remote configured")
	}

	if err := r.push(true); err != nil {
		r.status.SetError(fmt.Sprintf("force push failed: %v", err))
		return fmt.Errorf("force push failed: %w", err)
	}
	r.status.SetSuccess(time.Now())
	slog.Info("backup: force-pushed to remote", "remote", r.remoteForLog(), "branch", r.cfg.Branch)
	return nil
}

// Status returns a snapshot of the last backup time and any error.
func (r *Repository) Status() StatusSnapshot {
	return r.status.Snapshot()
}
