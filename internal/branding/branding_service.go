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
	"github.com/perber/wiki/internal/core/shared/errors"
)

// BrandingService provides branding operations
type BrandingService struct {
	store          *BrandingStore
	brandingConfig *BrandingConfig
	mu             sync.RWMutex
}

// NewBrandingService creates a new branding service
func NewBrandingService(storageDir string) (*BrandingService, error) {
	store := NewBrandingStore(storageDir)

	// Ensure branding assets directory exists
	assetsDir := store.brandingAssetsDir()
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create branding assets directory: %w", err)
	}
	brandingConfig, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load branding config: %w", err)
	}

	return &BrandingService{
		store:          store,
		brandingConfig: brandingConfig,
	}, nil
}

// GetBranding returns the current branding configuration
func (s *BrandingService) GetBranding() (*BrandingConfigResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.brandingConfig.ToResponse(), nil
}

// UpdateBranding updates the branding configuration
func (s *BrandingService) UpdateBranding(siteName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate site name
	ve := errors.NewValidationErrors()
	trimmedSiteName := strings.TrimSpace(siteName)

	if trimmedSiteName == "" {
		ve.Add("siteName", "Site name must not be empty")
	} else if len(trimmedSiteName) > s.brandingConfig.BrandingConstraints.MaxSiteNameLength {
		ve.Add("siteName", fmt.Sprintf("Site name must not exceed %d characters", s.brandingConfig.BrandingConstraints.MaxSiteNameLength))
	} else if containsControlCharacters(trimmedSiteName) {
		ve.Add("siteName", "Site name contains invalid control characters")
	}

	if ve.HasErrors() {
		return ve
	}

	s.brandingConfig.SiteName = trimmedSiteName

	if err := s.store.Save(s.brandingConfig); err != nil {
		return err
	}

	return nil
}

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

	if !s.brandingConfig.IsAllowedLogoExt(filename) {
		allowedExts := s.brandingConfig.AllowedLogoExtsAsString()
		return "", fmt.Errorf("invalid logo file type: %s (allowed: %s)", ext, allowedExts)
	}

	targetPath := filepath.Join(assetsDir, "logo"+ext)

	// Write new logo atomically first
	if err := shared.WriteStreamAtomic(targetPath, file, s.brandingConfig.BrandingConstraints.MaxLogoSize); err != nil {
		return "", fmt.Errorf("failed to save logo file: %w", err)
	}

	// Cleanup other logo.* after success
	removeOtherMatches(filepath.Join(assetsDir, "logo.*"), targetPath)

	// Update in-memory config + persist
	s.brandingConfig.LogoFile = "logo" + ext
	if err := s.store.Save(s.brandingConfig); err != nil {
		return "", err
	}

	return s.brandingConfig.LogoFile, nil
}

// DeleteLogo removes the custom logo image
func (s *BrandingService) DeleteLogo() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.brandingConfig.LogoFile == "" {
		return nil // No logo to delete
	}

	logoPath := filepath.Join(s.store.brandingAssetsDir(), s.brandingConfig.LogoFile)
	if err := os.Remove(logoPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete logo file: %w", err)
	}

	s.brandingConfig.LogoFile = ""
	if err := s.store.Save(s.brandingConfig); err != nil {
		return err
	}

	return nil
}

// UploadFavicon saves a custom favicon
func (s *BrandingService) UploadFavicon(file multipart.File, filename string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	assetsDir := s.store.brandingAssetsDir()
	ext := strings.ToLower(filepath.Ext(filename))

	if !s.brandingConfig.IsAllowedFaviconExt(filename) {
		allowedExts := s.brandingConfig.AllowedFaviconExtsAsString()
		return "", fmt.Errorf("invalid favicon file type: %s (allowed: %s)", ext, allowedExts)
	}

	targetPath := filepath.Join(assetsDir, "favicon"+ext)

	// Write new favicon atomically first
	if err := shared.WriteStreamAtomic(targetPath, file, s.brandingConfig.BrandingConstraints.MaxFaviconSize); err != nil {
		return "", fmt.Errorf("failed to save favicon file: %w", err)
	}

	// Cleanup other favicon.* after success
	removeOtherMatches(filepath.Join(assetsDir, "favicon.*"), targetPath)

	// Update in-memory config + persist
	s.brandingConfig.FaviconFile = "favicon" + ext
	if err := s.store.Save(s.brandingConfig); err != nil {
		return "", err
	}

	return s.brandingConfig.FaviconFile, nil
}

// DeleteFavicon removes the custom favicon
func (s *BrandingService) DeleteFavicon() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.brandingConfig.FaviconFile == "" {
		return nil // No favicon to delete
	}

	faviconPath := filepath.Join(s.store.brandingAssetsDir(), s.brandingConfig.FaviconFile)
	if err := os.Remove(faviconPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete favicon file: %w", err)
	}

	s.brandingConfig.FaviconFile = ""
	if err := s.store.Save(s.brandingConfig); err != nil {
		return err
	}

	return nil
}

// GetBrandingAssetsDir returns the branding assets directory path
func (s *BrandingService) GetBrandingAssetsDir() string {
	return s.store.brandingAssetsDir()
}
