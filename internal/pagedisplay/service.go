// Package pagedisplay owns the instance-wide page display settings an admin
// sets on the "Display" settings page: the "always show table of contents"
// flag (whether the page viewer's TOC panel/dropdown is shown for every
// page, regardless of PageViewer.tsx's default >3-heading threshold) and the
// page content width (see content_width.go).
//
// Unlike internal/publicaccess, this has no env/CLI-flag-pinned variant —
// these are cosmetic display preferences, not security decisions, so they
// are always settings-managed: a single JSON file in the data directory that
// an admin can change at runtime with no restart.
package pagedisplay

import (
	"fmt"
	"sync"

	"github.com/perber/wiki/internal/core/settings"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// FileName is the settings file inside the data directory. It keeps the
// package's original name on disk: existing data directories already hold
// it, and backups and restore list it by name (internal/restore/swap.go,
// internal/snapshot). Renaming it would orphan the stored setting.
const FileName = "toc-display.json"

// fileConfig is the on-disk shape of toc-display.json.
type fileConfig struct {
	AlwaysShow   bool   `json:"alwaysShow"`
	ContentWidth string `json:"contentWidth,omitempty"`
}

// Service is the process-wide holder of the current page display settings.
// Safe for concurrent use: the underlying settings.Store locks reads and
// writes, and writeMu serializes the setters' read-modify-write (and Reload)
// so changing one setting never drops a concurrent change to the other.
type Service struct {
	store   *settings.Store[fileConfig]
	writeMu sync.Mutex
}

// New returns a Service whose flag is read from (and written back to)
// <storageDir>/toc-display.json. The initial value is whatever the file
// currently holds; a missing file means disabled.
func New(storageDir string) (*Service, error) {
	st, err := settings.New(storageDir, FileName, 0o600, fileConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read toc-display config: %w", err)
	}
	return &Service{store: st}, nil
}

// Settings returns the current page display settings.
func (s *Service) Settings() Settings {
	cfg := s.store.Get()
	return Settings{
		AlwaysShowToc: cfg.AlwaysShow,
		ContentWidth:  normalizeContentWidth(cfg.ContentWidth),
	}
}

// Update validates every field of patch, then applies them all in one write.
// An invalid field fails the whole patch (ErrCodeInvalidContentWidth) and
// changes nothing; a failed write returns ErrCodeUpdateFailed. It returns
// the settings after the update.
func (s *Service) Update(patch Patch) (Settings, error) {
	if err := s.update(patch, persistError{
		code:     ErrCodeUpdateFailed,
		message:  "Failed to update page display setting",
		template: "failed to update page display setting",
	}); err != nil {
		return Settings{}, err
	}
	return s.Settings(), nil
}

// SetAlwaysShow persists a new value and updates the in-memory cache. A
// failed write keeps its historical code toc_display_update_failed.
func (s *Service) SetAlwaysShow(alwaysShow bool) error {
	return s.update(Patch{AlwaysShowToc: &alwaysShow}, persistError{
		code:     "toc_display_update_failed",
		message:  "Failed to update TOC display setting",
		template: "failed to persist toc-display config",
	})
}

// SetContentWidth validates and persists a new page content width. A value
// outside ContentWidths() fails with ErrCodeInvalidContentWidth and changes
// nothing.
func (s *Service) SetContentWidth(width string) error {
	return s.update(Patch{ContentWidth: &width}, persistError{
		code:     ErrCodeUpdateFailed,
		message:  "Failed to update page display setting",
		template: "failed to update page display setting",
	})
}

// Reload re-reads toc-display.json into the in-memory cache. Called after a
// restore swaps in a different data dir. It holds writeMu like the setters,
// so a setter cannot read the old cache before the reload and write it back
// over the restored values after it. Lock order is always writeMu, then the
// store's own lock.
func (s *Service) Reload() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := s.store.Reload(); err != nil {
		return fmt.Errorf("failed to reload toc-display config: %w", err)
	}
	return nil
}

// persistError is the LocalizedError a caller reports when the write fails;
// the setters keep the codes they returned before Update existed.
type persistError struct {
	code, message, template string
}

// update is the shared read-modify-write behind Update and the setters:
// validate first, then one Put under writeMu, so nothing is half-applied.
func (s *Service) update(patch Patch, onPersistErr persistError) error {
	if patch.ContentWidth != nil && !isValidContentWidth(*patch.ContentWidth) {
		return errInvalidContentWidth(*patch.ContentWidth)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	cfg := s.store.Get()
	if patch.AlwaysShowToc != nil {
		cfg.AlwaysShow = *patch.AlwaysShowToc
	}
	if patch.ContentWidth != nil {
		cfg.ContentWidth = *patch.ContentWidth
	}
	if err := s.store.Put(cfg); err != nil {
		return sharederrors.NewLocalizedError(onPersistErr.code, onPersistErr.message, onPersistErr.template, err)
	}
	return nil
}
