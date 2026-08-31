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
type Manager struct {
	mu sync.Mutex

	envManaged bool
	store      *ConfigStore // nil when envManaged
	rootDir    string
	assetsDir  string

	repo  *Repository // nil when idle
	sched *Scheduler  // nil when idle
	cfg   Config      // current effective config; zero when idle

	bootErr error // why an enabled config failed to start (surfaced by Status)
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

// NewSettingsManager builds a manager backed by git-backup.json. If that file
// exists and is enabled the backup starts immediately; a boot failure (bad
// credentials, unreachable remote, corrupt file) is retained for Status and
// the manager stays idle so the server still comes up. A missing or disabled
// file is simply idle. It only returns an error for genuinely unexpected
// problems — never for "not configured yet".
func NewSettingsManager(store *ConfigStore, rootDir, assetsDir string) (*Manager, error) {
	m := &Manager{store: store, rootDir: rootDir, assetsDir: assetsDir}

	cfg, enabled, err := store.Load()
	if err != nil {
		m.bootErr = err
		slog.Warn("backup: git-backup.json is unreadable; backup stays disabled until it is re-saved from settings",
			"error", err, "path", store.Path())
		return m, nil
	}
	if !enabled {
		return m, nil
	}
	if err := m.start(cfg.WithSettingsDefaults()); err != nil {
		m.bootErr = err
		slog.Warn("backup: configured backup failed to start; it will be retried when reconfigured from settings", "error", err)
		return m, nil
	}
	slog.Info("backup: started from git-backup.json",
		"remote", redactRemote(m.cfg.RemoteURL), "interval", m.cfg.Interval)
	return m, nil
}

// start builds and starts a Repository + Scheduler for cfg. Caller holds mu (or
// is the constructor, before the manager is shared).
func (m *Manager) start(cfg Config) error {
	cfg.Enabled = true
	cfg.RootDir = m.rootDir
	cfg.AssetsDir = m.assetsDir

	repo, err := Init(cfg)
	if err != nil {
		return err
	}
	m.repo = repo
	m.sched = NewScheduler(repo)
	m.cfg = cfg
	m.bootErr = nil
	return nil
}

// stop tears down the running Repository + Scheduler. Caller holds mu.
func (m *Manager) stop() {
	if m.sched != nil {
		m.sched.Stop()
	}
	m.sched = nil
	m.repo = nil
	m.cfg = Config{}
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.envManaged {
		return ErrEnvManaged
	}

	cfg = cfg.WithSettingsDefaults()
	cfg.Enabled = true
	if err := m.store.Save(cfg); err != nil {
		return err
	}

	m.stop()
	if err := m.start(cfg); err != nil {
		m.bootErr = err
		return err
	}
	slog.Info("backup: reconfigured from settings",
		"remote", redactRemote(m.cfg.RemoteURL), "interval", m.cfg.Interval)
	return nil
}

// Disable stops the running backup and marks git-backup.json disabled while
// keeping the remote/credentials so it can be re-enabled without re-entering
// everything.
func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.envManaged {
		return ErrEnvManaged
	}

	cfg, _, err := m.store.Load()
	if err != nil {
		// Corrupt file: nothing meaningful to keep — persist a bare disabled marker.
		cfg = Config{}
	}
	cfg.Enabled = false
	if err := m.store.Save(cfg); err != nil {
		return err
	}
	m.stop()
	m.bootErr = nil
	slog.Info("backup: disabled from settings")
	return nil
}

// CurrentConfig returns the effective configuration with secrets in the clear
// (the wiki layer redacts before responding). In settings mode it reflects
// git-backup.json even while the manager is idle, so the UI can pre-fill the
// form after a boot failure.
func (m *Manager) CurrentConfig() (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.envManaged || m.store == nil {
		return m.cfg, nil
	}
	cfg, _, err := m.store.Load()
	return cfg, err
}

// EnvManaged reports whether the backup is driven by flags/env.
func (m *Manager) EnvManaged() bool { return m.envManaged }

// Enabled reports whether a backup is currently running.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.repo != nil && m.sched != nil
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
	m.mu.Lock()
	repo := m.repo
	m.mu.Unlock()
	if repo == nil {
		return StatusSnapshot{}, false
	}
	return repo.Status(), true
}

// TriggerNow runs a backup immediately. Returns ErrNotRunning when idle.
func (m *Manager) TriggerNow() error {
	m.mu.Lock()
	sched := m.sched
	m.mu.Unlock()
	if sched == nil {
		return ErrNotRunning
	}
	sched.TriggerNow()
	return nil
}

// ForcePush force-pushes local history to the remote. Returns ErrNotRunning when idle.
func (m *Manager) ForcePush() error {
	m.mu.Lock()
	repo := m.repo
	m.mu.Unlock()
	if repo == nil {
		return ErrNotRunning
	}
	return repo.ForcePush()
}

// Pull fast-forwards local content from the remote. Returns ErrNotRunning when idle.
func (m *Manager) Pull() error {
	m.mu.Lock()
	repo := m.repo
	m.mu.Unlock()
	if repo == nil {
		return ErrNotRunning
	}
	return repo.Pull()
}

// Stop shuts the manager down (server shutdown). Safe to call on a nil-ish idle manager.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sched != nil {
		m.sched.Stop()
	}
}