// Package settings provides the shared persistence primitive behind
// LeafWiki's small settings-managed JSON config files living flat in the
// data directory (branding.json, public-access.json, and future ones) —
// see pleaf plan "Shared JSON Settings Store".
//
// It intentionally covers only the JSON-file side. The SQLite hot-swappable
// stores (users.db, api_keys.db, favorites.db, usersettings.db) are a
// separate, different-consistency-model concern handled in internal/restore
// directly.
//
// Deliberately NOT covered here (stays per-feature):
//   - secrets at rest (internal/backup's git-backup.json is the only one
//     that encrypts fields, via a SecretBox keyed from the JWT secret — its
//     ConfigStore keeps its own Load/Save rather than using Store[T])
//   - the env-managed vs settings-managed split (internal/publicaccess and
//     internal/backup each implement this today with a bool field + nil
//     store guard; there is no second concrete consumer of a generic
//     wrapper for it yet)
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/perber/wiki/internal/core/shared"
)

// Reloadable is anything internal/restore can re-read from disk after a
// snapshot restore swaps files in place, instead of the caller having to
// know each settings-backed service individually. *Store[T] satisfies it
// directly; a feature's own service (e.g. BrandingService) usually just
// forwards to its embedded store.
type Reloadable interface {
	Reload() error
}

// Store is an atomic, RWMutex-cached JSON file living in the data directory.
// A missing file is not an error: Get returns the zero value passed to New
// until the first Put.
type Store[T any] struct {
	path   string
	perm   os.FileMode
	onLoad func(*T)

	mu  sync.RWMutex
	val T
}

// New builds a Store for <dir>/<filename>, loading the current on-disk value
// (or zero if the file doesn't exist yet). perm is used for every write
// (e.g. 0600 for a file carrying no secrets but kept restrictive, 0644 for
// one that's fine to be world-readable). onLoad, if non-nil, is called after
// every successful load or Reload — it exists for fields that must be
// populated at runtime but never persisted (json:"-"), mirroring what
// internal/branding's BrandingStore.Load did by hand.
func New[T any](dir, filename string, perm os.FileMode, zero T, onLoad func(*T)) (*Store[T], error) {
	s := &Store[T]{
		path:   filepath.Join(dir, filename),
		perm:   perm,
		onLoad: onLoad,
		val:    zero,
	}
	if err := s.reloadLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns the current in-memory value.
func (s *Store[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.val
}

// Put marshals v as indented JSON, writes it atomically, and updates the
// in-memory cache on success. onLoad is applied to the cached copy so Get
// reflects the same runtime-injected fields a fresh Reload would produce.
func (s *Store[T]) Put(v T) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(s.path), err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := shared.WriteFileAtomic(s.path, data, s.perm); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(s.path), err)
	}
	s.val = v
	if s.onLoad != nil {
		s.onLoad(&s.val)
	}
	return nil
}

// Reload re-reads the file from disk into the in-memory cache, satisfying
// Reloadable. A missing file leaves the current in-memory value untouched
// (matching every existing store's "missing file ⇒ default, not an error"
// behavior — the default was already established at New time).
func (s *Store[T]) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked()
}

func (s *Store[T]) reloadLocked() error {
	data, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		var v T
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("parse %s: %w", filepath.Base(s.path), err)
		}
		s.val = v
	case os.IsNotExist(err):
		// Keep the current in-memory value (the zero passed to New on first
		// load). onLoad below still runs so a missing file gets the same
		// runtime-injected fields a present one would.
	default:
		return fmt.Errorf("read %s: %w", filepath.Base(s.path), err)
	}

	if s.onLoad != nil {
		s.onLoad(&s.val)
	}
	return nil
}
