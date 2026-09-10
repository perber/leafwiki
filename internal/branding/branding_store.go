package branding

import (
	"fmt"
	"path/filepath"

	"github.com/perber/wiki/internal/core/settings"
)

// BrandingStore persists BrandingConfig to <storageDir>/branding.json, backed
// by the shared settings.Store[T] primitive (atomic write, cached read,
// Reload after a restore).
type BrandingStore struct {
	storageDir string
	store      *settings.Store[BrandingConfig]
}

// NewBrandingStore creates a new branding store, loading whatever is
// currently on disk (or DefaultBrandingConfig if branding.json doesn't exist
// yet).
func NewBrandingStore(storageDir string) (*BrandingStore, error) {
	st, err := settings.New(storageDir, "branding.json", 0o644, *DefaultBrandingConfig(), injectBrandingConstraints)
	if err != nil {
		return nil, fmt.Errorf("failed to load branding config: %w", err)
	}
	return &BrandingStore{storageDir: storageDir, store: st}, nil
}

// injectBrandingConstraints re-populates the non-persisted (json:"-")
// BrandingConstraints field after every load/reload, mirroring what the
// hand-written Load() used to do inline.
func injectBrandingConstraints(cfg *BrandingConfig) {
	cfg.BrandingConstraints = DefaultBrandingConfig().BrandingConstraints
}

func (s *BrandingStore) brandingAssetsDir() string {
	return filepath.Join(s.storageDir, "branding")
}

// Load returns the current branding configuration.
func (s *BrandingStore) Load() *BrandingConfig {
	cfg := s.store.Get()
	return &cfg
}

// Save writes the branding configuration to disk and updates the cache.
func (s *BrandingStore) Save(config *BrandingConfig) error {
	if err := s.store.Put(*config); err != nil {
		return fmt.Errorf("failed to write branding config: %w", err)
	}
	return nil
}

// Reload re-reads branding.json from disk, satisfying settings.Reloadable.
func (s *BrandingStore) Reload() error {
	if err := s.store.Reload(); err != nil {
		return fmt.Errorf("failed to reload branding config: %w", err)
	}
	return nil
}
