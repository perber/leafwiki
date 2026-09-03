package publicaccess

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestService_SettingsManagedNoFile_DisabledByDefault(t *testing.T) {
	svc, err := NewSettingsManaged(t.TempDir())
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}
	if svc.Enabled() {
		t.Fatal("expected Enabled() false when public-access.json is absent")
	}
	if svc.EnvManaged() {
		t.Fatal("expected EnvManaged() false for a settings-managed service")
	}
}

func TestService_SettingsManagedSetEnabled_PersistsAcrossReconstruction(t *testing.T) {
	dir := t.TempDir()

	svc, err := NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}
	if err := svc.SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("expected Enabled() true right after SetEnabled(true)")
	}

	// A fresh service over the same dir must see the persisted value.
	reopened, err := NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged (reopen): %v", err)
	}
	if !reopened.Enabled() {
		t.Fatal("expected persisted Enabled() true after reconstruction")
	}

	if err := reopened.SetEnabled(false); err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	again, err := NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged (reopen 2): %v", err)
	}
	if again.Enabled() {
		t.Fatal("expected persisted Enabled() false after SetEnabled(false)")
	}
}

func TestService_SettingsManagedSetEnabled_WritesModeSixHundredJSON(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}
	if err := svc.SetEnabled(true); err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}

	path := filepath.Join(dir, "public-access.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat public-access.json: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected public-access.json mode 0600, got %o", perm)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read public-access.json: %v", err)
	}
	var cfg struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal public-access.json: %v", err)
	}
	if !cfg.Enabled {
		t.Fatalf("expected {\"enabled\": true} on disk, got %s", data)
	}
}

func TestService_SettingsManagedReload_PicksUpExternalFileChange(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewSettingsManaged(dir)
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}
	if svc.Enabled() {
		t.Fatal("precondition: expected disabled")
	}

	// Simulate a restore dropping in a different public-access.json.
	if err := os.WriteFile(filepath.Join(dir, "public-access.json"), []byte(`{"enabled": true}`), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	if svc.Enabled() {
		t.Fatal("expected cache to still read false before Reload()")
	}

	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !svc.Enabled() {
		t.Fatal("expected Enabled() true after Reload() picked up the new file")
	}
}

func TestService_SettingsManagedLoad_CorruptFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "public-access.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := NewSettingsManaged(dir); err == nil {
		t.Fatal("expected NewSettingsManaged to fail on an unparseable public-access.json")
	}
}

func TestService_EnvManaged_ReturnsFixedValueAndRejectsSetEnabled(t *testing.T) {
	svc := NewEnvManaged(true)

	if !svc.Enabled() {
		t.Fatal("expected Enabled() true for NewEnvManaged(true)")
	}
	if !svc.EnvManaged() {
		t.Fatal("expected EnvManaged() true")
	}

	err := svc.SetEnabled(false)
	if err == nil {
		t.Fatal("expected SetEnabled to fail on an env-managed service")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected a *LocalizedError, got %T", err)
	}
	if loc.Code != ErrCodeEnvManaged {
		t.Fatalf("expected error code %q, got %q", ErrCodeEnvManaged, loc.Code)
	}
	if !svc.Enabled() {
		t.Fatal("expected Enabled() still true after a rejected SetEnabled")
	}
}

func TestService_EnvManagedReload_IsANoOp(t *testing.T) {
	svc := NewEnvManaged(false)
	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload on env-managed should be a no-op, got %v", err)
	}
	if svc.Enabled() {
		t.Fatal("expected Enabled() unchanged (false) after Reload()")
	}
}

func TestService_EnvManagedSetEnabled_NeverWritesAFile(t *testing.T) {
	dir := t.TempDir()
	svc := NewEnvManaged(true)
	_ = svc.SetEnabled(false) // rejected

	if _, err := os.Stat(filepath.Join(dir, "public-access.json")); !os.IsNotExist(err) {
		t.Fatalf("env-managed service must not create public-access.json (stat err: %v)", err)
	}
}

func TestService_SettingsManagedConcurrentAccess_IsRaceFree(t *testing.T) {
	svc, err := NewSettingsManaged(t.TempDir())
	if err != nil {
		t.Fatalf("NewSettingsManaged: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(v int) { defer wg.Done(); _ = svc.SetEnabled(v%2 == 0) }(i)
		go func() { defer wg.Done(); _ = svc.Enabled() }()
	}
	wg.Wait()
}
