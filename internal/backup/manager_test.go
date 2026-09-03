package backup

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// managerFixtureNoBox is managerFixture with no encryption key, mirroring a
// --disable-auth instance where credentials are stored in plaintext.
func managerFixtureNoBox(t *testing.T) (*Manager, string) {
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
	m, err := NewSettingsManager(NewConfigStore(dataDir, nil), rootDir, assetsDir)
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

	// A fresh manager over the same data dir boots from git-backup.json in the
	// background, so it comes up running shortly after construction.
	m2, err := NewSettingsManager(m.store, m.rootDir, m.assetsDir)
	if err != nil {
		t.Fatalf("NewSettingsManager (reboot): %v", err)
	}
	t.Cleanup(m2.Stop)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !m2.Enabled() {
		time.Sleep(10 * time.Millisecond)
	}
	if !m2.Enabled() {
		t.Fatalf("expected the rebooted manager to start from git-backup.json (bootErr=%v)", m2.BootError())
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

// TestManager_Reconfigure_DoesNotBlockGettersDuringGitIO pins that the
// fast-path getters (Status/Enabled/BootError) stay responsive while a
// Reconfigure is parked inside Init()'s network I/O — the editor-facing
// /backup/alert poll must not hang for minutes during a reconfigure.
func TestManager_Reconfigure_DoesNotBlockGettersDuringGitIO(t *testing.T) {
	m, _ := managerFixture(t)
	t.Cleanup(m.Stop)

	// An HTTP "remote" whose smart-protocol handshake hangs until released, so
	// Reconfigure stalls inside Init().
	hit := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseNow := func() { releaseOnce.Do(func() { close(release) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case hit <- struct{}{}:
		default:
		}
		<-release
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(releaseNow)

	errc := make(chan error, 1)
	go func() {
		errc <- m.Reconfigure(Config{
			RemoteURL: srv.URL + "/repo.git",
			Branch:    "main",
			SSHKey:    testSSHKeyPEM,
			Interval:  2 * time.Minute,
		})
	}()

	select {
	case <-hit:
	case <-time.After(5 * time.Second):
		t.Fatal("Reconfigure never reached remote I/O")
	}

	done := make(chan struct{})
	go func() {
		m.Enabled()
		m.Status()
		m.BootError()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Enabled/Status/BootError blocked while Reconfigure held the lock across git I/O")
	}

	releaseNow()
	<-errc // Reconfigure returns (remote 404s) — the error itself is not asserted here.
}

// TestManager_NewSettings_DoesNotBlockStartupOnSlowRemote pins that a
// settings-managed backup whose remote is slow/unreachable does not stall
// server startup: NewSettingsManager returns promptly (idle) and the boot is
// retried in the background, surfacing a BootError if it fails.
func TestManager_NewSettings_DoesNotBlockStartupOnSlowRemote(t *testing.T) {
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

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseNow := func() { releaseOnce.Do(func() { close(release) }) }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(releaseNow)

	store := NewConfigStore(dataDir, testBox(t))
	if err := store.Save(Config{
		Enabled: true, RemoteURL: srv.URL + "/r.git", Branch: "main",
		SSHKey: testSSHKeyPEM, Interval: 2 * time.Minute,
	}); err != nil {
		t.Fatalf("store.Save: %v", err)
	}

	mch := make(chan *Manager, 1)
	go func() {
		mm, _ := NewSettingsManager(store, rootDir, assetsDir)
		mch <- mm
	}()

	var m *Manager
	select {
	case m = <-mch:
	case <-time.After(2 * time.Second):
		releaseNow()
		t.Fatal("NewSettingsManager blocked on a slow remote")
	}
	t.Cleanup(m.Stop)

	if m.Enabled() {
		t.Fatal("expected idle until the background boot finishes")
	}

	releaseNow() // let the handshake complete; the 404 fails the boot
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && m.BootError() == nil {
		time.Sleep(20 * time.Millisecond)
	}
	if m.BootError() == nil {
		t.Fatal("expected BootError after the background boot failed against the 404 remote")
	}
}

func TestManager_Reconfigure_NoEncryptionKey_PersistsPlaintextSecret(t *testing.T) {
	m, bare := managerFixtureNoBox(t)
	t.Cleanup(m.Stop)

	if m.CredentialsEncrypted() {
		t.Fatal("expected CredentialsEncrypted()=false without a box")
	}
	if err := m.Reconfigure(fileRemoteConfig(bare)); err != nil {
		t.Fatalf("Reconfigure without an encryption key should still work: %v", err)
	}
	if !m.Enabled() {
		t.Fatal("expected Enabled after Reconfigure")
	}

	cfg, enabled, err := m.store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if !enabled || cfg.SSHKey != testSSHKeyPEM {
		t.Fatalf("SSH key did not round trip through plaintext storage: %+v", cfg)
	}
	raw, err := os.ReadFile(m.store.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), "OPENSSH PRIVATE KEY") {
		t.Fatalf("expected the SSH key in plaintext on disk under --disable-auth, got:\n%s", raw)
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
