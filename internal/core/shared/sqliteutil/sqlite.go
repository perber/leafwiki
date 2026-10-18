package sqliteutil

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
)

// IsSQLiteRecoverableError reports whether err is a SQLite I/O or corruption
// error that may be resolved by deleting the database files and retrying.
// Transient errors (SQLITE_BUSY, SQLITE_LOCKED) and resource errors
// (SQLITE_IOERR_NOMEM) are not considered recoverable.
func IsSQLiteRecoverableError(err error) bool {
	if errors.Is(err, errEmptyWithSidecar) {
		return true
	}
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
//     OpenWithPolicy call; err is the error of the read-only probe or of open
//     (see OpenWithPolicy).
//   - retry=true means the policy has removed or moved the database and all
//     of its sidecars (-journal, -wal, -shm), so open can start from an empty
//     path. A policy that could not do so returns retry=false with an error
//     instead. No handle to the database is open when the policy runs: the
//     probe closes its own, and open closes its handle before returning an
//     error (see OpenWithPolicy). out is ignored when retry is true.
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
// Before open runs, an existing non-empty file is probed read-only. If the
// probe already fails with a recoverable error, policy gets that error and
// open runs only if policy asks for a retry. A read-write connection to a
// database whose header SQLite cannot read discards the -wal, even one that
// holds committed transactions, so without the probe that sidecar would be
// gone before FailOnCorruption or QuarantineOnCorruption could keep it. The
// guard fails closed: if the probe cannot read an existing file for any
// other reason, OpenWithPolicy returns that error and open never runs. The
// one exception is SQLITE_READONLY_* (a hot journal or a WAL that needs
// recovery): only a read-write connection can do that recovery, so open runs.
// The probe only reads the header and the schema; corruption deeper in the
// file still surfaces from open, after a read-write connection was made.
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
	err := probeReadOnly(dbPath)
	switch {
	case err == nil || needsWriteToRecover(err):
		err = open()
		if err == nil {
			return nil
		}
		if !IsSQLiteRecoverableError(err) {
			return err
		}
	case !IsSQLiteRecoverableError(err):
		return fmt.Errorf("could not check %s read-only before opening it: %w", dbPath, err)
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

// errEmptyWithSidecar marks a 0-byte database next to a non-empty -wal or
// -journal. The sidecar can only belong to a database that held data, so the
// main file was truncated or replaced; a read-write open would treat it as a
// new database and drop the sidecar. It counts as recoverable, so the store's
// policy decides what to keep.
var errEmptyWithSidecar = errors.New("database file is empty but its -wal or -journal is not")

// probeReadOnly opens an existing, non-empty database read-only, reads its
// schema and closes it again, returning the error SQLite reports. A missing
// file is a valid new database and is not probed; so is an empty one, unless
// a non-empty -wal or -journal sits next to it (errEmptyWithSidecar).
func probeReadOnly(dbPath string) error {
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		for _, sidecar := range []string{dbPath + "-wal", dbPath + "-journal"} {
			if sideInfo, statErr := os.Stat(sidecar); statErr == nil && sideInfo.Mode().IsRegular() && sideInfo.Size() > 0 {
				return fmt.Errorf("%s: %w", dbPath, errEmptyWithSidecar)
			}
		}
		return nil
	}
	dsn, err := readOnlyDSN(dbPath)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec("SELECT count(*) FROM sqlite_master")
	return err
}

// readOnlyDSN builds a read-only URI for dbPath. mode=ro is only honoured in
// URI form (a plain path with ?mode=ro opens read-write), and a URI path must
// be absolute: a relative one such as the default ./data would become
// file:///data/... and fail to open.
func readOnlyDSN(dbPath string) (string, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return "", err
	}
	path := filepath.ToSlash(stripLongPathPrefix(abs))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // Windows drive letter: file:///C:/...
	}
	// A UNC path (//server/share/...) gets an empty authority:
	// file:////server/share/..., which SQLite on Windows opens as UNC.
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(5000)"}
	return u.String(), nil
}

// stripLongPathPrefix removes the Windows extended-length prefix, which a
// file: URI cannot express: \\?\C:\x becomes C:\x and \\?\UNC\srv\share
// becomes \\srv\share.
func stripLongPathPrefix(p string) string {
	if rest, ok := strings.CutPrefix(p, `\\?\UNC\`); ok {
		return `\\` + rest
	}
	return strings.TrimPrefix(p, `\\?\`)
}

// needsWriteToRecover reports a probe error that only a read-write
// connection can resolve: SQLITE_READONLY_ROLLBACK (a hot journal) or
// SQLITE_READONLY_RECOVERY (a WAL that needs recovery), both primary code
// SQLITE_READONLY (8). That is SQLite's own crash recovery, not corruption.
func needsWriteToRecover(err error) bool {
	return sqliteCode(err)&0xFF == 8
}

// RebuildOnCorruption is the CorruptionPolicy for derived indexes (links,
// tags, properties, search) that are rebuilt from the Markdown files
// (ADR-0001): it deletes the database and its sidecars and asks for a retry,
// which recreates an empty database. If a file cannot be removed it does not
// retry, since opening next to a leftover sidecar could mix stale pages into
// the new database. Never use it for real state (ADR-0013).
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

// CorruptDatabaseError reports a database that could not be opened because of
// a SQLite I/O or corruption error and was not replaced (ADR-0013). The edge
// (cmd/leafwiki) finds it with errors.As and tells the operator the path, the
// SQLite code and the ways out.
type CorruptDatabaseError struct {
	Path string
	Err  error
	// Stranded lists files a failed quarantine moved aside and could not
	// move back; they now carry a .corrupt-* name. Empty means every file is
	// still where it was.
	Stranded []string
}

func (e *CorruptDatabaseError) Error() string {
	msg := fmt.Sprintf("database %s is unreadable (SQLite code %d): %v", e.Path, e.SQLiteCode(), e.Err)
	if len(e.Stranded) > 0 {
		msg += "; left under quarantine names: " + strings.Join(e.Stranded, ", ")
	}
	return msg
}

func (e *CorruptDatabaseError) Unwrap() error { return e.Err }

// SQLiteCode returns the extended SQLite result code of the underlying
// error, or 0 if it is not a SQLite error.
func (e *CorruptDatabaseError) SQLiteCode() int { return sqliteCode(e.Err) }

// FailOnCorruption is the CorruptionPolicy for real state that must never be
// replaced automatically (users.db, ADR-0013): it touches no file and
// returns a *CorruptDatabaseError, so the store's constructor and startup
// fail. It does not log; the caller reports the error once (ADR-0008).
func FailOnCorruption(dbPath string, err error) (bool, error) {
	return false, &CorruptDatabaseError{Path: dbPath, Err: err}
}

// quarantineTimeFormat is a UTC timestamp without ':' so the quarantined
// name is valid on Windows.
const quarantineTimeFormat = "20060102T150405Z"

// now and rename are package variables so tests can pin the quarantine name
// and simulate a failed rename. Tests that replace them must not run in
// parallel.
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
// moved back, nothing is deleted, and the policy returns a
// *CorruptDatabaseError instead of retrying. It never
// overwrites an earlier quarantine with the same timestamp.
//
// It logs one Error through slog.Default(): the store keeps running, but the
// user data in the file is no longer live, so the operator has to know.
func QuarantineOnCorruption(dbPath string, err error) (bool, error) {
	target := dbPath + ".corrupt-" + now().UTC().Format(quarantineTimeFormat)
	if stranded, mvErr := moveSQLiteFiles(dbPath, target); mvErr != nil {
		return false, &CorruptDatabaseError{Path: dbPath, Err: fmt.Errorf("%w (moving it aside failed: %v)", err, mvErr), Stranded: stranded}
	}
	slog.Default().Error("database corrupt, moved aside and continuing with an empty one",
		"path", dbPath, "quarantinedPath", target, "sqliteCode", sqliteCode(err), "error", err)
	return true, nil
}

// moveSQLiteFiles renames every existing file of the database at from to the
// matching name at to. It refuses to overwrite an existing target and, if a
// rename fails, moves the files it already renamed back. On error it returns
// the target names of files it could not move back.
func moveSQLiteFiles(from, to string) ([]string, error) {
	sources, targets := sqliteFiles(from), sqliteFiles(to)
	var pending []int
	for i := range sources {
		if _, err := os.Lstat(sources[i]); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if _, err := os.Lstat(targets[i]); !os.IsNotExist(err) {
			return nil, fmt.Errorf("quarantine target %s already exists", targets[i])
		}
		pending = append(pending, i)
	}

	for n, i := range pending {
		if err := rename(sources[i], targets[i]); err != nil {
			errs := []error{err}
			var stranded []string
			for _, j := range slices.Backward(pending[:n]) {
				if rbErr := rename(targets[j], sources[j]); rbErr != nil {
					errs = append(errs, fmt.Errorf("moving %s back failed: %w", targets[j], rbErr))
					stranded = append(stranded, targets[j])
				}
			}
			return stranded, errors.Join(errs...)
		}
	}
	return nil, nil
}
