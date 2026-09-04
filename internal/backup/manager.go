package backup

import (
	"errors"
	"log/slog"
	"sync"
)

// ErrEnvManaged is returned by Reconfigure/Disable when the backup is driven by
// CLI flags / environment variables. In that mode the settings UI is
// status-only and must not write git-backup.json.
var ErrEnvManaged = errors.New("git backup is configured via environment variables and cannot be changed from the settings UI")

// ErrNotRunning is returned by the manual operations (push, force-push, pull)
// when no backup is currently active.
var ErrNotRunning = errors.New("git backup is not enabled")

// Manager owns the git backup Repository + Scheduler and, in settings mode,
// lets an admin reconfigure them at runtime without restarting the server.
//
// Three shapes:
//   - env-managed: built from flags/env in cmd/leafwiki. cfg is fixed;
//     Reconfigure/Disable return ErrEnvManaged. Historical behaviour, unchanged.
//   - settings-managed, active: booted from git-backup.json or configured via
//     the UI. repo and sched are non-nil.
//   - settings-managed, idle: no git-backup.json, Enabled=false, or a boot
//     failure. repo/sched are nil; Reconfigure can bring it up.
//
// Locking: reconfigMu serialises the slow reconfigure/disable path (store write
// + scheduler teardown + git Init, any of which can take minutes) against
// itself. mu guards only the mutable pointers below and is held just long
// enough to read or swap them — never across git or network I/O — so the
// fast-path getters (Status/Enabled/BootError, polled by the editor-facing
// /backup/alert endpoint) stay responsive during a reconfigure.
type Manager struct {
	reconfigMu sync.Mutex

	mu      sync.Mutex
	repo    *Repository // nil when idle
	sched   *Scheduler  // nil when idle
	cfg     Config      // current effective config; zero when idle
	bootErr error       // why an enabled config failed to start (surfaced by Status)
	closed  bool        // Stop() has run; a still-in-flight background boot must not activate

	// Immutable after construction:
	envManaged bool
	store      *ConfigStore // nil when envManaged
	rootDir    string
	assetsDir  string
}

// NewEnvManager wraps an already-built repo + scheduler produced by the
// CLI/env path in cmd/leafwiki.
func NewEnvManager(repo *Repository, sched *Scheduler) *Manager {
	return &Manager{
		envManaged: true,
		repo:       repo,
		sched:      sched,
		cfg:        repo.cfg,
	}
}

// NewSettingsManager builds a manager backed by git-backup.json. It returns
// promptly: a missing/disabled file leaves it idle, an unreadable file records a
// BootError, and an enabled file is booted in the *background* (git Init can do
// minutes of network I/O and must not stall server startup). The manager is
// idle until that background boot succeeds; a failure is retained for Status. It
// only returns an error for genuinely unexpected problems — never for "not
// configured yet".
func NewSettingsManager(store *ConfigStore, rootDir, assetsDir string) (*Manager, error) {
	m := &Manager{store: store, rootDir: rootDir, assetsDir: assetsDir}

	_, enabled, err := store.Load()
	if err != nil {
		m.bootErr = err
		slog.Warn("backup: git-backup.json is unreadable; backup stays disabled until it is re-saved from settings",
			"error", err, "path", store.Path())
		return m, nil
	}
	if enabled {
		go m.bootInBackground()
	}
	return m, nil
}

// bootInBackground brings an enabled git-backup.json up after NewSettingsManager
// has already returned. It re-reads the store under reconfigMu so it loses
// cleanly to a concurrent Reconfigure/Disable, and it does not activate anything
// if the manager was shut down while it was booting.
func (m *Manager) bootInBackground() {
	m.reconfigMu.Lock()
	defer m.reconfigMu.Unlock()

	if m.Enabled() {
		return // a Reconfigure won the race and already brought it up
	}
	cfg, enabled, err := m.store.Load()
	if err != nil {
		m.setBootErr(err)
		slog.Warn("backup: git-backup.json became unreadable during background boot", "error", err)
		return
	}
	if !enabled {
		return // Disable ran, or the file changed
	}
	repo, sched, eff, err := m.build(cfg.WithSettingsDefaults())
	if err != nil {
		m.setBootErr(err)
		slog.Warn("backup: configured backup failed to start; it will be retried when reconfigured from settings", "error", err)
		return
	}
	if m.activateUnlessClosed(repo, sched, eff) {
		slog.Info("backup: started from git-backup.json",
			"remote", redactRemote(eff.RemoteURL), "interval", eff.Interval)
	} else {
		sched.Stop() // Stop() ran while we were booting
	}
}

// build initialises a Repository + Scheduler for cfg. It performs git and
// possibly network I/O and MUST NOT be called with m.mu held.
func (m *Manager) build(cfg Config) (*Repository, *Scheduler, Config, error) {
	cfg.Enabled = true
	cfg.RootDir = m.rootDir
	cfg.AssetsDir = m.assetsDir

	repo, err := Init(cfg)
	if err != nil {
		return nil, nil, Config{}, err
	}
	return repo, NewScheduler(repo), cfg, nil
}

// detach clears the running pointers under m.mu and returns the old scheduler so
// the caller can Stop() it (which blocks on the running backup) without holding
// the lock. bootErr is left untouched — the caller decides what it becomes next.
func (m *Manager) detach() *Scheduler {
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.sched
	m.repo, m.sched, m.cfg = nil, nil, Config{}
	return old
}

// activate installs a freshly built repo/scheduler under m.mu and clears bootErr.
func (m *Manager) activate(repo *Repository, sched *Scheduler, cfg Config) {
	m.mu.Lock()
	m.repo, m.sched, m.cfg, m.bootErr = repo, sched, cfg, nil
	m.mu.Unlock()
}

// activateUnlessClosed is activate that no-ops (returning false) when Stop() has
// already run, so a slow background boot can't resurrect the scheduler after
// shutdown.
func (m *Manager) activateUnlessClosed(repo *Repository, sched *Scheduler, cfg Config) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return false
	}
	m.repo, m.sched, m.cfg, m.bootErr = repo, sched, cfg, nil
	return true
}

// snapshot returns the current running repo/scheduler (both nil when idle).
func (m *Manager) snapshot() (*Repository, *Scheduler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.repo, m.sched
}

// Reconfigure persists cfg to git-backup.json and (re)starts the backup with
// it. The caller is responsible for having validated cfg
// (Config.ValidateForSettings) and verified connectivity (TestRemote) first.
//
// cfg must be complete: secret fields left empty here are stored empty. The
// wiki layer fills "unchanged" secrets from CurrentConfig before calling this.
//
// On a start failure the new config is still persisted (Enabled=true) so a
// later fix + restart picks it up, the running backup is left stopped, and the
// error is returned.
func (m *Manager) Reconfigure(cfg Config) error {
	if m.envManaged {
		return ErrEnvManaged
	}
	m.reconfigMu.Lock()
	defer m.reconfigMu.Unlock()

	cfg = cfg.WithSettingsDefaults()
	cfg.Enabled = true
	if err := m.store.Save(cfg); err != nil {
		return err
	}

	if old := m.detach(); old != nil {
		old.Stop()
	}
	repo, sched, eff, err := m.build(cfg)
	if err != nil {
		m.setBootErr(err)
		return err
	}
	m.activate(repo, sched, eff)
	slog.Info("backup: reconfigured from settings",
		"remote", redactRemote(eff.RemoteURL), "interval", eff.Interval)
	return nil
}

// Disable stops the running backup and marks git-backup.json disabled while
// keeping the remote/credentials so it can be re-enabled without re-entering
// everything.
func (m *Manager) Disable() error {
	if m.envManaged {
		return ErrEnvManaged
	}
	m.reconfigMu.Lock()
	defer m.reconfigMu.Unlock()

	cfg, _, err := m.store.Load()
	if err != nil {
		// Corrupt file: nothing meaningful to keep — persist a bare disabled marker.
		cfg = Config{}
	}
	cfg.Enabled = false
	if err := m.store.Save(cfg); err != nil {
		return err
	}
	if old := m.detach(); old != nil {
		old.Stop()
	}
	m.setBootErr(nil)
	slog.Info("backup: disabled from settings")
	return nil
}

func (m *Manager) setBootErr(err error) {
	m.mu.Lock()
	m.bootErr = err
	m.mu.Unlock()
}

// CurrentConfig returns the effective configuration with secrets in the clear
// (the wiki layer redacts before responding). In settings mode it reflects
// git-backup.json even while the manager is idle, so the UI can pre-fill the
// form after a boot failure.
func (m *Manager) CurrentConfig() (Config, error) {
	if m.envManaged || m.store == nil {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.cfg, nil
	}
	// store is immutable and Save is atomic (rename), so an unlocked Load races
	// only with itself and always sees a whole file.
	cfg, _, err := m.store.Load()
	return cfg, err
}

// EnvManaged reports whether the backup is driven by flags/env.
func (m *Manager) EnvManaged() bool { return m.envManaged }

// Configured reports whether a git backup is set up at all: running,
// env-managed, still booting in the background, or a settings-managed config
// whose boot failed. Unlike Enabled() ("currently running") this stays true
// across a boot failure, so the UI can surface the problem instead of hiding
// the backup entirely.
func (m *Manager) Configured() bool {
	if m.envManaged || m.Enabled() || m.BootError() != nil {
		return true
	}
	if m.store == nil {
		return false
	}
	_, enabled, _ := m.store.Load()
	return enabled
}

// CredentialsEncrypted reports whether saved backup credentials are encrypted at
// rest. False under --disable-auth, where there is no JWT secret to derive a key
// from and secrets are stored in plaintext in the 0600 config file; the UI
// surfaces this so an admin knows what to expect.
func (m *Manager) CredentialsEncrypted() bool {
	return m.store != nil && m.store.CanEncrypt()
}

// Enabled reports whether a backup is currently running.
func (m *Manager) Enabled() bool {
	repo, sched := m.snapshot()
	return repo != nil && sched != nil
}

// BootError returns the reason an enabled config failed to start, or nil.
func (m *Manager) BootError() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.bootErr
}

// Status returns the running backup's status snapshot. The bool is false when
// no backup is active.
func (m *Manager) Status() (StatusSnapshot, bool) {
	repo, _ := m.snapshot()
	if repo == nil {
		return StatusSnapshot{}, false
	}
	return repo.Status(), true
}

// TriggerNow runs a backup immediately. Returns ErrNotRunning when idle.
func (m *Manager) TriggerNow() error {
	_, sched := m.snapshot()
	if sched == nil {
		return ErrNotRunning
	}
	sched.TriggerNow()
	return nil
}

// ForcePush force-pushes local history to the remote. Returns ErrNotRunning when idle.
func (m *Manager) ForcePush() error {
	repo, _ := m.snapshot()
	if repo == nil {
		return ErrNotRunning
	}
	return repo.ForcePush()
}

// Pull fast-forwards local content from the remote. Returns ErrNotRunning when idle.
func (m *Manager) Pull() error {
	repo, _ := m.snapshot()
	if repo == nil {
		return ErrNotRunning
	}
	return repo.Pull()
}

// Stop shuts the manager down (server shutdown). Safe to call on an idle
// manager. It marks the manager closed so an in-flight background boot won't
// start a scheduler after this returns.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.closed = true
	sched := m.sched
	m.mu.Unlock()
	if sched != nil {
		sched.Stop()
	}
}
