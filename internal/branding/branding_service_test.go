package branding

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: create a service with temp storage dir
func newTestBrandingService(t *testing.T) (*BrandingService, string) {
	t.Helper()
	dir := t.TempDir()

	svc, err := NewBrandingService(dir)
	if err != nil {
		t.Fatalf("NewBrandingService() error: %v", err)
	}
	return svc, dir
}

func TestBrandingService_DeleteLogo_NoLogo_NoOp(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	// Ensure config persisted with empty logo
	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("expected initial LogoFile empty, got %q", cfg.LogoFile)
	}

	if err := svc.DeleteLogo(); err != nil {
		t.Fatalf("DeleteLogo() error: %v", err)
	}

	// Still empty after delete
	cfg2, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg2.LogoFile != "" {
		t.Fatalf("expected LogoFile empty after delete, got %q", cfg2.LogoFile)
	}
}

func TestBrandingService_DeleteFavicon_NoFavicon_NoOp(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.FaviconFile != "" {
		t.Fatalf("expected initial FaviconFile empty, got %q", cfg.FaviconFile)
	}

	if err := svc.DeleteFavicon(); err != nil {
		t.Fatalf("DeleteFavicon() error: %v", err)
	}

	cfg2, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg2.FaviconFile != "" {
		t.Fatalf("expected FaviconFile empty after delete, got %q", cfg2.FaviconFile)
	}
}

func TestBrandingService_DeleteLogo_RemovesFileAndClearsConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)
	assetsDir := filepath.Join(dir, "branding")

	// Seed logo file and config
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.png"), []byte("logo"), 0644); err != nil {
		t.Fatalf("seed logo file: %v", err)
	}
	if err := svc.UpdateBranding("X"); err != nil { // just to ensure Save works; not required
		t.Fatalf("UpdateBranding() error: %v", err)
	}
	// Set config to reference the seeded file
	svc.mu.Lock()
	svc.brandingConfig.LogoFile = "logo.png"
	if err := svc.store.Save(svc.brandingConfig); err != nil {
		svc.mu.Unlock()
		t.Fatalf("store.Save() error: %v", err)
	}
	svc.mu.Unlock()

	if err := svc.DeleteLogo(); err != nil {
		t.Fatalf("DeleteLogo() error: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.png")); err == nil {
		t.Fatalf("expected logo file to be removed")
	}

	// Config should be cleared on disk
	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("expected LogoFile cleared, got %q", cfg.LogoFile)
	}
}

func TestBrandingService_DeleteFavicon_RemovesFileAndClearsConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)
	assetsDir := filepath.Join(dir, "branding")

	// Seed favicon file and config
	if err := os.WriteFile(filepath.Join(assetsDir, "favicon.ico"), []byte("fav"), 0644); err != nil {
		t.Fatalf("seed favicon file: %v", err)
	}

	// Set config to reference the seeded file
	svc.mu.Lock()
	svc.brandingConfig.FaviconFile = "favicon.ico"
	if err := svc.store.Save(svc.brandingConfig); err != nil {
		svc.mu.Unlock()
		t.Fatalf("store.Save() error: %v", err)
	}
	svc.mu.Unlock()

	if err := svc.DeleteFavicon(); err != nil {
		t.Fatalf("DeleteFavicon() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.ico")); err == nil {
		t.Fatalf("expected favicon file to be removed")
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.FaviconFile != "" {
		t.Fatalf("expected FaviconFile cleared, got %q", cfg.FaviconFile)
	}
}

func TestBrandingService_DeleteLogo_FileMissingStillClearsConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	// Reference a file that doesn't exist
	svc.mu.Lock()
	svc.brandingConfig.LogoFile = "logo.png"
	if err := svc.store.Save(svc.brandingConfig); err != nil {
		svc.mu.Unlock()
		t.Fatalf("store.Save() error: %v", err)
	}
	svc.mu.Unlock()

	if err := svc.DeleteLogo(); err != nil {
		t.Fatalf("DeleteLogo() error: %v", err)
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("expected LogoFile cleared even if file missing, got %q", cfg.LogoFile)
	}
}

func TestBrandingService_DeleteFavicon_FileMissingStillClearsConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	svc.mu.Lock()
	svc.brandingConfig.FaviconFile = "favicon.ico"
	if err := svc.store.Save(svc.brandingConfig); err != nil {
		svc.mu.Unlock()
		t.Fatalf("store.Save() error: %v", err)
	}
	svc.mu.Unlock()

	if err := svc.DeleteFavicon(); err != nil {
		t.Fatalf("DeleteFavicon() error: %v", err)
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.FaviconFile != "" {
		t.Fatalf("expected FaviconFile cleared even if file missing, got %q", cfg.FaviconFile)
	}
}

func TestBrandingService_UploadThenDeleteLogo_EndToEnd(t *testing.T) {
	svc, dir := newTestBrandingService(t)
	assetsDir := filepath.Join(dir, "branding")

	// Upload logo.png
	tmp, err := os.CreateTemp(t.TempDir(), "logo-*.png")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(bytes.Repeat([]byte("a"), 64)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	got, err := svc.UploadLogo(tmp, "mylogo.png")
	if err != nil {
		t.Fatalf("UploadLogo() error: %v", err)
	}
	if got != "logo.png" {
		t.Fatalf("expected returned %q, got %q", "logo.png", got)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.png")); err != nil {
		t.Fatalf("expected logo.png to exist: %v", err)
	}

	// Delete
	if err := svc.DeleteLogo(); err != nil {
		t.Fatalf("DeleteLogo() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.png")); err == nil {
		t.Fatalf("expected logo.png to be removed after delete")
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("expected LogoFile cleared after delete, got %q", cfg.LogoFile)
	}
}

func TestBrandingService_UploadThenDeleteFavicon_EndToEnd(t *testing.T) {
	svc, dir := newTestBrandingService(t)
	assetsDir := filepath.Join(dir, "branding")

	tmp, err := os.CreateTemp(t.TempDir(), "fav-*.ico")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(bytes.Repeat([]byte("b"), 64)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	got, err := svc.UploadFavicon(tmp, "favicon.ico")
	if err != nil {
		t.Fatalf("UploadFavicon() error: %v", err)
	}
	if got != "favicon.ico" {
		t.Fatalf("expected returned %q, got %q", "favicon.ico", got)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.ico")); err != nil {
		t.Fatalf("expected favicon.ico to exist: %v", err)
	}

	if err := svc.DeleteFavicon(); err != nil {
		t.Fatalf("DeleteFavicon() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.ico")); err == nil {
		t.Fatalf("expected favicon.ico to be removed after delete")
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.FaviconFile != "" {
		t.Fatalf("expected FaviconFile cleared after delete, got %q", cfg.FaviconFile)
	}
}

func TestBrandingService_GetBranding_ReturnsResponseWithConstraints(t *testing.T) {
	svc, _ := newTestBrandingService(t)

	resp, err := svc.GetBranding()
	if err != nil {
		t.Fatalf("GetBranding() error: %v", err)
	}

	if resp.SiteName == "" {
		t.Fatalf("expected non-empty SiteName")
	}

	if resp.BrandingConstraints.MaxLogoSize <= 0 || resp.BrandingConstraints.MaxFaviconSize <= 0 {
		t.Fatalf("expected positive max sizes, got logo=%d favicon=%d",
			resp.BrandingConstraints.MaxLogoSize, resp.BrandingConstraints.MaxFaviconSize)
	}
	if len(resp.BrandingConstraints.LogoExts) == 0 || len(resp.BrandingConstraints.FaviconExts) == 0 {
		t.Fatalf("expected non-empty constraints maps")
	}
}

func TestBrandingService_UpdateBranding_PersistsToDisk(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	if err := svc.UpdateBranding("My Wiki"); err != nil {
		t.Fatalf("UpdateBranding() error: %v", err)
	}

	// Verify persisted config by reading via store
	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.SiteName != "My Wiki" {
		t.Fatalf("expected SiteName %q, got %q", "My Wiki", cfg.SiteName)
	}
}

func TestBrandingService_UploadLogo_InvalidExtension_ReturnsError(t *testing.T) {
	svc, _ := newTestBrandingService(t)

	// bytes.Reader implements io.Reader, but UploadLogo expects multipart.File.
	// multipart.File is an interface satisfied by *os.File and multipart.SectionReadCloser.
	// We'll use an actual temp file.
	f, err := os.CreateTemp(t.TempDir(), "badlogo-*")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	defer f.Close()

	_, err = svc.UploadLogo(f, "logo.exe")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid logo file type") {
		t.Fatalf("expected invalid logo type error, got: %v", err)
	}
}

func TestBrandingService_UploadFavicon_InvalidExtension_ReturnsError(t *testing.T) {
	svc, _ := newTestBrandingService(t)

	f, err := os.CreateTemp(t.TempDir(), "badfav-*")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	defer f.Close()

	_, err = svc.UploadFavicon(f, "favicon.jpg") // jpg should be invalid for favicon in defaults
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid favicon file type") {
		t.Fatalf("expected invalid favicon type error, got: %v", err)
	}
}

func TestBrandingService_UploadLogo_WritesFileAndUpdatesConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	// Create a small "image" file (content doesn't matter; size and extension do)
	content := bytes.Repeat([]byte("a"), 128)
	tmp, err := os.CreateTemp(t.TempDir(), "logo-*.png")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(content); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	rel, err := svc.UploadLogo(tmp, "mylogo.png")
	if err != nil {
		t.Fatalf("UploadLogo() error: %v", err)
	}
	if rel != "logo.png" {
		t.Fatalf("expected returned path %q, got %q", "logo.png", rel)
	}

	// File should exist under branding assets dir
	assetsDir := filepath.Join(dir, "branding")
	target := filepath.Join(assetsDir, "logo.png")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected logo file to exist at %s: %v", target, err)
	}

	// Config should be updated and persisted
	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.LogoFile != "logo.png" {
		t.Fatalf("expected cfg.LogoFile %q, got %q", "logo.png", cfg.LogoFile)
	}
}

func TestBrandingService_UploadFavicon_WritesFileAndUpdatesConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	content := bytes.Repeat([]byte("b"), 128)
	tmp, err := os.CreateTemp(t.TempDir(), "favicon-*.ico")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(content); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	rel, err := svc.UploadFavicon(tmp, "favicon.ico")
	if err != nil {
		t.Fatalf("UploadFavicon() error: %v", err)
	}
	if rel != "favicon.ico" {
		t.Fatalf("expected returned path %q, got %q", "favicon.ico", rel)
	}

	assetsDir := filepath.Join(dir, "branding")
	target := filepath.Join(assetsDir, "favicon.ico")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected favicon file to exist at %s: %v", target, err)
	}

	store := NewBrandingStore(dir)
	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error: %v", err)
	}
	if cfg.FaviconFile != "favicon.ico" {
		t.Fatalf("expected cfg.FaviconFile %q, got %q", "favicon.ico", cfg.FaviconFile)
	}
}

func TestBrandingService_UploadLogo_RemovesOldLogoVariants(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	assetsDir := filepath.Join(dir, "branding")

	// Seed old variants
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.jpg"), []byte("old"), 0644); err != nil {
		t.Fatalf("seed old logo.jpg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "logo.webp"), []byte("old"), 0644); err != nil {
		t.Fatalf("seed old logo.webp: %v", err)
	}

	// Upload new logo.png
	tmp, err := os.CreateTemp(t.TempDir(), "logo-*.png")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write([]byte("new")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	if _, err := svc.UploadLogo(tmp, "logo.png"); err != nil {
		t.Fatalf("UploadLogo() error: %v", err)
	}

	// New should exist
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.png")); err != nil {
		t.Fatalf("expected logo.png to exist: %v", err)
	}
	// Old should be removed
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.jpg")); err == nil {
		t.Fatalf("expected logo.jpg to be removed")
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "logo.webp")); err == nil {
		t.Fatalf("expected logo.webp to be removed")
	}
}

func TestBrandingService_UploadFavicon_RemovesOldFaviconVariants(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	assetsDir := filepath.Join(dir, "branding")

	// Seed old variants
	if err := os.WriteFile(filepath.Join(assetsDir, "favicon.png"), []byte("old"), 0644); err != nil {
		t.Fatalf("seed old favicon.png: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "favicon.webp"), []byte("old"), 0644); err != nil {
		t.Fatalf("seed old favicon.webp: %v", err)
	}

	// Upload new favicon.ico
	tmp, err := os.CreateTemp(t.TempDir(), "favicon-*.ico")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write([]byte("new")); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	if _, err := svc.UploadFavicon(tmp, "favicon.ico"); err != nil {
		t.Fatalf("UploadFavicon() error: %v", err)
	}

	// New should exist
	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.ico")); err != nil {
		t.Fatalf("expected favicon.ico to exist: %v", err)
	}
	// Old should be removed
	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.png")); err == nil {
		t.Fatalf("expected favicon.png to be removed")
	}
	if _, err := os.Stat(filepath.Join(assetsDir, "favicon.webp")); err == nil {
		t.Fatalf("expected favicon.webp to be removed")
	}
}

func TestBrandingService_UploadLogo_TooLarge_ReturnsErrorAndDoesNotUpdateConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	// Lower max size to make test fast
	svc.brandingConfig.BrandingConstraints.MaxLogoSize = 10

	// Create file > 10 bytes
	tmp, err := os.CreateTemp(t.TempDir(), "logo-big-*.png")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(bytes.Repeat([]byte("x"), 50)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	_, err = svc.UploadLogo(tmp, "logo.png")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	// Should not have updated persisted config
	store := NewBrandingStore(dir)
	cfg, err2 := store.Load()
	if err2 != nil {
		t.Fatalf("store.Load() error: %v", err2)
	}
	if cfg.LogoFile != "" {
		t.Fatalf("expected LogoFile to remain empty, got %q", cfg.LogoFile)
	}
}

func TestBrandingService_UploadFavicon_TooLarge_ReturnsErrorAndDoesNotUpdateConfig(t *testing.T) {
	svc, dir := newTestBrandingService(t)

	// Lower max size to make test fast
	svc.brandingConfig.BrandingConstraints.MaxFaviconSize = 10

	tmp, err := os.CreateTemp(t.TempDir(), "fav-big-*.ico")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	if _, err := tmp.Write(bytes.Repeat([]byte("y"), 50)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	defer tmp.Close()

	_, err = svc.UploadFavicon(tmp, "favicon.ico")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	store := NewBrandingStore(dir)
	cfg, err2 := store.Load()
	if err2 != nil {
		t.Fatalf("store.Load() error: %v", err2)
	}
	if cfg.FaviconFile != "" {
		t.Fatalf("expected FaviconFile to remain empty, got %q", cfg.FaviconFile)
	}
}
