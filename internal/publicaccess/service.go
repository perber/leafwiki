// Package publicaccess owns the "public mode" flag: whether unauthenticated
// visitors may read every page.
//
// It has two modes, chosen once at construction, mirroring the env- vs
// settings-managed split internal/backup uses for git backup. Both are
// carried as a single settings.Value[fileConfig]:
//
//   - env-managed (NewEnvManaged): a settings.Fixed value pinned by
//     --public-access / LEAFWIKI_PUBLIC_ACCESS, or forced true by
//     --disable-auth. Enabled() returns that fixed value, SetEnabled always
//     fails with ErrCodeEnvManaged, and no file is ever touched. The Settings
//     UI shows a status-only view for these instances.
//   - settings-managed (NewSettingsManaged): a settings.Managed value living
//     in <storageDir>/public-access.json that an admin can toggle at runtime
//     with no restart. A missing file means disabled.
package publicaccess

import (
	"errors"
	"fmt"

	"github.com/perber/wiki/internal/core/settings"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// fileConfig is the on-disk shape of public-access.json. Deliberately a
// single field so future runtime-toggleable options get their own file
// rather than accreting here.
type fileConfig struct {
	Enabled bool `json:"enabled"`
}

// Service is the process-wide holder of the current public-access flag. All
// methods are safe for concurrent use — the underlying settings.Value
// provides the locking (settings.Managed) or is immutable (settings.Fixed).
type Service struct {
	val settings.Value[fileConfig]
}

// NewEnvManaged returns a Service pinned to enabled; SetEnabled/Reload are
// inert. Used when --public-access / LEAFWIKI_PUBLIC_ACCESS is set or
// --disable-auth forces public mode on.
func NewEnvManaged(enabled bool) *Service {
	return &Service{val: settings.NewFixed(fileConfig{Enabled: enabled})}
}

// NewSettingsManaged returns a Service whose flag is read from (and written
// back to) <storageDir>/public-access.json. The initial value is whatever the
// file currently holds; a missing file means disabled.
func NewSettingsManaged(storageDir string) (*Service, error) {
	st, err := settings.New(storageDir, "public-access.json", 0o600, fileConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read public-access config: %w", err)
	}
	return &Service{val: settings.Managed[fileConfig]{Store: st}}, nil
}

// Enabled reports whether anonymous read access is currently allowed.
func (s *Service) Enabled() bool {
	return s.val.Get().Enabled
}

// EnvManaged reports whether the flag is pinned by environment configuration
// (and therefore read-only in the Settings UI). Immutable after construction.
func (s *Service) EnvManaged() bool {
	return s.val.EnvManaged()
}

// SetEnabled persists a new value and updates the in-memory cache. It returns
// a *LocalizedError with code ErrCodeEnvManaged on an env-managed instance.
func (s *Service) SetEnabled(enabled bool) error {
	switch err := s.val.Put(fileConfig{Enabled: enabled}); {
	case errors.Is(err, settings.ErrEnvManaged):
		return errEnvManaged()
	case err != nil:
		return sharederrors.NewLocalizedError(
			"public_access_update_failed",
			"Failed to update public access mode",
			"failed to persist public-access config",
			err,
		)
	}
	return nil
}

// Reload re-reads public-access.json into the in-memory cache. Called after a
// restore swaps in a different data dir. A no-op (nil) for env-managed
// instances, which have no file.
func (s *Service) Reload() error {
	if err := s.val.Reload(); err != nil {
		return fmt.Errorf("failed to reload public-access config: %w", err)
	}
	return nil
}
