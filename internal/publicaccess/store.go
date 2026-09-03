package publicaccess

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/perber/wiki/internal/core/shared"
)

// store persists the settings-managed public-access flag to
// <storageDir>/public-access.json (0600). It mirrors internal/branding's
// BrandingStore: a missing file is not an error, it just means "disabled".
type store struct {
	storageDir string
}

func newStore(storageDir string) *store {
	return &store{storageDir: storageDir}
}

func (s *store) path() string {
	return filepath.Join(s.storageDir, "public-access.json")
}

// fileConfig is the on-disk shape. Kept deliberately minimal so future
// runtime-toggleable options get their own file rather than accreting here.
type fileConfig struct {
	Enabled bool `json:"enabled"`
}

// Load reads the persisted flag. A missing file ⇒ (false, nil).
func (s *store) Load() (bool, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read public-access config: %w", err)
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false, fmt.Errorf("failed to parse public-access config: %w", err)
	}
	return cfg.Enabled, nil
}

// Save atomically writes the flag with 0600 perms (it carries no secret, but
// matches the restrictive default the other data-dir config files use).
func (s *store) Save(enabled bool) error {
	data, err := json.MarshalIndent(fileConfig{Enabled: enabled}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal public-access config: %w", err)
	}
	if err := shared.WriteFileAtomic(s.path(), data, 0o600); err != nil {
		return fmt.Errorf("failed to write public-access config: %w", err)
	}
	return nil
}
