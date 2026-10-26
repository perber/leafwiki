package restore

import (
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/settings"
	"github.com/perber/wiki/internal/snapshot"
)

// SwappableStore is a SQLite store holding real state (ADR-0013) that a live
// restore releases before the file swap and reopens afterwards.
//
// PauseForSwap closes the store's connection and keeps it closed (requests
// get a "store unavailable" error) until Replace; it is idempotent. On
// Windows an open handle blocks renaming the file, which is why every store
// must be paused before SwapAll. Replace reopens the store from dataDir,
// whatever is on disk there now.
type SwappableStore interface {
	PauseForSwap() error
	Replace(dataDir string) error
}

// LiveStore is a SwappableStore together with its database file name in the
// data directory (e.g. "favorites.db"), used in errors and logs. File must be
// listed in stateDBs so snapshots capture it and restores swap it.
type LiveStore struct {
	File  string
	Store SwappableStore
}

// Config holds everything the restore Manager needs to validate, stage, and
// swap a snapshot back into a live instance.
type Config struct {
	// SnapshotManager resolves a snapshot id to its ZIP path (SnapshotZipPath
	// is also the security boundary against path traversal — see
	// internal/snapshot.Manager).
	SnapshotManager *snapshot.Manager
	// DataDir is the instance's data directory (contains root/, assets/,
	// branding/, avatars/, branding.json, schema.json, users.db). The restore staging
	// directory is created inside DataDir (not the OS temp dir) so the final
	// swap can use os.Rename instead of a cross-filesystem copy.
	DataDir string
	// WikiVersion is this binary's version, compared against the snapshot's
	// recorded version to surface a (non-fatal) mismatch warning.
	WikiVersion string
	// WriteGate is engaged for the duration of the file swap so mutating
	// requests don't land in a directory that's about to be renamed away.
	WriteGate *WriteGate
	// AuthService's user store (users.db) is hot-swapped in place; its
	// session store is untouched (sessions.db isn't part of the snapshot).
	AuthService *auth.AuthService
	// Stores are the live state stores (api_keys.db, favorites.db,
	// usersettings.db, …) that are hot-swapped in place, mirroring
	// AuthService's users.db handling. The composition root registers only
	// the stores that exist in this process: a disabled optional feature
	// (e.g. API key management, off by default) is simply not listed. Its
	// file is still swapped on disk if the snapshot holds one (see
	// stateDBs), so enabling the feature later picks up the restored data.
	Stores []LiveStore
	// Reloadables are the settings-managed JSON-config services (branding,
	// public-access, and future ones built on internal/core/settings.Store[T])
	// whose in-memory cache is reloaded from disk after the file swap. A nil
	// entry is skipped; env-managed services (e.g. a public-access.Service
	// pinned by --public-access) may still be included since their Reload is
	// a no-op.
	Reloadables []settings.Reloadable
	// UserResolver's own in-memory author-label cache is reloaded after
	// AuthService.ReplaceUserStore succeeds — the live UserService pointer
	// alone doesn't invalidate labels already cached before the restore. nil
	// is tolerated (every use below is nil-guarded) for callers/tests that
	// don't wire one up.
	UserResolver *auth.UserResolver
	// TriggerResync rebuilds the derived tree/search/links/tags/properties
	// indexes from the restored root/assets. Typically wiki.Wiki.TriggerResyncAsync.
	TriggerResync func()
	// MaxUploadSizeBytes bounds how large an uploaded backup ZIP
	// (TriggerRestoreFromUpload, via POST /restore/upload) may be. <= 0 falls
	// back to DefaultMaxUploadSizeBytes.
	MaxUploadSizeBytes int64
}
