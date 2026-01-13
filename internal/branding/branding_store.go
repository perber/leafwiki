package branding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/perber/wiki/internal/core/shared"
)

// BrandingStore handles reading and writing branding configuration
type BrandingStore struct {
	storageDir string
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

	// Inject runtime-only validation constraints (not persisted)
	config.BrandingConstraints = DefaultBrandingConfig().BrandingConstraints

	return &config, nil
}

// Save writes the branding configuration to disk
func (s *BrandingStore) Save(config *BrandingConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal branding config: %w", err)
	}

	if err := shared.WriteFileAtomic(s.configPath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write branding config: %w", err)
	}

	return nil
}
