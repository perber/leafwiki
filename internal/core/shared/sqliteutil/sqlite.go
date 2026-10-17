package sqliteutil

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

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
// Missing files are not an error. It tries every file and returns the joined
// errors of those it could not remove.
func RemoveSQLiteFiles(dbPath string) error {
	var errs []error
	for _, path := range sqliteFiles(dbPath) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// sqliteFiles lists a database and the sidecars SQLite may keep next to it.
func sqliteFiles(dbPath string) []string {
	return []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"}
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
//     of its sidecars (-journal, -wal, -shm), so open can start from an empty
//     path. A policy that could not do so returns retry=false with an error
//     instead. open has already closed its handle by the time the policy runs
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
// which recreates an empty database. If a file cannot be removed it does not
// retry, since opening next to a leftover sidecar could mix stale pages into
// the new database. Real-state stores still use it until steps 2 and 3 of
// the ADR-0013 rollout give them their own policies.
func RebuildOnCorruption(dbPath string, err error) (bool, error) {
	slog.Default().Warn("database corrupt, removing and retrying", "path", dbPath, "sqliteCode", sqliteCode(err), "error", err)
	if rmErr := RemoveSQLiteFiles(dbPath); rmErr != nil {
		return false, fmt.Errorf("%w (removing the corrupt database for a rebuild failed: %v)", err, rmErr)
	}
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

// quarantineTimeFormat is a UTC timestamp without ':' so the quarantined
// name is valid on Windows.
const quarantineTimeFormat = "20060102T150405Z"

// now and rename are package variables so tests can pin the quarantine name
// and simulate a failed rename.
var (
	now    = time.Now
	rename = os.Rename
)

// QuarantineOnCorruption is the CorruptionPolicy for real state that is worth
// keeping but not worth refusing to start over (sessions, email tokens,
// favorites, user settings; ADR-0013). It renames the database and every
// existing sidecar to "<name>.corrupt-<UTC timestamp>" (a sidecar keeps its
// suffix after the new name, so SQLite still pairs them when an operator
// opens the quarantined copy) and asks for a retry with an empty database.
//
// OpenWithPolicy runs it only after open has closed its handle, which the
// rename needs on Windows. If any rename fails, the files already moved are
// moved back, nothing is deleted, and the policy does not retry. It never
// overwrites an earlier quarantine with the same timestamp.
//
// It logs one Error through slog.Default(): the store keeps running, but the
// user data in the file is no longer live, so the operator has to know.
func QuarantineOnCorruption(dbPath string, err error) (bool, error) {
	target := dbPath + ".corrupt-" + now().UTC().Format(quarantineTimeFormat)
	if mvErr := moveSQLiteFiles(dbPath, target); mvErr != nil {
		return false, fmt.Errorf("%w (moving the corrupt database aside failed: %v)", err, mvErr)
	}
	slog.Default().Error("database corrupt, moved aside and continuing with an empty one",
		"path", dbPath, "quarantinedPath", target, "sqliteCode", sqliteCode(err), "error", err)
	return true, nil
}

// moveSQLiteFiles renames every existing file of the database at from to the
// matching name at to. It refuses to overwrite an existing target and, if a
// rename fails, moves the files it already renamed back.
func moveSQLiteFiles(from, to string) error {
	sources, targets := sqliteFiles(from), sqliteFiles(to)
	var pending []int
	for i := range sources {
		if _, err := os.Lstat(sources[i]); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if _, err := os.Lstat(targets[i]); !os.IsNotExist(err) {
			return fmt.Errorf("quarantine target %s already exists", targets[i])
		}
		pending = append(pending, i)
	}

	for n, i := range pending {
		if err := rename(sources[i], targets[i]); err != nil {
			errs := []error{err}
			for _, j := range slices.Backward(pending[:n]) {
				if rbErr := rename(targets[j], sources[j]); rbErr != nil {
					errs = append(errs, fmt.Errorf("moving %s back failed: %w", targets[j], rbErr))
				}
			}
			return errors.Join(errs...)
		}
	}
	return nil
}
