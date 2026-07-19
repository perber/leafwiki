package snapshot

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Import SQLite driver
)

type Config struct {
	BackupsDir         string
	RootDir            string
	AssetsDir          string
	BrandingDir        string
	BrandingConfigFile string // storageDir/branding.json (site name, active logo/favicon filename) — separate from BrandingDir, which only holds the uploaded logo/favicon image files
	SchemaFile         string
	UsersDBPath        string
	WikiVersion        string // injected from build info

	// Interval is the automatic snapshot interval; 0 means manual-only
	// (no ticker, only explicit triggers). Mirrors backup.Config.Interval.
	Interval time.Duration
	// RetentionCount is how many of the newest snapshots to keep after each
	// successful run; <= 0 means unlimited (no pruning).
	RetentionCount int
}

type backupMeta struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Version   string    `json:"version"`
}

// createSnapshot builds the ZIP and sidecar JSON, returns the snapshot ID.
func createSnapshot(ctx context.Context, cfg Config) (string, error) {
	if err := os.MkdirAll(cfg.BackupsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %w", err)
	}

	id, err := uniqueSnapshotID(cfg.BackupsDir, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("failed to allocate snapshot id: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "leafwiki-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	usersDBCopy := filepath.Join(tmpDir, "users.db")
	if err := vacuumUsersDB(ctx, cfg.UsersDBPath, usersDBCopy); err != nil {
		return "", fmt.Errorf("failed to vacuum users database: %w", err)
	}

	createdAt := time.Now().UTC()
	zipPath := filepath.Join(cfg.BackupsDir, id+".zip")
	if err := writeSnapshotZip(zipPath, cfg, id, createdAt, usersDBCopy); err != nil {
		_ = os.Remove(zipPath) // best-effort cleanup of a partial/invalid zip left by the failed write
		return "", fmt.Errorf("failed to write snapshot zip: %w", err)
	}

	info, err := os.Stat(zipPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat snapshot zip: %w", err)
	}

	entry := SnapshotEntry{ID: id, CreatedAt: createdAt, SizeBytes: info.Size()}
	sidecarPath := filepath.Join(cfg.BackupsDir, id+".json")
	if err := writeJSONFile(sidecarPath, entry); err != nil {
		return "", fmt.Errorf("failed to write snapshot sidecar: %w", err)
	}

	return id, nil
}

// uniqueSnapshotID returns a snapshot ID based on t that has no existing
// .zip or .json file in backupsDir yet. IDs are second-precision timestamps,
// so two runs completing within the same second would otherwise collide and
// silently overwrite each other's snapshot; this appends a "-2", "-3", ...
// suffix until a free slot is found.
func uniqueSnapshotID(backupsDir string, t time.Time) (string, error) {
	base := "snapshot-" + t.Format("20060102-150405")
	id := base
	for i := 2; ; i++ {
		_, zipErr := os.Stat(filepath.Join(backupsDir, id+".zip"))
		_, jsonErr := os.Stat(filepath.Join(backupsDir, id+".json"))
		if os.IsNotExist(zipErr) && os.IsNotExist(jsonErr) {
			return id, nil
		}
		if zipErr != nil && !os.IsNotExist(zipErr) {
			return "", zipErr
		}
		if jsonErr != nil && !os.IsNotExist(jsonErr) {
			return "", jsonErr
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

// vacuumBusyTimeout bounds how long VACUUM INTO waits for a writer to
// release its lock before failing with SQLITE_BUSY. users.db has no WAL
// mode or busy_timeout configured elsewhere, so without this a snapshot
// taken while an admin action (e.g. user create/update) is mid-write would
// fail immediately instead of just waiting the write out.
const vacuumBusyTimeout = 5 * time.Second

func vacuumUsersDB(ctx context.Context, srcPath, dstPath string) error {
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(%d)", srcPath, vacuumBusyTimeout.Milliseconds())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open users database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", dstPath); err != nil {
		return fmt.Errorf("failed to vacuum into %s: %w", dstPath, err)
	}
	return nil
}

func writeSnapshotZip(zipPath string, cfg Config, id string, createdAt time.Time, usersDBPath string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)

	if err := addDirToZip(w, cfg.RootDir, "root/"); err != nil {
		return err
	}
	if err := addDirToZip(w, cfg.AssetsDir, "assets/"); err != nil {
		return err
	}
	if err := addDirToZip(w, cfg.BrandingDir, "branding/"); err != nil {
		return err
	}
	if err := addFileToZip(w, cfg.BrandingConfigFile, "branding.json"); err != nil {
		return err
	}
	if err := addFileToZip(w, cfg.SchemaFile, "schema.json"); err != nil {
		return err
	}
	if err := addFileToZip(w, usersDBPath, "users.db"); err != nil {
		return err
	}

	metaBytes, err := json.Marshal(backupMeta{ID: id, CreatedAt: createdAt, Version: cfg.WikiVersion})
	if err != nil {
		return fmt.Errorf("failed to marshal backup metadata: %w", err)
	}
	if err := addBytesToZip(w, "backup-meta.json", metaBytes); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %w", err)
	}
	return nil
}

// addDirToZip walks srcDir and adds every file under it to the zip, prefixed
// with prefix. A missing srcDir (e.g. optional BrandingDir) is not an error.
func addDirToZip(w *zip.Writer, srcDir, prefix string) error {
	if srcDir == "" {
		return nil
	}
	if _, err := os.Stat(srcDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat %s: %w", srcDir, err)
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to compute relative path for %s: %w", path, err)
		}
		return addFileToZip(w, path, prefix+filepath.ToSlash(rel))
	})
}

// addFileToZip adds a single file to the zip under name. A missing srcFile
// (e.g. optional SchemaFile) is not an error.
func addFileToZip(w *zip.Writer, srcFile, name string) error {
	if srcFile == "" {
		return nil
	}
	f, err := os.Open(srcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", srcFile, err)
	}
	defer func() { _ = f.Close() }()

	zw, err := w.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create zip entry %s: %w", name, err)
	}
	if _, err := io.Copy(zw, f); err != nil {
		return fmt.Errorf("failed to write zip entry %s: %w", name, err)
	}
	return nil
}

func addBytesToZip(w *zip.Writer, name string, data []byte) error {
	zw, err := w.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create zip entry %s: %w", name, err)
	}
	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("failed to write zip entry %s: %w", name, err)
	}
	return nil
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}
