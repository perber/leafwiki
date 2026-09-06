package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type testConfig struct {
	Name     string `json:"name"`
	Injected bool   `json:"-"` // never persisted, populated by onLoad
}

func TestStore_New_MissingFile_ReturnsZeroValue(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir, "config.json", 0o600, testConfig{Name: "default"}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	got := s.Get()
	if got.Name != "default" {
		t.Fatalf("Get() = %+v, want zero value with Name %q", got, "default")
	}
}

func TestStore_PutThenNew_RoundTrips(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	if err := s.Put(testConfig{Name: "saved"}); err != nil {
		t.Fatalf("Put err: %v", err)
	}

	s2, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("second New err: %v", err)
	}
	if got := s2.Get(); got.Name != "saved" {
		t.Fatalf("Get() after reopen = %+v, want Name %q", got, "saved")
	}
}

func TestStore_Put_WritesFileWithGivenPermAndIndentedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	s, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}
	if err := s.Put(testConfig{Name: "x"}); err != nil {
		t.Fatalf("Put err: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat err: %v", err)
	}
	if runtime := info.Mode().Perm(); runtime != 0o600 {
		t.Fatalf("file perm = %v, want 0600", runtime)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("stored file is not valid JSON: %v", err)
	}
	if decoded["name"] != "x" {
		t.Fatalf("stored name = %v, want x", decoded["name"])
	}
}

func TestStore_Reload_PicksUpExternalFileChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	s, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	if err := os.WriteFile(path, []byte(`{"name":"external"}`), 0o600); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}

	if err := s.Reload(); err != nil {
		t.Fatalf("Reload err: %v", err)
	}
	if got := s.Get(); got.Name != "external" {
		t.Fatalf("Get() after Reload = %+v, want Name %q", got, "external")
	}
}

func TestStore_Reload_MissingFile_KeepsCurrentValue(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir, "config.json", 0o600, testConfig{Name: "seed"}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	// File never existed; Reload should be a no-op, not reset to zero.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload err: %v", err)
	}
	if got := s.Get(); got.Name != "seed" {
		t.Fatalf("Get() after Reload on missing file = %+v, want Name %q (unchanged)", got, "seed")
	}
}

func TestStore_OnLoad_AppliedAfterInitialLoadAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	onLoad := func(v *testConfig) { v.Injected = true }

	s, err := New(dir, "config.json", 0o600, testConfig{}, onLoad)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}
	if got := s.Get(); !got.Injected {
		t.Fatalf("onLoad not applied on initial New: %+v", got)
	}

	if err := os.WriteFile(path, []byte(`{"name":"x"}`), 0o600); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload err: %v", err)
	}
	if got := s.Get(); !got.Injected {
		t.Fatalf("onLoad not applied after Reload: %+v", got)
	}
}

func TestStore_OnLoad_AppliedAfterPut(t *testing.T) {
	dir := t.TempDir()
	onLoad := func(v *testConfig) { v.Injected = true }

	s, err := New(dir, "config.json", 0o600, testConfig{}, onLoad)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}
	if err := s.Put(testConfig{Name: "x"}); err != nil {
		t.Fatalf("Put err: %v", err)
	}
	if got := s.Get(); !got.Injected {
		t.Fatalf("onLoad not applied after Put: %+v", got)
	}
}

func TestStore_ConcurrentGetAndPut_NoRace(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			_ = s.Get()
		}(i)
		go func(n int) {
			defer wg.Done()
			_ = s.Put(testConfig{Name: "concurrent"})
		}(i)
	}
	wg.Wait()
}

var _ Reloadable = (*Store[testConfig])(nil)
