package settings

import (
	"errors"
	"sync"
	"testing"
)

func TestFixed_Get_ReturnsPinnedValue(t *testing.T) {
	f := NewFixed(testConfig{Name: "pinned"})
	if got := f.Get(); got.Name != "pinned" {
		t.Fatalf("Get() = %+v, want Name %q", got, "pinned")
	}
}

func TestFixed_Put_AlwaysFailsWithErrEnvManaged(t *testing.T) {
	f := NewFixed(testConfig{Name: "pinned"})
	if err := f.Put(testConfig{Name: "changed"}); !errors.Is(err, ErrEnvManaged) {
		t.Fatalf("Put() err = %v, want ErrEnvManaged", err)
	}
	if got := f.Get(); got.Name != "pinned" {
		t.Fatalf("Get() after rejected Put = %+v, want unchanged Name %q", got, "pinned")
	}
}

func TestFixed_Reload_IsANoOp(t *testing.T) {
	f := NewFixed(testConfig{Name: "pinned"})
	if err := f.Reload(); err != nil {
		t.Fatalf("Reload() err = %v, want nil", err)
	}
	if got := f.Get(); got.Name != "pinned" {
		t.Fatalf("Get() after Reload = %+v, want unchanged Name %q", got, "pinned")
	}
}

func TestFixed_EnvManaged_IsTrue(t *testing.T) {
	if !NewFixed(testConfig{}).EnvManaged() {
		t.Fatal("Fixed.EnvManaged() = false, want true")
	}
}

func TestManaged_DelegatesToStoreAndReportsNotEnvManaged(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir, "config.json", 0o600, testConfig{Name: "default"}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}
	m := Managed[testConfig]{Store: st}

	if m.EnvManaged() {
		t.Fatal("Managed.EnvManaged() = true, want false")
	}
	if got := m.Get(); got.Name != "default" {
		t.Fatalf("Get() = %+v, want Name %q", got, "default")
	}
	if err := m.Put(testConfig{Name: "saved"}); err != nil {
		t.Fatalf("Put err: %v", err)
	}
	if got := m.Get(); got.Name != "saved" {
		t.Fatalf("Get() after Put = %+v, want Name %q", got, "saved")
	}

	// The write really landed on disk and Reload re-reads it.
	reopened, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("reopen New err: %v", err)
	}
	if got := reopened.Get(); got.Name != "saved" {
		t.Fatalf("reopened Get() = %+v, want Name %q", got, "saved")
	}
}

func TestValue_BothImplementationsSatisfyInterface_AndReloadInSlice(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}

	values := []Value[testConfig]{
		Managed[testConfig]{Store: st},
		NewFixed(testConfig{Name: "pinned"}),
	}
	// internal/restore iterates []Reloadable; a Fixed entry must be a safe
	// no-op there.
	for i, v := range values {
		if err := v.Reload(); err != nil {
			t.Fatalf("values[%d].Reload() err = %v", i, err)
		}
	}
}

func TestManaged_ConcurrentGetAndPut_NoRace(t *testing.T) {
	dir := t.TempDir()
	st, err := New(dir, "config.json", 0o600, testConfig{}, nil)
	if err != nil {
		t.Fatalf("New err: %v", err)
	}
	m := Managed[testConfig]{Store: st}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = m.Get() }()
		go func() { defer wg.Done(); _ = m.Put(testConfig{Name: "concurrent"}) }()
	}
	wg.Wait()
}
