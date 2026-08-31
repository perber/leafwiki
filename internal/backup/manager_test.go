package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// managerFixture returns a settings-mode manager over a fresh data dir plus the
// bare remote path tests can point it at.
func managerFixture(t *testing.T) (*Manager, string) {
	t.Helper()
	dataDir := t.TempDir()
	rootDir := filepath.Join(dataDir, "root")
	assetsDir := filepath.Join(dataDir, "assets")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0o644); err != nil {
		t.Fatalf("WriteFile page: %v", err)
	}

	store := NewConfigStore(dataDir, testBox(t))
	m, err := NewSettingsManager(store, rootDir, assetsDir)
	if err != nil {
		t.Fatalf("NewSettingsManager: %v", err)
	}
	return m, initBareRemote(t)
}

func fileRemoteConfig(bare string) Config {
	return Config{
		RemoteURL: "file://" + bare,
		Branch:    "main",
		SSHKey:    testSSHKeyPEM,
		Interval:  2 * time.Minute,
	}
}

func TestManager_NewSettings_NoConfigFile_IsIdle(t *testing.T) {
	m, _ := managerFixture(t)
	t.Cleanup(m.Stop)

	if m.EnvManaged() {
		t.Fatal("settings manager reports EnvManaged")
	}
	if m.Enabled() {
		t.Fatal("expected idle manager with no config file")
	}
	if _, running := m.Status(); running {
		t.Fatal("Status should report not running")
	}
	if err := m.TriggerNow(); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("TriggerNow while idle: got %v, want ErrNotRunning", err)
	}
}

func TestManager_Reconfigure_BringsBackupUpAndPersists(t *testing.T) {
	m, bare := managerFixture(t)
	t.Cleanup(m.Stop)

	if err := m.Reconfigure(fileRemoteConfig(bare)); err != nil {
		t.Fatalf("Reconfigure: %v", err)
	}
	if !m.Enabled() {
		t.Fatal("expected Enabled after Reconfigure")
	}

	// Persisted and enabled on disk.
	cfg, enabled, err := m.store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if !enabled || cfg.RemoteURL != "file://"+bare {
		t.Fatalf("config not persisted correctly: %+v enabled=%v", cfg, enabled)
	}
}

func TestManager_Reconfigure_SwapsSchedulerWithoutLeaking(t *testing.T) {
	m, bare1 := managerFixture(t)
	t.Cleanup(m.Stop)

	if err := m.Reconfigure(fileRemoteConfig(bare1)); err != nil {
		t.Fatalf("first Reconfigure: %v", err)
	}
	bare2 := initBareRemote(t)

	done := make(chan error, 1)
	go func() { done <- m.Reconfigure(fileRemoteConfig(bare2)) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Reconfigure: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Reconfigure hung — old scheduler likely not stopped")
	}

	if !m.Enabled() {
		t.Fatal("expected Enabled after second Reconfigure")
	}
	cfg, _, _ := m.store.Load()
	if cfg.RemoteURL != "file://"+bare2 {
		t.Fatalf("expected remote to be bare2, got %q", cfg.RemoteURL)
	}
}

func TestManager_Disable_StopsBackupButKeepsRemote(t *testing.T) {
	m, bare := managerFixture(t)
	t.Cleanup(m.Stop)

	if err := m.Reconfigure(fileRemoteConfig(bare)); err != nil {
		t.Fatalf("Reconfigure: %v", err)
	}
	if err := m.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if m.Enabled() {
		t.Fatal("expected idle after Disable")
	}

	cfg, enabled, err := m.store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if enabled {
		t.Fatal("expected enabled=false on disk after Disable")
	}
	if cfg.RemoteURL != "file://"+bare {
		t.Fatalf("Disable should keep the remote, got %q", cfg.RemoteURL)
	}
}

func TestManager_NewSettings_BootsFromEnabledConfigFile(t *testing.T) {
	m, bare := managerFixture(t)
	if err := m.Reconfigure(fileRemoteConfig(bare)); err != nil {
		t.Fatalf("Reconfigure: %v", err)
	}
	m.Stop()

	// A fresh manager over the same data dir should come up already running.
	m2, err := NewSettingsManager(m.store, m.rootDir, m.assetsDir)
	if err != nil {
		t.Fatalf("NewSettingsManager (reboot): %v", err)
	}
	t.Cleanup(m2.Stop)
	if !m2.Enabled() {
		t.Fatal("expected the rebooted manager to start from git-backup.json")
	}
}

func TestManager_NewSettings_CorruptConfig_StaysIdleWithBootError(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, ConfigFileName), []byte("{broken"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := NewSettingsManager(NewConfigStore(dataDir, testBox(t)), filepath.Join(dataDir, "root"), filepath.Join(dataDir, "assets"))
	if err != nil {
		t.Fatalf("NewSettingsManager should not hard-fail on a corrupt file: %v", err)
	}
	t.Cleanup(m.Stop)
	if m.Enabled() {
		t.Fatal("expected idle manager for a corrupt config")
	}
	if m.BootError() == nil {
		t.Fatal("expected BootError to be set for a corrupt config")
	}
}

func TestManager_EnvManaged_RejectsReconfigureAndDisable(t *testing.T) {
	bare := initBareRemote(t)
	repo, _ := newRepoWithRemote(t, bare)
	sched := NewScheduler(repo)
	m := NewEnvManager(repo, sched)
	t.Cleanup(m.Stop)

	if !m.EnvManaged() {
		t.Fatal("expected EnvManaged")
	}
	if err := m.Reconfigure(fileRemoteConfig(bare)); !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("Reconfigure: got %v, want ErrEnvManaged", err)
	}
	if err := m.Disable(); !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("Disable: got %v, want ErrEnvManaged", err)
	}
	if !m.Enabled() {
		t.Fatal("env-managed manager should report Enabled")
	}
}
