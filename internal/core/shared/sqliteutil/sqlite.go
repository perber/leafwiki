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

// CorruptionPolicy decides what happens when a store's open function fails
// with a recoverable SQLite I/O or corruption error (see
// IsSQLiteRecoverableError). Each store names its policy at its constructor,
// because only the store knows whether its database is a derived index that
// can be rebuilt or real state that must never be deleted automatically
// (ADR-0013).
//
// Contract, enforced by OpenWithPolicy and its policy table test:
//
//   - The policy is only called for recoverable errors, and at most once per
//     OpenWithPolicy call; err is the error open returned.
//   - retry=true means the policy has removed or moved the database and all
//     of its sidecars (-journal, -wal, -shm), or attempted to, so open can
//     start from an empty path; a failed removal surfaces as the retry's
//     error. open has already closed its handle by the time the policy runs
//     (see OpenWithPolicy). out is ignored when retry is true.
//   - retry=false means OpenWithPolicy returns out without retrying. out must
//     then be non-nil; a policy returning (false, nil) breaks the contract and
//     OpenWithPolicy returns the original err, so a faulty policy can never
//     turn a failed open into a store that looks healthy.
type CorruptionPolicy func(dbPath string, err error) (retry bool, out error)

// OpenWithPolicy calls open, which is expected to fully bring a store to a
// working state (open the connection and ensure its schema exists) and to
// close any handle it opened before returning an error. If open fails with a
// recoverable SQLite error, policy decides whether open runs exactly once
// more. Non-recoverable errors and the error of the retry are returned
// unchanged.
//
// Each store keeps its own opening mechanics (eager sql.Open, a lazy
// Connect(), or a fully lazy withDB helper) inside the closure; only the
// recovery sequence is shared. policy must not be nil: there is deliberately
// no default, so a new store has to choose (ADR-0013). A nil policy panics up
// front, not only on corruption, so every constructor test catches it.
func OpenWithPolicy(dbPath string, policy CorruptionPolicy, open func() error) error {
	if policy == nil {
		panic("sqliteutil: nil CorruptionPolicy")
	}
	err := open()
	if err == nil {
		return nil
	}
	if !IsSQLiteRecoverableError(err) {
		return err
	}
	retry, out := policy(dbPath, err)
	if !retry {
		if out == nil {
			return err
		}
		return out
	}
	return open()
}

// RebuildOnCorruption is the CorruptionPolicy for derived indexes (links,
// tags, properties, search) that are rebuilt from the Markdown files
// (ADR-0001): it deletes the database and its sidecars and asks for a retry,
// which recreates an empty database. Real-state stores still use it until
// steps 2 and 3 of the ADR-0013 rollout give them their own policies.
func RebuildOnCorruption(dbPath string, err error) (bool, error) {
	slog.Default().Warn("database corrupt, removing and retrying", "path", dbPath, "sqliteCode", sqliteCode(err), "error", err)
	RemoveSQLiteFiles(dbPath)
	return true, nil
}

// sqliteCode returns the extended SQLite result code of err, or 0 if err is
// not a SQLite error, so operators can tell IOERR from CORRUPT or NOTADB.
func sqliteCode(err error) int {
	var e *sqlite.Error
	if !errors.As(err, &e) {
		return 0
	}
	return e.Code()
}
