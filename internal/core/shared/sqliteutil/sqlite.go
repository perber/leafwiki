package sqliteutil

import (
	"errors"
	"log/slog"
	"os"

	sqlite "modernc.org/sqlite"
)

// IsSQLiteRecoverableError reports whether err is a SQLite I/O or corruption
// error that may be resolved by deleting the database files and retrying.
// Transient errors (SQLITE_BUSY, SQLITE_LOCKED) and resource errors
// (SQLITE_IOERR_NOMEM) are not considered recoverable.
func IsSQLiteRecoverableError(err error) bool {
	var e *sqlite.Error
	if !errors.As(err, &e) {
		return false
	}
	code := e.Code()
	// SQLITE_IOERR_NOMEM (10 | 12<<8 = 3082): SQLite could not allocate
	// memory during an I/O operation. The file itself is not corrupt;
	// deleting it would not help and would destroy the index unnecessarily.
	if code == 10|(12<<8) {
		return false
	}
	// Primary error code lives in the low 8 bits of the extended code.
	// SQLITE_IOERR=10, SQLITE_CORRUPT=11, SQLITE_NOTADB=26
	switch code & 0xFF {
	case 10, 11, 26:
		return true
	}
	return false
}

// RemoveSQLiteFiles deletes a SQLite database and any sidecar files
// (-journal, -wal, -shm) that may have been left behind by a crashed run.
func RemoveSQLiteFiles(dbPath string) {
	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Default().Warn("failed to remove database file", "path", path, "error", err)
		}
	}
}

// RetryOnCorruption calls open, which is expected to fully bring a store to
// a working state (open the connection and ensure its schema exists),
// cleaning up after itself on failure. If open's error is a recoverable
// SQLite I/O/corruption error, the database files at dbPath are removed and
// open is called exactly once more; any other error, or a second failure,
// is returned as-is.
//
// This factors out the "open → ensure schema → on recoverable corruption,
// wipe and retry once" sequence that is otherwise duplicated, near-verbatim,
// across every per-domain SQLite store constructor (tags, links, properties,
// search, favorites, users, sessions). Each store keeps its own opening
// mechanics (eager sql.Open, a lazy Connect(), or a fully lazy withDB helper)
// inside the closure; only the recovery policy is shared.
func RetryOnCorruption(dbPath string, open func() error) error {
	err := open()
	if err == nil {
		return nil
	}
	if !IsSQLiteRecoverableError(err) {
		return err
	}
	slog.Default().Warn("database corrupt, removing and retrying", "path", dbPath, "error", err)
	RemoveSQLiteFiles(dbPath)
	return open()
}
