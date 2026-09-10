package branding

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/perber/wiki/internal/core/shared"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// BrandingService provides branding operations
const (
	errFailedToDeleteLogo    = "Failed to delete logo"
	errFailedToDeleteFavicon = "Failed to delete favicon"
)

type BrandingService struct {
	store *BrandingStore
	mu    sync.RWMutex
}

// NewBrandingService creates a new branding service
func NewBrandingService(storageDir string) (*BrandingService, error) {
	store, err := NewBrandingStore(storageDir)
	if err != nil {
		return nil, err
	}

	// Ensure branding assets directory exists
	assetsDir := store.brandingAssetsDir()
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create branding assets directory: %w", err)
	}

	return &BrandingService{
		store: store,
	}, nil
}

// Reload re-reads the branding configuration from disk. Used after a restore
// swaps in a different branding.json — without this, GetBranding/UpdateBranding
// would keep serving the pre-restore config until the process next restarted.
func (s *BrandingService) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.Reload()
}

// GetBranding returns the current branding configuration
func (s *BrandingService) GetBranding() (*BrandingConfigResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.store.Load().ToResponse(), nil
}

// UpdateBranding updates the branding configuration
func (s *BrandingService) UpdateBranding(siteName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.Load()

	// Validate site name
	ve := sharederrors.NewValidationErrors()
	trimmedSiteName := strings.TrimSpace(siteName)

	switch {
	case trimmedSiteName == "":
		ve.Add("siteName", "Site name must not be empty")
	case len(trimmedSiteName) > cfg.BrandingConstraints.MaxSiteNameLength:
		ve.Add("siteName", fmt.Sprintf("Site name must not exceed %d characters", cfg.BrandingConstraints.MaxSiteNameLength))
	case containsControlCharacters(trimmedSiteName):
		ve.Add("siteName", "Site name contains invalid control characters")
	}

	if ve.HasErrors() {
		return ve
	}

	cfg.SiteName = trimmedSiteName

	if err := s.store.Save(cfg); err != nil {
		return sharederrors.NewLocalizedError(
			"branding_update_failed",
			"Failed to update branding",
			"failed to update branding",
			err,
		)
	}

	return nil
}

// ContainsUnsafePath reports whether s is an unsafe path component:
// contains path separators, null bytes, is absolute, or references a Windows volume.
func ContainsUnsafePath(s string) bool {
	return strings.Contains(s, "..") ||
		strings.Contains(s, "/") ||
		strings.Contains(s, "\\") ||
		filepath.IsAbs(s) ||
		filepath.VolumeName(s) != "" ||
		strings.Contains(s, "\x00")
}

// containsPathTraversal is the unexported alias used within this package.
func containsPathTraversal(s string) bool { return ContainsUnsafePath(s) }

// containsControlCharacters checks if a string contains control characters
// that could break UI layout or cause display issues.
// Blocks all control characters (unicode.IsControl) except common whitespace:
// - \t (tab, U+0009)
// - \n (newline, U+000A)
// - \r (carriage return, U+000D)
// These exceptions allow for normal text formatting while preventing
// null bytes, vertical tabs, form feeds, and other problematic characters.
func containsControlCharacters(s string) bool {
	for _, r := range s {
		// Disallow control characters except for common whitespace
		if unicode.IsControl(r) && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// UploadLogo saves a custom logo image
func (s *BrandingService) UploadLogo(file multipart.File, filename string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	assetsDir := s.store.brandingAssetsDir()
	ext := strings.ToLower(filepath.Ext(filename))

	cfg := s.store.Load()

	if !cfg.IsAllowedLogoExt(filename) {
		allowedExts := cfg.AllowedLogoExtsAsString()
		return "", sharederrors.NewLocalizedError(
			"branding_logo_invalid_type",
			"Invalid logo file type",
			"invalid logo file type %s (allowed: %s)",
			nil,
			ext,
			allowedExts,
		)
	}

	targetPath := filepath.Join(assetsDir, "logo"+ext)

	// Write new logo atomically first
	if err := shared.WriteStreamAtomic(targetPath, file, cfg.BrandingConstraints.MaxLogoSize, 0o644); err != nil {
		return "", sharederrors.NewLocalizedError(
			"branding_logo_upload_failed",
			"Failed to save logo file",
			"failed to save logo file",
			err,
		)
	}

	// Cleanup other logo.* after success
	removeOtherMatches(filepath.Join(assetsDir, "logo.*"), targetPath)

	// Update in-memory config + persist
	cfg.LogoFile = "logo" + ext
	if err := s.store.Save(cfg); err != nil {
		return "", sharederrors.NewLocalizedError(
			"branding_logo_upload_failed",
			"Failed to save logo file",
			"failed to save logo file",
			err,
		)
	}

	return cfg.LogoFile, nil
}

// DeleteLogo removes the custom logo image
func (s *BrandingService) DeleteLogo() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.Load()

	if cfg.LogoFile == "" {
		return nil // No logo to delete
	}

	if containsPathTraversal(cfg.LogoFile) {
		return sharederrors.NewLocalizedError(
			"branding_logo_delete_failed",
			errFailedToDeleteLogo,
			"invalid logo file path",
			nil,
		)
	}

	logoPath := filepath.Join(s.store.brandingAssetsDir(), cfg.LogoFile)
	if err := os.Remove(logoPath); err != nil && !os.IsNotExist(err) {
		return sharederrors.NewLocalizedError(
			"branding_logo_delete_failed",
			errFailedToDeleteLogo,
			"failed to delete logo",
			err,
		)
	}

	cfg.LogoFile = ""
	if err := s.store.Save(cfg); err != nil {
		return sharederrors.NewLocalizedError(
			"branding_logo_delete_failed",
			errFailedToDeleteLogo,
			"failed to delete logo",
			err,
		)
	}

	return nil
}

// UploadFavicon saves a custom favicon
func (s *BrandingService) UploadFavicon(file multipart.File, filename string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	assetsDir := s.store.brandingAssetsDir()
	ext := strings.ToLower(filepath.Ext(filename))

	cfg := s.store.Load()

	if !cfg.IsAllowedFaviconExt(filename) {
		allowedExts := cfg.AllowedFaviconExtsAsString()
		return "", sharederrors.NewLocalizedError(
			"branding_favicon_invalid_type",
			"Invalid favicon file type",
			"invalid favicon file type %s (allowed: %s)",
			nil,
			ext,
			allowedExts,
		)
	}

	targetPath := filepath.Join(assetsDir, "favicon"+ext)

	// Write new favicon atomically first
	if err := shared.WriteStreamAtomic(targetPath, file, cfg.BrandingConstraints.MaxFaviconSize, 0o644); err != nil {
		return "", sharederrors.NewLocalizedError(
			"branding_favicon_upload_failed",
			"Failed to save favicon file",
			"failed to save favicon file",
			err,
		)
	}

	// Cleanup other favicon.* after success
	removeOtherMatches(filepath.Join(assetsDir, "favicon.*"), targetPath)

	// Update in-memory config + persist
	cfg.FaviconFile = "favicon" + ext
	if err := s.store.Save(cfg); err != nil {
		return "", sharederrors.NewLocalizedError(
			"branding_favicon_upload_failed",
			"Failed to save favicon file",
			"failed to save favicon file",
			err,
		)
	}

	return cfg.FaviconFile, nil
}

// DeleteFavicon removes the custom favicon
func (s *BrandingService) DeleteFavicon() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.store.Load()

	if cfg.FaviconFile == "" {
		return nil // No favicon to delete
	}

	if containsPathTraversal(cfg.FaviconFile) {
		return sharederrors.NewLocalizedError(
			"branding_favicon_delete_failed",
			errFailedToDeleteFavicon,
			"invalid favicon file path",
			nil,
		)
	}

	faviconPath := filepath.Join(s.store.brandingAssetsDir(), cfg.FaviconFile)
	if err := os.Remove(faviconPath); err != nil && !os.IsNotExist(err) {
		return sharederrors.NewLocalizedError(
			"branding_favicon_delete_failed",
			errFailedToDeleteFavicon,
			"failed to delete favicon",
			err,
		)
	}

	cfg.FaviconFile = ""
	if err := s.store.Save(cfg); err != nil {
		return sharederrors.NewLocalizedError(
			"branding_favicon_delete_failed",
			errFailedToDeleteFavicon,
			"failed to delete favicon",
			err,
		)
	}

	return nil
}

// GetBrandingAssetsDir returns the branding assets directory path
func (s *BrandingService) GetBrandingAssetsDir() string {
	return s.store.brandingAssetsDir()
}
