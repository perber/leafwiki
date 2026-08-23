package restore

import (
	"fmt"
	"os"
	"path/filepath"

	coreshared "github.com/perber/wiki/internal/core/shared"
)

// RestoreOffline validates and swaps a snapshot ZIP directly into dataDir,
// without any of the live-instance machinery (no write gate, no AuthService
// reopen, no resync trigger). Intended to run before the server starts —
// the next NewWiki() cold boot picks up the restored users.db/tree/etc for
// free. Used by `leafwiki restore-snapshot`.
//
// zipPath is a local filesystem path the operator running this CLI command
// chose directly — running it already requires the same filesystem access
// the caps would be defending, so extraction runs unrestricted rather than
// under DefaultZipExtractionLimits, matching the by-id online-restore path.
func RestoreOffline(dataDir, zipPath string) error {
	stagingDir, _, err := extractAndValidateWithLimits(zipPath, dataDir, coreshared.UnrestrictedExtractionLimits)
	if err != nil {
		return fmt.Errorf("snapshot validation failed: %w", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

	// No live connection exists in this offline path (the server hasn't
	// started yet), so any leftover -wal/-shm here is from a prior unclean
	// shutdown, not in-flight state — see removeStaleWALSidecars for why
	// this still needs cleaning up before the swap.
	if err := removeStaleWALSidecars(filepath.Join(dataDir, "users.db")); err != nil {
		return fmt.Errorf("failed to clean up stale users.db WAL files before swap: %w", err)
	}
	// api_keys.db runs in WAL mode too (internal/core/auth/apikey_store.go) —
	// same stale-sidecar risk as users.db once it's part of swapNames, same
	// fix.
	if err := removeStaleWALSidecars(filepath.Join(dataDir, "api_keys.db")); err != nil {
		return fmt.Errorf("failed to clean up stale api_keys.db WAL files before swap: %w", err)
	}
	// favorites.db and usersettings.db run in WAL mode too, same stale-sidecar
	// risk, same fix.
	if err := removeStaleWALSidecars(filepath.Join(dataDir, "favorites.db")); err != nil {
		return fmt.Errorf("failed to clean up stale favorites.db WAL files before swap: %w", err)
	}
	if err := removeStaleWALSidecars(filepath.Join(dataDir, "usersettings.db")); err != nil {
		return fmt.Errorf("failed to clean up stale usersettings.db WAL files before swap: %w", err)
	}

	sw := newSwapper(dataDir, stagingDir)
	if err := sw.SwapAll(); err != nil {
		if rbErr := sw.RollbackAll(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v; data directory may need manual repair)", err, rbErr)
		}
		return fmt.Errorf("failed to swap restored files (rolled back, data directory untouched): %w", err)
	}

	sw.CommitAll()
	return nil
}
