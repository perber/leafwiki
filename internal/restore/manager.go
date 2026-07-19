package restore

import (
	"fmt"
	"log/slog"
	"os"
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

// gateDrainTimeout bounds how long the restore sequence waits, once the
// write gate is engaged, for requests already in flight (started just before
// Engage()) to finish before files are swapped out from under them. A
// timeout here is logged and the restore proceeds anyway rather than failing
// the whole operation over a slow request.
const gateDrainTimeout = 10 * time.Second

type Manager struct {
	cfg Config
	job *Job
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg, job: NewJob()}
}

// TriggerRestore starts a restore job asynchronously for the given snapshot
// id. Returns ErrRestoreAlreadyRunning if a restore is already in progress.
func (m *Manager) TriggerRestore(id string) error {
	if !m.job.Start() {
		return ErrRestoreAlreadyRunning
	}
	go m.runLocked(id)
	return nil
}

// Status returns the current restore job state (thread-safe).
func (m *Manager) Status() JobStatus {
	return m.job.Status()
}

// SelfRestart re-execs the current process. Callers (the HTTP handler) are
// expected to only allow this once Status().NeedsIntervention is true.
func (m *Manager) SelfRestart() error {
	return SelfRestart()
}

// runLocked performs the full validate -> gate -> swap -> reopen-auth ->
// invalidate-sessions -> reload-branding -> commit -> resync sequence. A
// panic anywhere in this chain is recovered here (not just logged), and
// treated as NeedsIntervention: a panic mid-sequence means we don't know
// which phases actually completed, so failing safe (gate stays engaged,
// admin must self-restart) is the only sound response.
func (m *Manager) runLocked(id string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Default().Error("panic during restore", "panic", r)
			m.job.FinishNeedsIntervention(fmt.Errorf("panic during restore: %v", r))
		}
	}()

	m.job.SetPhase(PhaseValidating)
	zipPath, err := m.cfg.SnapshotManager.SnapshotZipPath(id)
	if err != nil {
		m.job.Finish(err)
		return
	}

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

	// If the failure happened after AuthService.ReplaceUserStore already
	// succeeded (e.g. a later branding-reload failure), AuthService's
	// in-memory handle is still open against the users.db RollbackAll just
	// renamed away (POSIX keeps an already-open fd valid against its
	// now-unlinked inode) — it would keep silently serving the rolled-back
	// content instead of the original. Re-point it at whatever is actually
	// on disk now (the just-restored original) so it can never drift from
	// disk reality. Safe to call even when auth was never reopened this run
	// (falls back to the file that's already there — the original).
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
