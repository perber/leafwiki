package restore

import (
	"github.com/perber/wiki/internal/branding"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/snapshot"
)

// Config holds everything the restore Manager needs to validate, stage, and
// swap a snapshot back into a live instance.
type Config struct {
	// SnapshotManager resolves a snapshot id to its ZIP path (SnapshotZipPath
	// is also the security boundary against path traversal — see
	// internal/snapshot.Manager).
	SnapshotManager *snapshot.Manager
	// DataDir is the instance's data directory (contains root/, assets/,
	// branding/, branding.json, schema.json, users.db). The restore staging
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
	// BrandingService's in-memory config cache is reloaded from the restored
	// branding.json after the file swap.
	BrandingService *branding.BrandingService
	// TriggerResync rebuilds the derived tree/search/links/tags/properties
	// indexes from the restored root/assets. Typically wiki.Wiki.TriggerResyncAsync.
	TriggerResync func()
}
