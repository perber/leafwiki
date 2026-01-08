package branding

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BrandingConfig holds the branding configuration for the wiki
type BrandingConfig struct {
	SiteName         string `json:"siteName"`
	LogoImagePath    string `json:"logoImagePath"`    // path to custom logo image
	FaviconImagePath string `json:"faviconImagePath"` // path to custom favicon
}

// DefaultBrandingConfig returns the default branding configuration
func DefaultBrandingConfig() *BrandingConfig {
	return &BrandingConfig{
		SiteName:         "LeafWiki",
		LogoImagePath:    "",
		FaviconImagePath: "",
	}
}

// BrandingStore handles reading and writing branding configuration
type BrandingStore struct {
	storageDir string
	mu         sync.RWMutex
}

// NewBrandingStore creates a new branding store
func NewBrandingStore(storageDir string) *BrandingStore {
	return &BrandingStore{
		storageDir: storageDir,
	}
}

func (s *BrandingStore) configPath() string {
	return filepath.Join(s.storageDir, "branding.json")
}

func (s *BrandingStore) brandingAssetsDir() string {
	return filepath.Join(s.storageDir, "branding")
}

// Load reads the branding configuration from disk
func (s *BrandingStore) Load() (*BrandingConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultBrandingConfig(), nil
		}
		return nil, fmt.Errorf("failed to read branding config: %w", err)
	}

	var config BrandingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse branding config: %w", err)
	}

	return &config, nil
}

// Save writes the branding configuration to disk
func (s *BrandingStore) Save(config *BrandingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal branding config: %w", err)
	}

	if err := os.WriteFile(s.configPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write branding config: %w", err)
	}

	return nil
}

// BrandingService provides branding operations
type BrandingService struct {
	store *BrandingStore
}

// NewBrandingService creates a new branding service
func NewBrandingService(storageDir string) (*BrandingService, error) {
	store := NewBrandingStore(storageDir)

	// Ensure branding assets directory exists
	assetsDir := store.brandingAssetsDir()
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create branding assets directory: %w", err)
	}

	return &BrandingService{
		store: store,
	}, nil
}

// GetBranding returns the current branding configuration
func (s *BrandingService) GetBranding() (*BrandingConfig, error) {
	return s.store.Load()
}

// UpdateBranding updates the branding configuration
func (s *BrandingService) UpdateBranding(siteName string) error {
	config, err := s.store.Load()
	if err != nil {
		return err
	}

	if siteName != "" {
		config.SiteName = siteName
	}

	return s.store.Save(config)
}

// UploadLogo saves a custom logo image
func (s *BrandingService) UploadLogo(file multipart.File, filename string) (string, error) {
	assetsDir := s.store.brandingAssetsDir()

	// Clean filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".svg" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
		return "", fmt.Errorf("invalid logo file type: %s (allowed: png, svg, jpg, jpeg, webp)", ext)
	}

	targetPath := filepath.Join(assetsDir, "logo"+ext)

	// Remove old logo files
	oldLogos, _ := filepath.Glob(filepath.Join(assetsDir, "logo.*"))
	for _, old := range oldLogos {
		os.Remove(old)
	}

	// Save new logo
	out, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create logo file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("failed to save logo file: %w", err)
	}

	// Update config
	config, err := s.store.Load()
	if err != nil {
		return "", err
	}

	config.LogoImagePath = "logo" + ext

	if err := s.store.Save(config); err != nil {
		return "", err
	}

	return config.LogoImagePath, nil
}

// UploadFavicon saves a custom favicon
func (s *BrandingService) UploadFavicon(file multipart.File, filename string) (string, error) {
	assetsDir := s.store.brandingAssetsDir()

	// Clean filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".svg" && ext != ".ico" && ext != ".webp" {
		return "", fmt.Errorf("invalid favicon file type: %s (allowed: png, svg, ico, webp)", ext)
	}

	targetPath := filepath.Join(assetsDir, "favicon"+ext)

	// Remove old favicon files
	oldFavicons, _ := filepath.Glob(filepath.Join(assetsDir, "favicon.*"))
	for _, old := range oldFavicons {
		os.Remove(old)
	}

	// Save new favicon
	out, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create favicon file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("failed to save favicon file: %w", err)
	}

	// Update config
	config, err := s.store.Load()
	if err != nil {
		return "", err
	}

	config.FaviconImagePath = "favicon" + ext

	if err := s.store.Save(config); err != nil {
		return "", err
	}

	return config.FaviconImagePath, nil
}

// GetBrandingAssetsDir returns the branding assets directory path
func (s *BrandingService) GetBrandingAssetsDir() string {
	return s.store.brandingAssetsDir()
}
