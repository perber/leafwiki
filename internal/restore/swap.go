package restore

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/perber/wiki/internal/core/shared"
	_ "modernc.org/sqlite" // Import SQLite driver
)

// backupMeta mirrors internal/snapshot's unexported backupMeta struct — the
// shape written to backup-meta.json inside every snapshot ZIP.
type backupMeta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Version   string    `json:"version"`
}

// requiredZipEntries are the only entries createSnapshot always writes
// unconditionally (root/, assets/, branding/, avatars/, branding.json, and
// schema.json are all skipped by the writer when the corresponding source is
// empty or missing, so their absence in a given ZIP isn't itself invalid).
var requiredZipEntries = []string{"backup-meta.json", "users.db"}

// extractAndValidate opens zipPath, verifies the required entries exist,
// extracts everything into a fresh staging directory *inside* dataDir (not
// the OS temp dir — so the later swap can use os.Rename instead of a
// cross-filesystem copy), and sanity-checks the staged users.db by running a
// trivial query against it — all before any live file is touched. The caller
// must os.RemoveAll the returned staging dir once done with it.
func extractAndValidate(zipPath, dataDir string) (stagingDir string, meta backupMeta, err error) {
	return extractAndValidateWithLimits(zipPath, dataDir, shared.DefaultZipExtractionLimits)
}

func extractAndValidateWithLimits(zipPath, dataDir string, limits shared.ExtractionLimits) (stagingDir string, meta backupMeta, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", backupMeta{}, fmt.Errorf("failed to open snapshot zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	present := map[string]bool{}
	for _, f := range r.File {
		present[f.Name] = true
	}
	for _, want := range requiredZipEntries {
		if !present[want] {
			return "", backupMeta{}, fmt.Errorf("snapshot zip is missing required entry %q", want)
		}
	}

	stagingDir, err = os.MkdirTemp(dataDir, ".leafwiki-restore-*")
	if err != nil {
		return "", backupMeta{}, fmt.Errorf("failed to create staging directory: %w", err)
	}

	budget := shared.NewSizeBudget(limits.MaxTotalBytes)
	for _, f := range r.File {
		if err := extractZipEntry(f, stagingDir, limits, budget); err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", backupMeta{}, fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}
	}

	metaBytes, err := os.ReadFile(filepath.Join(stagingDir, "backup-meta.json"))
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", backupMeta{}, fmt.Errorf("failed to read backup-meta.json: %w", err)
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", backupMeta{}, fmt.Errorf("failed to parse backup-meta.json: %w", err)
	}

	if err := sanityCheckSQLiteDB(filepath.Join(stagingDir, "users.db"), "users", usersRequiredColumns); err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", backupMeta{}, fmt.Errorf("staged users.db failed sanity check: %w", err)
	}

	// api_keys.db, favorites.db, and usersettings.db are all optional (API
	// key management is off by default; an older snapshot may predate the
	// favorites/usersettings feature), so only sanity-check them when the
	// snapshot actually staged one — same presence guard SwapAll itself uses.
	if _, err := os.Stat(filepath.Join(stagingDir, "api_keys.db")); err == nil {
		if err := sanityCheckSQLiteDB(filepath.Join(stagingDir, "api_keys.db"), "api_keys", apiKeysRequiredColumns); err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", backupMeta{}, fmt.Errorf("staged api_keys.db failed sanity check: %w", err)
		}
	}
	if _, err := os.Stat(filepath.Join(stagingDir, "favorites.db")); err == nil {
		if err := sanityCheckSQLiteDB(filepath.Join(stagingDir, "favorites.db"), "favorites", favoritesRequiredColumns); err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", backupMeta{}, fmt.Errorf("staged favorites.db failed sanity check: %w", err)
		}
	}
	if _, err := os.Stat(filepath.Join(stagingDir, "usersettings.db")); err == nil {
		if err := sanityCheckSQLiteDB(filepath.Join(stagingDir, "usersettings.db"), "user_settings", userSettingsRequiredColumns); err != nil {
			_ = os.RemoveAll(stagingDir)
			return "", backupMeta{}, fmt.Errorf("staged usersettings.db failed sanity check: %w", err)
		}
	}

	return stagingDir, meta, nil
}

// extractZipEntry writes a single zip entry under destDir, rejecting any
// entry whose path would escape destDir (zip slip).
func extractZipEntry(f *zip.File, destDir string, limits shared.ExtractionLimits, budget *shared.SizeBudget) error {
	cleanName := filepath.Clean(f.Name)
	if cleanName == "." || strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
		return fmt.Errorf("unsafe entry path %q", f.Name)
	}
	destPath := filepath.Join(destDir, cleanName)
	rel, err := filepath.Rel(destDir, destPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("unsafe entry path %q", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if err := shared.CopyWithBudget(out, rc, f.CompressedSize64, limits, budget); err != nil {
		return err
	}
	return nil
}

// usersRequiredColumns, apiKeysRequiredColumns, favoritesRequiredColumns,
// and userSettingsRequiredColumns are the columns sanityCheckSQLiteDB probes
// for each staged database. usersRequiredColumns deliberately lists only the
// columns present in users.db's original schema, not ones an additive
// migration adds later (totp_secret_encrypted/totp_enabled/
// totp_recovery_codes_json/totp_enabled_at/totp_last_reset_at, added by
// UserStore.ensureTOTPColumns; must_set_password, added by
// ensureMustSetPasswordColumn) — an older snapshot predating one of those
// migrations is still legitimately restorable (Replace's fresh UserStore
// reopen re-runs the migration against it afterward), and requiring those
// columns here would reject it before it ever gets that chance. api_keys.db,
// favorites.db, and usersettings.db have no such migration history (each is
// created by a single CREATE TABLE IF NOT EXISTS with no later ALTER TABLE),
// so their lists are simply their full current schemas.
var (
	usersRequiredColumns        = []string{"id", "username", "password", "email", "role", "created_at"}
	apiKeysRequiredColumns      = []string{"id", "name", "user_id", "prefix", "key_hash", "role", "expires_at", "created_by", "created_at", "last_used_at", "revoked_at"}
	favoritesRequiredColumns    = []string{"user_id", "page_id", "created_at"}
	userSettingsRequiredColumns = []string{"user_id", "language", "autosave", "updated_at"}
)

// sanityCheckSQLiteDB opens path, runs PRAGMA integrity_check, and probes
// table for columns — proving the staged file is both a structurally sound
// SQLite database (catching a corrupt page or index that a bare
// table-existence query can't see) and one whose table actually has the
// shape this restore expects (catching a same-named table with different or
// missing columns, which a plain "SELECT count(*)" can't tell apart from the
// real thing). Without this, a corrupt or wrong-shaped file would get
// swapped in and, for favorites.db/usersettings.db specifically,
// RetryOnCorruption (see NewFavoritesStore/NewUserSettingsStore) would then
// silently delete and recreate it empty the moment it's reopened — with the
// restore still reporting success either way.
func sanityCheckSQLiteDB(path, table string, columns []string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	var integrity string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("failed to run integrity check: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check failed: %s", integrity)
	}

	// LIMIT 0 fetches no rows but still forces SQLite to resolve every named
	// column against the table's actual schema, so a missing table or a
	// missing/renamed column fails the query the same way — no rows need to
	// exist in the table for this to work.
	rows, err := db.Query(fmt.Sprintf("SELECT %s FROM %s LIMIT 0", strings.Join(columns, ", "), table))
	if err != nil {
		return fmt.Errorf("failed to query %s schema: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to query %s schema: %w", table, err)
	}
	return nil
}

// swapNames lists every top-level item a snapshot ZIP may contain, in the
// order they're swapped. All are optional in the staging dir — see
// newSwapper's doc comment. sessions.db is deliberately absent: it's never
// part of the snapshot (session state is ephemeral, tied to the running
// process) — see AuthService.InvalidateAllSessions, called post-swap instead
// of being restored. "avatars" is a plain per-user-avatar asset directory
// (like "assets"/"branding") — no hot-swappable service owns it, so it
// carries no reload/rollback step of its own.
var swapNames = []string{"root", "assets", "branding", "avatars", "branding.json", "schema.json", "users.db", "api_keys.db", "favorites.db", "usersettings.db"}

// walSidecarDBNames lists every WAL-mode database whose stale -wal/-shm
// sidecars may need cleaning up before a swap — derived from swapNames
// itself (every entry that's a .db file; the non-DB entries —
// root/assets/branding/schema.json — never run in WAL mode) so the two
// lists can't drift apart when a future store gets added. Shared by
// manager.go and offline.go.
var walSidecarDBNames = func() []string {
	names := make([]string, 0, len(swapNames))
	for _, name := range swapNames {
		if strings.HasSuffix(name, ".db") {
			names = append(names, name)
		}
	}
	return names
}()

// removeStaleWALSidecars deletes dbPath's -wal and -shm sidecar files, if
// present, without touching dbPath itself. Missing files are not an error.
//
// users.db runs in WAL mode (internal/core/auth/user_store.go). A snapshot
// ZIP's users.db is always a plain, sidecar-free single file — createSnapshot
// (internal/snapshot) produces it via `VACUUM INTO`, which never emits a
// -wal/-shm — so the *staged* replacement never has this problem. The live
// side is the concern: before a swap, the live UserStore is either closed
// (live restore, via UserStore.suspend — SQLite auto-checkpoints and
// truncates the WAL when the last connection to a WAL-mode database closes)
// or was never opened by this process at all (offline `restore-snapshot`,
// run before the server starts). Either way, by the time a swap runs, any
// -wal/-shm left at the live path is expected to be an empty/harmless
// checkpoint remnant, not unflushed data — but leaving it in place is still
// a bad idea: swapNames renames only users.db itself, so a stale sidecar
// would be left sitting next to the *replacement* users.db, and a WAL file
// that doesn't belong to the database next to it is exactly the kind of
// ambiguous state worth not creating, however unlikely SQLite is to
// misinterpret it. Called before newSwapper/SwapAll for the live users.db
// path in both the live-restore (manager.go) and offline (offline.go)
// paths.
func removeStaleWALSidecars(dbPath string) error {
	var errs []error
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// swapItem is one live path <-> staged path pair.
type swapItem struct {
	name       string
	livePath   string
	stagedPath string
	preRestore string
	// movedAside is true once SwapAll has passed the move-aside step for this
	// item — i.e. either the live path existed and was successfully renamed
	// to preRestore, or it didn't exist at all (nothing to move aside).
	// Tracked independently of swapped so that if the *second* rename
	// (staged -> live) then fails, RollbackAll still knows to check for and
	// restore a preRestore copy instead of silently leaving the item missing.
	movedAside bool
	// swapped is true once this item's staged replacement was fully moved
	// into place (implies movedAside; only items present in the staging dir
	// get swapped at all).
	swapped bool
}

// swapper drives the rename-aside/rename-in dance for every restorable item,
// on both POSIX and Windows (os.Rename isn't atomic-over-an-existing-target
// on Windows, so this two-step sequence is used uniformly rather than having
// a separate Windows code path).
type swapper struct {
	items []*swapItem
}

// newSwapper prepares the live<->staged path pairs. Items not present in
// stagingDir (e.g. an empty assets/ dir at snapshot time, or an older
// snapshot taken before branding.json was captured) are left untouched by
// SwapAll rather than cleared — restore only ever brings back what the
// snapshot actually captured, it never deletes live content the snapshot
// simply didn't record.
func newSwapper(dataDir, stagingDir string) *swapper {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	items := make([]*swapItem, 0, len(swapNames))
	for _, name := range swapNames {
		live := filepath.Join(dataDir, name)
		items = append(items, &swapItem{
			name:       name,
			livePath:   live,
			stagedPath: filepath.Join(stagingDir, name),
			preRestore: live + ".pre-restore-" + stamp,
		})
	}
	return &swapper{items: items}
}

// SwapAll performs rename-aside then rename-in for every item present in the
// staging dir, stopping at the first failure. It does not roll back on
// failure itself — the caller decides how to handle a partial swap (see
// Manager.rollbackOrIntervene), since the same rollback path is shared with
// failures from later phases (auth reopen, branding reload).
func (sw *swapper) SwapAll() error {
	for _, item := range sw.items {
		if _, err := os.Stat(item.stagedPath); err != nil {
			continue // not captured by this snapshot; leave the live item alone
		}

		if _, err := os.Stat(item.livePath); err == nil {
			if err := os.Rename(item.livePath, item.preRestore); err != nil {
				return fmt.Errorf("failed to move aside %s: %w", item.name, err)
			}
		}
		item.movedAside = true

		if err := os.Rename(item.stagedPath, item.livePath); err != nil {
			return fmt.Errorf("failed to move in restored %s: %w", item.name, err)
		}
		item.swapped = true
	}
	return nil
}

// RollbackAll reverses every item SwapAll touched: removes any restored
// content currently at the live path and moves each .pre-restore-* copy back
// into place. Driven by movedAside, not swapped — an item whose move-aside
// succeeded but whose move-in then failed (SwapAll's second rename) has no
// live content of its own at all right now, but still has a pre-restore copy
// that must be put back; checking only swapped would skip it and leave the
// item permanently missing. Best-effort — accumulates every error rather than
// stopping at the first, so every item still gets attempted even if one fails.
func (sw *swapper) RollbackAll() error {
	var errs []error
	for _, item := range sw.items {
		// swapped always implies movedAside (SwapAll only ever sets swapped
		// after movedAside, in the same iteration), so movedAside alone is
		// the correct "did SwapAll touch this item at all" check.
		if !item.movedAside {
			continue
		}
		// Safe no-op if the move-in step never ran (nothing to remove).
		if err := os.RemoveAll(item.livePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to remove restored content: %w", item.name, err))
			continue
		}
		if _, statErr := os.Stat(item.preRestore); statErr != nil {
			// Nothing was renamed aside (the live item didn't exist before
			// the swap) — nothing to restore, the item is now correctly absent.
			item.movedAside = false
			item.swapped = false
			continue
		}
		if err := os.Rename(item.preRestore, item.livePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to restore pre-restore copy: %w", item.name, err))
			continue
		}
		item.movedAside = false
		item.swapped = false
	}
	return errors.Join(errs...)
}

// CommitAll deletes every .pre-restore-* backup copy. Only call once the
// entire restore sequence (swap + auth reopen + branding reload) has
// succeeded — see Manager.runLocked.
func (sw *swapper) CommitAll() {
	for _, item := range sw.items {
		if !item.swapped {
			continue
		}
		if err := os.RemoveAll(item.preRestore); err != nil {
			slog.Default().Warn("restore: failed to clean up pre-restore backup copy", "item", item.name, "path", item.preRestore, "error", err)
		}
	}
}
