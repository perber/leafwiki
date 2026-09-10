package branding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrandingStore_New_WhenConfigMissing_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("NewBrandingStore() error: %v", err)
	}

	cfg := store.Load()
	def := DefaultBrandingConfig()

	if cfg.SiteName != def.SiteName {
		t.Fatalf("expected SiteName %q, got %q", def.SiteName, cfg.SiteName)
	}
	if cfg.LogoFile != def.LogoFile {
		t.Fatalf("expected LogoFile %q, got %q", def.LogoFile, cfg.LogoFile)
	}
	if cfg.FaviconFile != def.FaviconFile {
		t.Fatalf("expected FaviconFile %q, got %q", def.FaviconFile, cfg.FaviconFile)
	}

	// Constraints should be present (runtime-only)
	if cfg.BrandingConstraints.MaxLogoSize != def.BrandingConstraints.MaxLogoSize {
		t.Fatalf("expected MaxLogoSize %d, got %d", def.BrandingConstraints.MaxLogoSize, cfg.BrandingConstraints.MaxLogoSize)
	}
	if cfg.BrandingConstraints.MaxFaviconSize != def.BrandingConstraints.MaxFaviconSize {
		t.Fatalf("expected MaxFaviconSize %d, got %d", def.BrandingConstraints.MaxFaviconSize, cfg.BrandingConstraints.MaxFaviconSize)
	}
	if len(cfg.BrandingConstraints.LogoExts) == 0 || len(cfg.BrandingConstraints.FaviconExts) == 0 {
		t.Fatalf("expected non-empty constraints maps, got logo=%d favicon=%d", len(cfg.BrandingConstraints.LogoExts), len(cfg.BrandingConstraints.FaviconExts))
	}
}

func TestBrandingStore_SaveThenLoad_RoundTrip_PersistsFields(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("NewBrandingStore() error: %v", err)
	}

	cfg := DefaultBrandingConfig()
	cfg.SiteName = "MyWiki"
	cfg.LogoFile = "logo.png"
	cfg.FaviconFile = "favicon.ico"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got := store.Load()
	if got.SiteName != "MyWiki" {
		t.Fatalf("expected SiteName %q, got %q", "MyWiki", got.SiteName)
	}
	if got.LogoFile != "logo.png" {
		t.Fatalf("expected LogoFile %q, got %q", "logo.png", got.LogoFile)
	}
	if got.FaviconFile != "favicon.ico" {
		t.Fatalf("expected FaviconFile %q, got %q", "favicon.ico", got.FaviconFile)
	}

	// Runtime-only constraints should still be present after Save.
	def := DefaultBrandingConfig()
	if got.BrandingConstraints.MaxLogoSize != def.BrandingConstraints.MaxLogoSize {
		t.Fatalf("expected injected MaxLogoSize %d, got %d", def.BrandingConstraints.MaxLogoSize, got.BrandingConstraints.MaxLogoSize)
	}
	if got.BrandingConstraints.MaxFaviconSize != def.BrandingConstraints.MaxFaviconSize {
		t.Fatalf("expected injected MaxFaviconSize %d, got %d", def.BrandingConstraints.MaxFaviconSize, got.BrandingConstraints.MaxFaviconSize)
	}

	// And it survives reopening the store from disk too.
	reopened, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("second NewBrandingStore() error: %v", err)
	}
	if got := reopened.Load(); got.SiteName != "MyWiki" {
		t.Fatalf("expected persisted SiteName %q after reopen, got %q", "MyWiki", got.SiteName)
	}
}

func TestBrandingStore_Save_WritesFileToExpectedLocation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("NewBrandingStore() error: %v", err)
	}

	cfg := DefaultBrandingConfig()
	cfg.SiteName = "CheckFile"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	p := filepath.Join(dir, "branding.json")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("expected branding.json to exist: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected branding.json to be a file, got directory")
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(b), `"siteName": "CheckFile"`) {
		t.Fatalf("expected branding.json to contain siteName, got:\n%s", string(b))
	}
}

func TestBrandingStore_New_WhenInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "branding.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("setup write invalid json: %v", err)
	}

	_, err := NewBrandingStore(dir)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to load branding config") {
		t.Fatalf("expected load error wrapper, got: %v", err)
	}
}

func TestBrandingStore_New_InsertsConstraintsEvenIfZeroInFile(t *testing.T) {
	dir := t.TempDir()

	// Only persisted fields are on disk. BrandingConstraints is json:"-" and
	// should be injected.
	raw := `{
  "siteName": "X",
  "logoFile": "logo.webp",
  "faviconFile": "favicon.png"
}`
	if err := os.WriteFile(filepath.Join(dir, "branding.json"), []byte(raw), 0644); err != nil {
		t.Fatalf("setup write json: %v", err)
	}

	store, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("NewBrandingStore() error: %v", err)
	}
	got := store.Load()

	def := DefaultBrandingConfig()
	if got.BrandingConstraints.MaxLogoSize != def.BrandingConstraints.MaxLogoSize {
		t.Fatalf("expected injected MaxLogoSize %d, got %d", def.BrandingConstraints.MaxLogoSize, got.BrandingConstraints.MaxLogoSize)
	}
	if got.BrandingConstraints.LogoExts[".png"] != def.BrandingConstraints.LogoExts[".png"] {
		t.Fatalf("expected injected LogoExts to match default")
	}
}

func TestBrandingStore_Reload_PicksUpExternalFileChange(t *testing.T) {
	dir := t.TempDir()
	store, err := NewBrandingStore(dir)
	if err != nil {
		t.Fatalf("NewBrandingStore() error: %v", err)
	}

	// Simulate a restore swapping in a different branding.json out from
	// under this store, without going through its own Save.
	raw := `{"siteName":"Restored Site","logoFile":"","faviconFile":""}`
	if err := os.WriteFile(filepath.Join(dir, "branding.json"), []byte(raw), 0644); err != nil {
		t.Fatalf("write branding.json: %v", err)
	}

	if got := store.Load(); got.SiteName == "Restored Site" {
		t.Fatal("test setup: SiteName should not already reflect the externally written file before Reload")
	}

	if err := store.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if got := store.Load(); got.SiteName != "Restored Site" {
		t.Fatalf("expected Reload() to pick up %q, got %q", "Restored Site", got.SiteName)
	}
}
