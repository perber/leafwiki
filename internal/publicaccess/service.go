// Package publicaccess owns the "public mode" flag: whether unauthenticated
// visitors may read every page.
//
// It has two modes, chosen once at construction, mirroring the env- vs
// settings-managed split internal/backup uses for git backup:
//
//   - env-managed (NewEnvManaged): the value is pinned by --public-access /
//     LEAFWIKI_PUBLIC_ACCESS, or forced true by --disable-auth. Enabled()
//     returns that fixed value, SetEnabled always fails with
//     ErrCodeEnvManaged, and no file is ever touched. The Settings UI shows a
//     status-only view for these instances.
//   - settings-managed (NewSettingsManaged): the value lives in
//     <storageDir>/public-access.json and an admin can toggle it at runtime
//     with no restart. A missing file means disabled.
package publicaccess

import (
	"fmt"
	"sync"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// Service is the process-wide holder of the current public-access flag. All
// methods are safe for concurrent use.
type Service struct {
	mu         sync.RWMutex
	enabled    bool
	envManaged bool
	store      *store // nil in env-managed mode
}

// NewEnvManaged returns a Service pinned to enabled; SetEnabled/Reload are
// inert. Used when --public-access / LEAFWIKI_PUBLIC_ACCESS is set or
// --disable-auth forces public mode on.
func NewEnvManaged(enabled bool) *Service {
	return &Service{enabled: enabled, envManaged: true}
}

// NewSettingsManaged returns a Service whose flag is read from (and written
// back to) <storageDir>/public-access.json. The initial value is whatever the
// file currently holds; a missing file means disabled.
func NewSettingsManaged(storageDir string) (*Service, error) {
	st := newStore(storageDir)
	enabled, err := st.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load public-access config: %w", err)
	}
	return &Service{enabled: enabled, store: st}, nil
}

// Enabled reports whether anonymous read access is currently allowed.
func (s *Service) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// EnvManaged reports whether the flag is pinned by environment configuration
// (and therefore read-only in the Settings UI). Immutable after construction.
func (s *Service) EnvManaged() bool {
	return s.envManaged
}

// SetEnabled persists a new value and updates the in-memory cache. It returns
// a *LocalizedError with code ErrCodeEnvManaged on an env-managed instance.
func (s *Service) SetEnabled(enabled bool) error {
	if s.envManaged {
		return errEnvManaged()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.store.Save(enabled); err != nil {
		return sharederrors.NewLocalizedError(
			"public_access_update_failed",
			"Failed to update public access mode",
			"failed to persist public-access config",
			err,
		)
	}
	s.enabled = enabled
	return nil
}

// Reload re-reads public-access.json into the in-memory cache. Called after a
// restore swaps in a different data dir. A no-op (nil) for env-managed
// instances, which have no file.
func (s *Service) Reload() error {
	if s.envManaged {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	enabled, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("failed to reload public-access config: %w", err)
	}
	s.enabled = enabled
	return nil
}
