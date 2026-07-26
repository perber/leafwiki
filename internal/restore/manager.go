package restore

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// ErrRestoreAlreadyRunning is returned by TriggerRestore when a restore is
// already in progress. Fixed package-level LocalizedError, matching
// wikiresync.ErrResyncAlreadyRunning / snapshot.ErrAlreadyRunning.
var ErrRestoreAlreadyRunning = sharederrors.NewLocalizedError(
	"restore_already_running",
	"A restore is already in progress",
	"a restore is already in progress",
	nil,
)

// ErrRestoreNeedsIntervention is returned by TriggerRestore when a previous
// restore left the instance in a NeedsIntervention state. Starting a new
// restore on top of a half-swapped filesystem and orphaned .pre-restore-*
// copies would compound the inconsistency instead of resolving it — the only
// supported way out is self-restart (see Manager.SelfRestart / ADR-0009).
var ErrRestoreNeedsIntervention = sharederrors.NewLocalizedError(
	"restore_needs_intervention",
	"This instance needs attention before a new restore can be started — restart the server first",
	"a previous restore needs intervention before a new restore can be started",
	nil,
)

// ErrWritesDisabled is returned by the write-gate HTTP middleware
// (internal/http/middleware/maintenance) when a mutating request arrives
// while a restore is swapping files. Defined here, not hand-rolled in the
// middleware, so that response follows the same *sharederrors.LocalizedError
// convention as every other error this feature returns.
var ErrWritesDisabled = sharederrors.NewLocalizedError(
	"restore_writes_disabled",
	"A restore is in progress; writes are temporarily disabled",
	"a restore is in progress; writes are temporarily disabled",
	nil,
)

// gateDrainTimeout bounds how long the restore sequence waits, once the
// write gate is engaged, for requests already in flight (started just before
// Engage()) to finish before files are swapped out from under them. A
// timeout here is logged and the restore proceeds anyway rather than failing
// the whole operation over a slow request.
const gateDrainTimeout = 10 * time.Second

// DefaultMaxUploadSizeBytes is used by TriggerRestoreFromUpload's HTTP
// handler (POST /restore/upload) when Config.MaxUploadSizeBytes is unset.
const DefaultMaxUploadSizeBytes int64 = 500 * 1024 * 1024 // 500 MiB

type Manager struct {
	cfg Config
	job *Job
	// wg tracks the in-flight runLocked goroutine (if any), so callers that
	// need the process to shut down cleanly (main.go) can wait for a restore
	// to finish before closing the services it depends on (AuthService,
	// BrandingService) out from under it.
	wg sync.WaitGroup
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg, job: NewJob()}
}

// TriggerRestore starts a restore job asynchronously for the given snapshot
// id. Returns ErrRestoreAlreadyRunning if a restore is already in progress,
// or ErrRestoreNeedsIntervention if a previous restore left the instance in
// a state where a new one must not be started (see ErrRestoreNeedsIntervention).
func (m *Manager) TriggerRestore(id string) error {
	if m.job.Status().NeedsIntervention {
		return ErrRestoreNeedsIntervention
	}
	if !m.job.Start() {
		return ErrRestoreAlreadyRunning
	}
	m.wg.Go(func() {
		m.runLocked(id)
	})
	return nil
}

// TriggerRestoreFromUpload starts a restore job asynchronously from an
// uploaded ZIP, read in full from r. Returns the same errors as
// TriggerRestore (ErrRestoreAlreadyRunning / ErrRestoreNeedsIntervention),
// plus any error encountered while staging the upload to disk.
//
// Unlike TriggerRestore(id), staging happens synchronously before this
// method returns: r is expected to be backed by the inbound HTTP request's
// multipart body, which is only valid for the lifetime of that request — it
// must be fully drained into a durable temp file before control returns to
// the HTTP handler, since the actual restore then runs in a background
// goroutine after the handler (and its response) has already completed.
func (m *Manager) TriggerRestoreFromUpload(r io.Reader) error {
	if m.job.Status().NeedsIntervention {
		return ErrRestoreNeedsIntervention
	}
	if !m.job.Start() {
		return ErrRestoreAlreadyRunning
	}

	zipPath, cleanup, err := m.stageUpload(r)
	if err != nil {
		m.job.Finish(err)
		return err
	}

	m.wg.Go(func() {
		defer cleanup()
		m.runGuarded(func() { m.runFromZipPath(zipPath) })
	})
	return nil
}

// stageUpload drains r into a fresh temp file inside dataDir (the same
// filesystem extractAndValidate's staging directory uses, and for the same
// reason — so nothing downstream needs a cross-filesystem copy). The
// returned cleanup func removes the temp file; callers must invoke it
// exactly once, whether or not the restore that follows succeeds.
func (m *Manager) stageUpload(r io.Reader) (zipPath string, cleanup func(), err error) {
	f, err := os.CreateTemp(m.cfg.DataDir, ".leafwiki-restore-upload-*.zip")
	if err != nil {
		return "", nil, errStageUploadFailed(err)
	}
	cleanup = func() { _ = os.Remove(f.Name()) }

	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, errStageUploadFailed(err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, errStageUploadFailed(err)
	}
	return f.Name(), cleanup, nil
}

// errStageUploadFailed wraps any failure to drain an uploaded backup ZIP to a
// durable temp file (disk full, permission error, ...) as a LocalizedError,
// per this repo's convention that domain errors reaching an HTTP handler must
// be *sharederrors.LocalizedError, not a bare fmt.Errorf/errors.New. Unlike
// this file's other fmt.Errorf calls (which only ever feed the async
// job-status string), stageUpload's errors are returned synchronously to
// TriggerRestoreFromUpload's HTTP handler, which only renders a specific
// message for LocalizedError — anything else falls back to a generic
// "Restore request failed".
func errStageUploadFailed(cause error) error {
	return sharederrors.NewLocalizedError(
		"restore_upload_staging_failed",
		"Failed to save the uploaded backup to disk",
		"failed to stage uploaded backup: %s",
		cause,
		cause.Error(),
	)
}

// Status returns the current restore job state (thread-safe).
func (m *Manager) Status() JobStatus {
	return m.job.Status()
}

// MaxUploadSizeBytes returns the configured limit for POST /restore/upload,
// falling back to DefaultMaxUploadSizeBytes when Config.MaxUploadSizeBytes is
// unset (<= 0).
func (m *Manager) MaxUploadSizeBytes() int64 {
	if m.cfg.MaxUploadSizeBytes > 0 {
		return m.cfg.MaxUploadSizeBytes
	}
	return DefaultMaxUploadSizeBytes
}

// Wait blocks until any in-flight restore triggered via TriggerRestore has
// fully finished. Intended to be called during process shutdown, before
// closing services (AuthService, BrandingService) a running restore depends
// on — see cmd/leafwiki/main.go.
func (m *Manager) Wait() {
	m.wg.Wait()
}

// SelfRestart re-execs the current process. Callers (the HTTP handler) are
// expected to only allow this once Status().NeedsIntervention is true.
func (m *Manager) SelfRestart() error {
	return SelfRestart()
}

// runLocked resolves the given snapshot id to a zip path and runs the
// restore sequence against it (see runFromZipPath).
func (m *Manager) runLocked(id string) {
	m.runGuarded(func() {
		zipPath, err := m.cfg.SnapshotManager.SnapshotZipPath(id)
		if err != nil {
			m.job.SetPhase(PhaseValidating)
			m.job.Finish(err)
			return
		}
		m.runFromZipPath(zipPath)
	})
}

// runGuarded recovers from any panic raised by fn, treating it as
// NeedsIntervention: a panic mid-sequence means we don't know which phases
// actually completed, so failing safe (gate stays engaged, admin must
// self-restart) is the only sound response. Shared by both the by-id
// (runLocked) and by-upload (TriggerRestoreFromUpload) entrypoints.
func (m *Manager) runGuarded(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Default().Error("panic during restore", "panic", r)
			m.job.FinishNeedsIntervention(fmt.Errorf("panic during restore: %v", r))
		}
	}()
	fn()
}

// runFromZipPath performs the full validate -> gate -> swap -> reopen-auth ->
// invalidate-sessions -> reload-branding -> commit -> resync sequence against
// an already-resolved zip file on disk. Shared by the by-id and by-upload
// restore entrypoints once each has produced a concrete zipPath.
func (m *Manager) runFromZipPath(zipPath string) {
	m.job.SetPhase(PhaseValidating)
	stagingDir, meta, err := extractAndValidate(zipPath, m.cfg.DataDir)
	if err != nil {
		m.job.Finish(fmt.Errorf("snapshot validation failed: %w", err))
		return
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	if meta.Version != "" && m.cfg.WikiVersion != "" && meta.Version != m.cfg.WikiVersion {
		m.job.SetVersionWarning(fmt.Sprintf("snapshot was created by version %s, this server is running %s", meta.Version, m.cfg.WikiVersion))
	}

	m.job.SetPhase(PhaseSwapping)
	m.cfg.WriteGate.Engage()
	if !m.cfg.WriteGate.WaitForDrain(gateDrainTimeout) {
		slog.Default().Warn("restore: timed out waiting for in-flight requests to drain, proceeding anyway")
	}

	// AuthService's user store must release its OS-level handle on users.db
	// before SwapAll renames it — on Windows an open handle (even one a GET
	// request lazily reopened mid-swap) blocks the rename with a sharing
	// violation, which POSIX doesn't have. Nothing on disk has been touched
	// yet, so a failure here doesn't need a rollback — but PauseUserStoreForSwap
	// (UserStore.suspend) marks the store suspended even when it fails, so
	// AuthService is left unable to serve any request until something re-opens
	// it. Recover that here by re-opening a fresh store before reporting the
	// (retryable) failure — only if that recovery itself fails does this need
	// NeedsIntervention, since at that point AuthService has no working store.
	if m.cfg.AuthService != nil {
		if err := m.cfg.AuthService.PauseUserStoreForSwap(); err != nil {
			m.cfg.WriteGate.Disengage()
			if repErr := m.cfg.AuthService.ReplaceUserStore(m.cfg.DataDir); repErr != nil {
				m.job.FinishNeedsIntervention(fmt.Errorf("failed to release users.db before swap: %w (and failed to recover the user store: %v)", err, repErr))
				return
			}
			m.job.Finish(fmt.Errorf("failed to release users.db before swap: %w", err))
			return
		}
	}

	sw := newSwapper(m.cfg.DataDir, stagingDir)
	if err := sw.SwapAll(); err != nil {
		m.rollbackOrIntervene(sw, fmt.Errorf("failed to swap restored files: %w", err))
		return
	}

	// AuthService is nil when the server runs with --disable-auth: there's no
	// user/session state to reopen or invalidate in that mode.
	if m.cfg.AuthService != nil {
		m.job.SetPhase(PhaseReopeningAuth)
		if err := m.cfg.AuthService.ReplaceUserStore(m.cfg.DataDir); err != nil {
			m.rollbackOrIntervene(sw, fmt.Errorf("failed to reopen user database: %w", err))
			return
		}

		m.job.SetPhase(PhaseInvalidatingSessions)
		if err := m.cfg.AuthService.InvalidateAllSessions(); err != nil {
			// sessions.db isn't part of the restored content, so a failure here
			// doesn't leave restored data inconsistent — log and continue rather
			// than rolling back an otherwise-successful restore over it.
			slog.Default().Warn("restore: failed to invalidate sessions", "error", err)
		}
	}

	m.job.SetPhase(PhaseReloadingBranding)
	if err := m.cfg.BrandingService.Reload(); err != nil {
		m.rollbackOrIntervene(sw, fmt.Errorf("failed to reload branding: %w", err))
		return
	}

	sw.CommitAll()
	m.cfg.WriteGate.Disengage()
	if m.cfg.TriggerResync != nil {
		m.cfg.TriggerResync()
	}
	m.job.Finish(nil)
}

// rollbackOrIntervene is the shared failure path for every phase after
// SwapAll starts: it attempts to roll every swapped item back to its
// pre-restore state. If that succeeds, the gate is disengaged and the job
// reports a normal (retryable) failure — live data is exactly as it was
// before the restore was triggered. If rollback itself fails, the instance
// may be left in a partially-restored state, so the gate stays engaged (fail
// closed: no mutating request should land in inconsistent state) and the job
// is marked NeedsIntervention — self-restart (a fresh cold boot reading
// whatever is actually on disk) is the supported way out.
func (m *Manager) rollbackOrIntervene(sw *swapper, cause error) {
	if rbErr := sw.RollbackAll(); rbErr != nil {
		slog.Default().Error("restore: rollback failed after a failed restore phase, instance needs manual intervention",
			"cause", cause, "rollback_error", rbErr)
		m.job.FinishNeedsIntervention(fmt.Errorf("%w (rollback also failed: %v)", cause, rbErr))
		return
	}

	// By the time this runs, AuthService's user store is always at least
	// suspended (PauseUserStoreForSwap, before SwapAll) and possibly already
	// pointed at the swapped-in file (ReplaceUserStore, if a later phase like
	// branding-reload is what failed) — POSIX keeps an already-open fd valid
	// against its now-unlinked inode, so in the latter case it would keep
	// silently serving the rolled-back content instead of the original.
	// Re-point it at whatever is actually on disk now (the just-restored
	// original) so it can never drift from disk reality. Safe to call
	// regardless of which of those two states auth was left in (falls back to
	// the file that's already there — the original).
	if m.cfg.AuthService != nil {
		if err := m.cfg.AuthService.ReplaceUserStore(m.cfg.DataDir); err != nil {
			slog.Default().Error("restore: rollback succeeded but re-syncing AuthService against the restored files failed, instance needs manual intervention",
				"cause", cause, "resync_error", err)
			m.job.FinishNeedsIntervention(fmt.Errorf("%w (rollback succeeded but AuthService re-sync failed: %v)", cause, err))
			return
		}
	}

	m.cfg.WriteGate.Disengage()
	m.job.Finish(cause)
}
