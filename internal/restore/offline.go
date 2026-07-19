package restore

import (
	"fmt"
	"os"
)

// RestoreOffline validates and swaps a snapshot ZIP directly into dataDir,
// without any of the live-instance machinery (no write gate, no AuthService
// reopen, no resync trigger). Intended to run before the server starts —
// the next NewWiki() cold boot picks up the restored users.db/tree/etc for
// free. Used by `leafwiki restore-snapshot`.
func RestoreOffline(dataDir, zipPath string) error {
	stagingDir, _, err := extractAndValidate(zipPath, dataDir)
	if err != nil {
		return fmt.Errorf("snapshot validation failed: %w", err)
	}
	defer func() { _ = os.RemoveAll(stagingDir) }()

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
