// Package pagedisplay owns the instance-wide page display settings an admin
// sets on the "Display" settings page. Today that is the "always show table
// of contents" flag: whether the page viewer's TOC panel/dropdown should be
// shown for every page, regardless of how many headings it has (see
// PageViewer.tsx's default >3-heading threshold).
//
// Unlike internal/publicaccess, this has no env/CLI-flag-pinned variant —
// these are cosmetic display preferences, not security decisions, so they
// are always settings-managed: a single JSON file in the data directory that
// an admin can change at runtime with no restart.
package pagedisplay

import (
	"fmt"

	"github.com/perber/wiki/internal/core/settings"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// fileName keeps the package's original name on disk: existing data
// directories already hold it, and backups and restore list it by name
// (internal/restore/swap.go, internal/snapshot). Renaming it would orphan
// the stored setting.
const fileName = "toc-display.json"

// fileConfig is the on-disk shape of toc-display.json.
type fileConfig struct {
	AlwaysShow bool `json:"alwaysShow"`
}

// Service is the process-wide holder of the current always-show-TOC flag.
// Safe for concurrent use — the underlying settings.Store provides locking.
type Service struct {
	store *settings.Store[fileConfig]
}

// New returns a Service whose flag is read from (and written back to)
// <storageDir>/toc-display.json. The initial value is whatever the file
// currently holds; a missing file means disabled.
func New(storageDir string) (*Service, error) {
	st, err := settings.New(storageDir, fileName, 0o600, fileConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read toc-display config: %w", err)
	}
	return &Service{store: st}, nil
}

// AlwaysShow reports whether the TOC should always be shown, regardless of
// heading count.
func (s *Service) AlwaysShow() bool {
	return s.store.Get().AlwaysShow
}

// SetAlwaysShow persists a new value and updates the in-memory cache.
func (s *Service) SetAlwaysShow(alwaysShow bool) error {
	if err := s.store.Put(fileConfig{AlwaysShow: alwaysShow}); err != nil {
		return sharederrors.NewLocalizedError(
			"toc_display_update_failed",
			"Failed to update TOC display setting",
			"failed to persist toc-display config",
			err,
		)
	}
	return nil
}

// Reload re-reads toc-display.json into the in-memory cache. Called after a
// restore swaps in a different data dir.
func (s *Service) Reload() error {
	if err := s.store.Reload(); err != nil {
		return fmt.Errorf("failed to reload toc-display config: %w", err)
	}
	return nil
}
