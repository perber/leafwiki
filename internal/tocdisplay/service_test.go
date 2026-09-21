package tocdisplay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func TestService_NoFile_DisabledByDefault(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc.AlwaysShow() {
		t.Fatal("expected AlwaysShow() false when toc-display.json is absent")
	}
}

func TestService_SetAlwaysShow_PersistsAcrossReconstruction(t *testing.T) {
	dir := t.TempDir()

	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}
	if !svc.AlwaysShow() {
		t.Fatal("expected AlwaysShow() true right after SetAlwaysShow(true)")
	}

	reopened, err := New(dir)
	if err != nil {
		t.Fatalf("New (reopen): %v", err)
	}
	if !reopened.AlwaysShow() {
		t.Fatal("expected persisted AlwaysShow() true after reconstruction")
	}

	if err := reopened.SetAlwaysShow(false); err != nil {
		t.Fatalf("SetAlwaysShow(false): %v", err)
	}
	again, err := New(dir)
	if err != nil {
		t.Fatalf("New (reopen 2): %v", err)
	}
	if again.AlwaysShow() {
		t.Fatal("expected persisted AlwaysShow() false after SetAlwaysShow(false)")
	}
}

func TestService_SetAlwaysShow_WritesModeSixHundredJSON(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := svc.SetAlwaysShow(true); err != nil {
		t.Fatalf("SetAlwaysShow(true): %v", err)
	}

	path := filepath.Join(dir, "toc-display.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat toc-display.json: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("expected toc-display.json mode 0600, got %o", perm)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read toc-display.json: %v", err)
	}
	var cfg struct {
		AlwaysShow bool `json:"alwaysShow"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal toc-display.json: %v", err)
	}
	if !cfg.AlwaysShow {
		t.Fatalf("expected {\"alwaysShow\": true} on disk, got %s", data)
	}
}

func TestService_Reload_PicksUpExternalFileChange(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc.AlwaysShow() {
		t.Fatal("precondition: expected disabled")
	}

	// Simulate a restore dropping in a different toc-display.json.
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte(`{"alwaysShow": true}`), 0o600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	if svc.AlwaysShow() {
		t.Fatal("expected cache to still read false before Reload()")
	}

	if err := svc.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if !svc.AlwaysShow() {
		t.Fatal("expected AlwaysShow() true after Reload() picked up the new file")
	}
}

func TestService_Load_CorruptFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toc-display.json"), []byte("not json"), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	if _, err := New(dir); err == nil {
		t.Fatal("expected New to fail on an unparseable toc-display.json")
	}
}

func TestService_ConcurrentAccess_IsRaceFree(t *testing.T) {
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(v int) { defer wg.Done(); _ = svc.SetAlwaysShow(v%2 == 0) }(i)
		go func() { defer wg.Done(); _ = svc.AlwaysShow() }()
	}
	wg.Wait()
}
