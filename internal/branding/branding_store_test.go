package branding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrandingStore_Load_WhenConfigMissing_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	store := NewBrandingStore(dir)

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

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
	store := NewBrandingStore(dir)

	// Prepare config to save.
	cfg := DefaultBrandingConfig()
	cfg.SiteName = "MyWiki"
	cfg.LogoFile = "logo.png"
	cfg.FaviconFile = "favicon.ico"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got.SiteName != "MyWiki" {
		t.Fatalf("expected SiteName %q, got %q", "MyWiki", got.SiteName)
	}
	if got.LogoFile != "logo.png" {
		t.Fatalf("expected LogoFile %q, got %q", "logo.png", got.LogoFile)
	}
	if got.FaviconFile != "favicon.ico" {
		t.Fatalf("expected FaviconFile %q, got %q", "favicon.ico", got.FaviconFile)
	}

	// Runtime-only constraints should be injected on Load, even though they are not persisted.
	def := DefaultBrandingConfig()
	if got.BrandingConstraints.MaxLogoSize != def.BrandingConstraints.MaxLogoSize {
		t.Fatalf("expected injected MaxLogoSize %d, got %d", def.BrandingConstraints.MaxLogoSize, got.BrandingConstraints.MaxLogoSize)
	}
	if got.BrandingConstraints.MaxFaviconSize != def.BrandingConstraints.MaxFaviconSize {
		t.Fatalf("expected injected MaxFaviconSize %d, got %d", def.BrandingConstraints.MaxFaviconSize, got.BrandingConstraints.MaxFaviconSize)
	}
}

func TestBrandingStore_Save_WritesFileToExpectedLocation(t *testing.T) {
	dir := t.TempDir()
	store := NewBrandingStore(dir)

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

	// Basic sanity: file contains our siteName
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(b), `"siteName": "CheckFile"`) {
		t.Fatalf("expected branding.json to contain siteName, got:\n%s", string(b))
	}
}

func TestBrandingStore_Load_WhenInvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewBrandingStore(dir)

	// Write broken JSON
	if err := os.WriteFile(filepath.Join(dir, "branding.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("setup write invalid json: %v", err)
	}

	_, err := store.Load()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse branding config") {
		t.Fatalf("expected parse error wrapper, got: %v", err)
	}
}

func TestBrandingStore_Load_InsertsConstraintsEvenIfZeroInFile(t *testing.T) {
	dir := t.TempDir()
	store := NewBrandingStore(dir)

	// Save JSON that includes only persisted fields. BrandingConstraints is json:"-" and should be injected.
	raw := `{
  "siteName": "X",
  "logoFile": "logo.webp",
  "faviconFile": "favicon.png"
}`
	if err := os.WriteFile(filepath.Join(dir, "branding.json"), []byte(raw), 0644); err != nil {
		t.Fatalf("setup write json: %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	def := DefaultBrandingConfig()

	// Ensure constraints are injected and usable
	if got.BrandingConstraints.MaxLogoSize != def.BrandingConstraints.MaxLogoSize {
		t.Fatalf("expected injected MaxLogoSize %d, got %d", def.BrandingConstraints.MaxLogoSize, got.BrandingConstraints.MaxLogoSize)
	}
	if got.BrandingConstraints.LogoExts[".png"] != def.BrandingConstraints.LogoExts[".png"] {
		t.Fatalf("expected injected LogoExts to match default")
	}
}
