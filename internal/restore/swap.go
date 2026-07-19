package restore

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

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
// unconditionally (root/, assets/, branding/, branding.json, and schema.json
// are all skipped by the writer when the corresponding source is empty or
// missing, so their absence in a given ZIP isn't itself invalid).
var requiredZipEntries = []string{"backup-meta.json", "users.db"}

// extractAndValidate opens zipPath, verifies the required entries exist,
// extracts everything into a fresh staging directory *inside* dataDir (not
// the OS temp dir — so the later swap can use os.Rename instead of a
// cross-filesystem copy), and sanity-checks the staged users.db by running a
// trivial query against it — all before any live file is touched. The caller
// must os.RemoveAll the returned staging dir once done with it.
func extractAndValidate(zipPath, dataDir string) (stagingDir string, meta backupMeta, err error) {
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

	for _, f := range r.File {
		if err := extractZipEntry(f, stagingDir); err != nil {
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

	if err := sanityCheckUsersDB(filepath.Join(stagingDir, "users.db")); err != nil {
		_ = os.RemoveAll(stagingDir)
		return "", backupMeta{}, fmt.Errorf("staged users.db failed sanity check: %w", err)
	}

	return stagingDir, meta, nil
}

// extractZipEntry writes a single zip entry under destDir, rejecting any
// entry whose path would escape destDir (zip slip).
func extractZipEntry(f *zip.File, destDir string) error {
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

	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return nil
}

func sanityCheckUsersDB(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	var count int
	if err := db.QueryRow("SELECT count(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("failed to query users table: %w", err)
	}
	return nil
}

// swapNames lists every top-level item a snapshot ZIP may contain, in the
// order they're swapped. All are optional in the staging dir — see
// newSwapper's doc comment.
var swapNames = []string{"root", "assets", "branding", "branding.json", "schema.json", "users.db"}

// swapItem is one live path <-> staged path pair.
type swapItem struct {
	name       string
	livePath   string
	stagedPath string
	preRestore string
	// swapped is true once this item's live path has actually been renamed
	// aside and the staged replacement moved in (only items present in the
	// staging dir get swapped at all).
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

		if err := os.Rename(item.stagedPath, item.livePath); err != nil {
			return fmt.Errorf("failed to move in restored %s: %w", item.name, err)
		}
		item.swapped = true
	}
	return nil
}

// RollbackAll reverses every item SwapAll already committed: removes the
// restored content and moves each .pre-restore-* copy back into place.
// Best-effort — accumulates every error rather than stopping at the first, so
// every item still gets attempted even if one fails.
func (sw *swapper) RollbackAll() error {
	var errs []error
	for _, item := range sw.items {
		if !item.swapped {
			continue
		}
		if err := os.RemoveAll(item.livePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to remove restored content: %w", item.name, err))
			continue
		}
		if _, statErr := os.Stat(item.preRestore); statErr != nil {
			// Nothing was renamed aside (the live item didn't exist before
			// the swap) — nothing to restore, the item is now correctly absent.
			item.swapped = false
			continue
		}
		if err := os.Rename(item.preRestore, item.livePath); err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to restore pre-restore copy: %w", item.name, err))
			continue
		}
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
