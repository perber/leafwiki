package sqliteutil

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	sqlite "modernc.org/sqlite"
)

func TestIsSQLiteRecoverableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "SQLITE_IOERR",
			err:  sqliteErrorWithCode(10),
			want: true,
		},
		{
			name: "SQLITE_CORRUPT",
			err:  sqliteErrorWithCode(11),
			want: true,
		},
		{
			name: "SQLITE_NOTADB",
			err:  sqliteErrorWithCode(26),
			want: true,
		},
		{
			name: "SQLITE_IOERR_NOMEM",
			err:  sqliteErrorWithCode(10 | (12 << 8)),
			want: false,
		},
		{
			name: "SQLITE_BUSY",
			err:  sqliteErrorWithCode(5),
			want: false,
		},
		{
			name: "SQLITE_LOCKED",
			err:  sqliteErrorWithCode(6),
			want: false,
		},
		{
			name: "non-sqlite error",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSQLiteRecoverableError(tt.err); got != tt.want {
				t.Fatalf("IsSQLiteRecoverableError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveSQLiteFiles(t *testing.T) {
	all := []string{"", "-journal", "-wal", "-shm"}
	tests := []struct {
		name     string
		existing []string // suffixes written as files before the call
		blocked  string   // suffix replaced by an undeletable directory, "" for none
		wantErr  bool
	}{
		{name: "removes the db and every sidecar", existing: all},
		{name: "missing files are not an error", existing: nil},
		{name: "reports a failure and still removes the rest", existing: []string{"", "-journal", "-shm"}, blocked: "-wal", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "search.db")
			for _, suffix := range tt.existing {
				writeTestFile(t, dbPath+suffix, "x")
			}
			if tt.blocked != "" {
				blockRemoval(t, dbPath+tt.blocked)
			}

			err := RemoveSQLiteFiles(dbPath)

			if (err != nil) != tt.wantErr {
				t.Fatalf("RemoveSQLiteFiles error = %v, want error: %v", err, tt.wantErr)
			}
			for _, suffix := range all {
				if tt.blocked != "" && suffix == tt.blocked {
					if _, statErr := os.Stat(dbPath + suffix); statErr != nil {
						t.Errorf("expected the blocked %q to remain, err=%v", dbPath+suffix, statErr)
					}
					continue
				}
				assertMissing(t, dbPath+suffix)
			}
		})
	}
}

// TestOpenWithPolicy_Contract pins the contract every CorruptionPolicy relies
// on (ADR-0013), using a recording fake policy so the rows test OpenWithPolicy
// itself rather than any concrete policy.
func TestOpenWithPolicy_Contract(t *testing.T) {
	corrupt := sqliteErrorWithCode(11) // SQLITE_CORRUPT
	notADB := sqliteErrorWithCode(26)  // SQLITE_NOTADB
	boom := errors.New("boom")
	stillBroken := errors.New("still broken")
	typed := errors.New("typed policy error")

	tests := []struct {
		name          string
		openErrs      []error // error returned by the n-th open call; nil = success
		policyRetry   bool
		policyOut     error
		wantErr       error
		wantOpenCalls int
		wantPolicy    bool
	}{
		{
			name:          "success on first try does not call the policy",
			openErrs:      []error{nil},
			wantOpenCalls: 1,
		},
		{
			name:          "non-recoverable error passes through without calling the policy",
			openErrs:      []error{boom},
			wantErr:       boom,
			wantOpenCalls: 1,
		},
		{
			name:          "retry then success",
			openErrs:      []error{corrupt, nil},
			policyRetry:   true,
			wantOpenCalls: 2,
			wantPolicy:    true,
		},
		{
			name:          "retry then non-recoverable failure returns that error unchanged",
			openErrs:      []error{corrupt, stillBroken},
			policyRetry:   true,
			wantErr:       stillBroken,
			wantOpenCalls: 2,
			wantPolicy:    true,
		},
		{
			name:          "retry then recoverable failure is not retried again",
			openErrs:      []error{corrupt, notADB},
			policyRetry:   true,
			wantErr:       notADB,
			wantOpenCalls: 2,
			wantPolicy:    true,
		},
		{
			name:          "retry ignores a non-nil out",
			openErrs:      []error{corrupt, nil},
			policyRetry:   true,
			policyOut:     typed,
			wantOpenCalls: 2,
			wantPolicy:    true,
		},
		{
			name:          "no retry returns the policy error",
			openErrs:      []error{corrupt},
			policyOut:     typed,
			wantErr:       typed,
			wantOpenCalls: 1,
			wantPolicy:    true,
		},
		{
			name:          "no retry with nil out returns the original error",
			openErrs:      []error{corrupt},
			wantErr:       corrupt,
			wantOpenCalls: 1,
			wantPolicy:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const dbPath = "unused.db"
			openCalls, policyCalls := 0, 0
			policy := func(gotPath string, gotErr error) (bool, error) {
				policyCalls++
				if gotPath != dbPath {
					t.Errorf("policy got path %q, want %q", gotPath, dbPath)
				}
				if !errors.Is(gotErr, tt.openErrs[0]) {
					t.Errorf("policy got err %v, want the first open error %v", gotErr, tt.openErrs[0])
				}
				return tt.policyRetry, tt.policyOut
			}

			err := OpenWithPolicy(dbPath, policy, func() error {
				openCalls++
				if openCalls > len(tt.openErrs) {
					t.Fatalf("open called %d times, more than the %d scripted results", openCalls, len(tt.openErrs))
				}
				return tt.openErrs[openCalls-1]
			})

			if tt.wantErr == nil && err != nil {
				t.Fatalf("OpenWithPolicy: unexpected error %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("OpenWithPolicy error = %v, want exactly %v", err, tt.wantErr)
			}
			if openCalls != tt.wantOpenCalls {
				t.Fatalf("open called %d times, want %d", openCalls, tt.wantOpenCalls)
			}
			wantPolicyCalls := 0
			if tt.wantPolicy {
				wantPolicyCalls = 1
			}
			if policyCalls != wantPolicyCalls {
				t.Fatalf("policy called %d times, want %d", policyCalls, wantPolicyCalls)
			}
		})
	}
}

func TestOpenWithPolicy_NilPolicyPanics(t *testing.T) {
	openCalls := 0
	defer func() {
		if recover() == nil {
			t.Fatal("expected OpenWithPolicy to panic on a nil policy")
		}
		if openCalls != 0 {
			t.Fatalf("expected the panic before open runs, open called %d times", openCalls)
		}
	}()
	_ = OpenWithPolicy("unused.db", nil, func() error {
		openCalls++
		return nil
	})
}

// TestOpenWithPolicy_RebuildOnCorruption covers the derived-index policy:
// a recoverable error removes the database and its sidecars before the one
// retry; anything else leaves the files alone.
func TestOpenWithPolicy_RebuildOnCorruption(t *testing.T) {
	boom := errors.New("boom")
	stillBroken := errors.New("still broken")

	tests := []struct {
		name             string
		openErrs         []error
		wantErr          error
		wantOpenCalls    int
		wantFilesRemoved bool
	}{
		{
			name:          "succeeds on first try",
			openErrs:      []error{nil},
			wantOpenCalls: 1,
		},
		{
			name:          "non-recoverable error returns immediately",
			openErrs:      []error{boom},
			wantErr:       boom,
			wantOpenCalls: 1,
		},
		{
			name:             "recoverable error removes files and retries once",
			openErrs:         []error{sqliteErrorWithCode(11), nil},
			wantOpenCalls:    2,
			wantFilesRemoved: true,
		},
		{
			name:             "recoverable error then second failure returns that error",
			openErrs:         []error{sqliteErrorWithCode(11), stillBroken},
			wantErr:          stillBroken,
			wantOpenCalls:    2,
			wantFilesRemoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "corrupt.db")
			paths := sqliteFilePaths(dbPath)
			// An empty main file is a valid database, so the read-only probe
			// passes and the scripted open results drive the policy.
			writeTestFile(t, dbPath, "")
			for _, path := range paths[1:] {
				writeTestFile(t, path, "x")
			}

			calls := 0
			err := OpenWithPolicy(dbPath, RebuildOnCorruption, func() error {
				calls++
				if calls == 2 {
					for _, path := range paths {
						if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
							t.Errorf("expected %q to be removed before the retry, err=%v", path, statErr)
						}
					}
				}
				if calls > len(tt.openErrs) {
					t.Fatalf("open called %d times, more than the %d scripted results", calls, len(tt.openErrs))
				}
				return tt.openErrs[calls-1]
			})

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("OpenWithPolicy error = %v, want %v", err, tt.wantErr)
			}
			if calls != tt.wantOpenCalls {
				t.Fatalf("open called %d times, want %d", calls, tt.wantOpenCalls)
			}
			for _, path := range paths {
				_, statErr := os.Stat(path)
				if tt.wantFilesRemoved && !os.IsNotExist(statErr) {
					t.Fatalf("expected %q to have been removed, err=%v", path, statErr)
				}
				if !tt.wantFilesRemoved && statErr != nil {
					t.Fatalf("expected %q to be left alone, err=%v", path, statErr)
				}
			}
		})
	}
}

// TestRebuildOnCorruption_RemovalFails_DoesNotRetry pins the retry=true
// guarantee (ADR-0013): if a file cannot be removed, the policy must not ask
// for a retry that would open next to a leftover sidecar.
func TestRebuildOnCorruption_RemovalFails_DoesNotRetry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	writeTestFile(t, dbPath, "") // passes the probe; the scripted open fails
	blockRemoval(t, dbPath+"-wal")
	corrupt := sqliteErrorWithCode(11)

	calls := 0
	err := OpenWithPolicy(dbPath, RebuildOnCorruption, func() error {
		calls++
		return corrupt
	})

	if calls != 1 {
		t.Fatalf("open called %d times, want 1", calls)
	}
	if !errors.Is(err, corrupt) {
		t.Fatalf("OpenWithPolicy error = %v, want it to wrap the original %v", err, corrupt)
	}
}

// TestFailOnCorruption covers the policy for users.db (ADR-0013): no retry,
// no file touched, and a typed error that keeps the SQLite error.
func TestFailOnCorruption(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "users.db")
	original := map[string]string{dbPath: "not a database", dbPath + "-wal": "wal bytes"}
	for path, content := range original {
		writeTestFile(t, path, content)
	}
	openCalls := 0
	err := OpenWithPolicy(dbPath, FailOnCorruption, func() error {
		openCalls++
		return nil
	})

	// The read-only probe already reports SQLITE_NOTADB, so open never runs.
	if openCalls != 0 {
		t.Fatalf("open called %d times, want 0", openCalls)
	}
	var corrupt *CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		t.Fatalf("OpenWithPolicy error = %v (%T), want *CorruptDatabaseError", err, err)
	}
	if corrupt.Path != dbPath {
		t.Errorf("Path = %q, want %q", corrupt.Path, dbPath)
	}
	if !IsSQLiteRecoverableError(err) {
		t.Errorf("error %v does not wrap the SQLite error", err)
	}
	if len(corrupt.Stranded) != 0 {
		t.Errorf("Stranded = %v, want none", corrupt.Stranded)
	}
	if got := corrupt.SQLiteCode(); got != 26 {
		t.Errorf("SQLiteCode() = %d, want 26", got)
	}
	for _, part := range []string{dbPath, "26"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("Error() = %q, want it to contain %q", err.Error(), part)
		}
	}
	for path, content := range original {
		assertFileContent(t, path, content)
	}
}

// TestQuarantineOnCorruption covers the move-aside policy for real state
// (ADR-0013): the bytes of the database and every sidecar survive under the
// quarantine name, and a failed move leaves everything where it was and
// does not retry.
func TestQuarantineOnCorruption(t *testing.T) {
	const stamp = "20261017T143005Z"
	failSecondRename := func(calls *int) func(string, string) error {
		return func(from, to string) error {
			*calls++
			if *calls == 2 {
				return errors.New("rename blocked")
			}
			return os.Rename(from, to)
		}
	}

	tests := []struct {
		name          string
		sidecars      []string // sidecar suffixes present next to the db
		existingQuar  bool     // a quarantined file with the same timestamp already exists
		quarSuffix    string   // which quarantined file exists ("" = the db itself)
		failRename    bool     // the second rename fails
		wantQuarantin bool     // files moved and retry requested
	}{
		{name: "moves the db and every sidecar", sidecars: []string{"-journal", "-wal", "-shm"}, wantQuarantin: true},
		{name: "moves only the sidecars that exist", sidecars: []string{"-wal"}, wantQuarantin: true},
		{name: "db without sidecars", wantQuarantin: true},
		{name: "failed rename moves back and does not retry", sidecars: []string{"-wal", "-shm"}, failRename: true},
		{name: "never overwrites an earlier quarantine", sidecars: []string{"-wal"}, existingQuar: true},
		{name: "never overwrites an earlier quarantined sidecar", sidecars: []string{"-wal"}, existingQuar: true, quarSuffix: "-wal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNow(t, time.Date(2026, 10, 17, 16, 30, 5, 0, time.FixedZone("CEST", 2*3600)))
			renameCalls := 0
			if tt.failRename {
				setRename(t, failSecondRename(&renameCalls))
			}
			dbPath := filepath.Join(t.TempDir(), "favorites.db")
			quarantined := dbPath + ".corrupt-" + stamp
			original := map[string]string{dbPath: "not a database"}
			for _, suffix := range tt.sidecars {
				original[dbPath+suffix] = "sidecar " + suffix
			}
			for path, content := range original {
				writeTestFile(t, path, content)
			}
			if tt.existingQuar {
				writeTestFile(t, quarantined+tt.quarSuffix, "earlier quarantine")
			}
			corrupt := sqliteErrorWithCode(26)

			retry, err := QuarantineOnCorruption(dbPath, corrupt)

			if retry != tt.wantQuarantin {
				t.Fatalf("QuarantineOnCorruption retry = %v, want %v (err %v)", retry, tt.wantQuarantin, err)
			}
			if !tt.wantQuarantin {
				if !errors.Is(err, corrupt) {
					t.Fatalf("QuarantineOnCorruption error = %v, want it to wrap the original %v", err, corrupt)
				}
				for path, content := range original {
					assertFileContent(t, path, content)
				}
				if tt.existingQuar {
					assertFileContent(t, quarantined+tt.quarSuffix, "earlier quarantine")
					if tt.quarSuffix != "" {
						assertMissing(t, quarantined)
					}
				} else {
					assertMissing(t, quarantined)
				}
				return
			}

			if err != nil {
				t.Fatalf("QuarantineOnCorruption: unexpected error %v", err)
			}
			for path, content := range original {
				assertMissing(t, path)
				assertFileContent(t, quarantined+strings.TrimPrefix(path, dbPath), content)
			}
			for _, suffix := range []string{"-journal", "-wal", "-shm"} {
				if _, ok := original[dbPath+suffix]; !ok {
					assertMissing(t, quarantined+suffix)
				}
			}
		})
	}
}

// TestQuarantineOnCorruption_FailedMoveBack_KeepsEveryByte covers the worst
// case: the move aside and the move back both fail. Nothing is deleted, the
// error names both failures, and there is no retry.
func TestQuarantineOnCorruption_FailedMoveBack_KeepsEveryByte(t *testing.T) {
	setNow(t, time.Date(2026, 10, 17, 14, 30, 5, 0, time.UTC))
	calls := 0
	setRename(t, func(from, to string) error {
		calls++
		if calls >= 2 {
			return fmt.Errorf("rename %d blocked", calls)
		}
		return os.Rename(from, to)
	})
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	writeTestFile(t, dbPath, "db bytes")
	writeTestFile(t, dbPath+"-wal", "wal bytes")

	retry, err := QuarantineOnCorruption(dbPath, sqliteErrorWithCode(11))

	if retry {
		t.Fatal("QuarantineOnCorruption asked for a retry after a failed move")
	}
	if err == nil || !strings.Contains(err.Error(), "rename 2 blocked") || !strings.Contains(err.Error(), "rename 3 blocked") {
		t.Fatalf("error = %v, want it to name the failed move and the failed move back", err)
	}
	var corrupt *CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		t.Fatalf("error = %v (%T), want *CorruptDatabaseError", err, err)
	}
	want := []string{dbPath + ".corrupt-20261017T143005Z"}
	if !slices.Equal(corrupt.Stranded, want) {
		t.Fatalf("Stranded = %v, want %v", corrupt.Stranded, want)
	}
	if !strings.Contains(err.Error(), want[0]) {
		t.Errorf("Error() = %q, want it to name the stranded file", err.Error())
	}
	assertFileContent(t, dbPath+".corrupt-20261017T143005Z", "db bytes")
	assertFileContent(t, dbPath+"-wal", "wal bytes")
}

// TestProbeReadOnly runs the probe against real files, with absolute and
// relative paths: the default --data-dir is the relative ./data, and a
// relative path the probe cannot open would let the read-write open discard
// the -wal (ADR-0013).
func TestProbeReadOnly(t *testing.T) {
	files := []struct {
		name            string
		setup           func(t *testing.T, path string)
		wantRecoverable bool
	}{
		{name: "missing file", setup: func(*testing.T, string) {}},
		{name: "0-byte file", setup: func(t *testing.T, path string) { writeTestFile(t, path, "") }},
		{name: "healthy database", setup: func(t *testing.T, path string) {
			if err := openReadWrite(t, path); err != nil {
				t.Fatalf("create database: %v", err)
			}
		}},
		{name: "not a database", setup: func(t *testing.T, path string) {
			writeTestFile(t, path, "this is not a sqlite database, just bytes!!")
		}, wantRecoverable: true},
	}

	for _, relative := range []bool{false, true} {
		for _, f := range files {
			name := "absolute/" + f.name
			if relative {
				name = "relative/" + f.name
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, "probe.db")
				if relative {
					t.Chdir(dir)
					dbPath = filepath.Join(".", "probe.db")
				}
				f.setup(t, dbPath)

				err := probeReadOnly(dbPath)

				if got := IsSQLiteRecoverableError(err); got != f.wantRecoverable {
					t.Fatalf("probeReadOnly(%q) = %v, recoverable %v, want recoverable %v", dbPath, err, got, f.wantRecoverable)
				}
				if !f.wantRecoverable && err != nil {
					t.Fatalf("probeReadOnly(%q): unexpected error %v", dbPath, err)
				}
			})
		}
	}
}

// TestOpenWithPolicy_ProbeCannotRead_FailsClosed pins that an existing file
// the probe cannot read for a non-recoverable reason stops the open instead
// of falling through to a read-write open that could discard the -wal.
func TestOpenWithPolicy_ProbeCannotRead_FailsClosed(t *testing.T) {
	// An unreadable file exists and is non-empty, but SQLite cannot open it:
	// SQLITE_CANTOPEN, which is not recoverable.
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX file permissions that the current user cannot bypass")
	}
	dbPath := filepath.Join(t.TempDir(), "unreadable.db")
	writeTestFile(t, dbPath, "this is not a sqlite database, just bytes!!")
	if err := os.Chmod(dbPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dbPath, 0o644) })

	policyCalls, openCalls := 0, 0
	err := OpenWithPolicy(dbPath, func(string, error) (bool, error) {
		policyCalls++
		return true, nil
	}, func() error {
		openCalls++
		return nil
	})

	if err == nil {
		t.Fatal("OpenWithPolicy: expected the probe error, got nil")
	}
	if policyCalls != 0 || openCalls != 0 {
		t.Fatalf("policy called %d times, open called %d times, want neither", policyCalls, openCalls)
	}
}

func TestNeedsWriteToRecover(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "SQLITE_READONLY_ROLLBACK (hot journal)", err: sqliteErrorWithCode(8 | (3 << 8)), want: true},
		{name: "SQLITE_READONLY_RECOVERY (WAL needs recovery)", err: sqliteErrorWithCode(8 | (1 << 8)), want: true},
		{name: "SQLITE_NOTADB", err: sqliteErrorWithCode(26)},
		{name: "SQLITE_CANTOPEN", err: sqliteErrorWithCode(14)},
		{name: "non-sqlite error", err: errors.New("boom")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsWriteToRecover(tt.err); got != tt.want {
				t.Fatalf("needsWriteToRecover(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestStripLongPathPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "drive path", in: `\\?\C:\data\users.db`, want: `C:\data\users.db`},
		{name: "UNC path", in: `\\?\UNC\server\share\users.db`, want: `\\server\share\users.db`},
		{name: "plain path is unchanged", in: `C:\data\users.db`, want: `C:\data\users.db`},
		{name: "POSIX path is unchanged", in: "/srv/data/users.db", want: "/srv/data/users.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripLongPathPrefix(tt.in); got != tt.want {
				t.Fatalf("stripLongPathPrefix(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestOpenWithPolicy_ProbesBeforeOpen pins why OpenWithPolicy looks at an
// existing file read-only before open runs: a read-write connection to a
// database whose header SQLite cannot read discards the -wal, even when it
// holds committed transactions, before any policy could keep it (ADR-0013).
func TestOpenWithPolicy_ProbesBeforeOpen(t *testing.T) {
	tests := []struct {
		name          string
		page1InWAL    bool // the WAL holds page 1, so SQLite can still read the database
		breakHeader   bool
		wantPolicy    bool
		wantOpenCalls int
	}{
		{name: "healthy database with a WAL opens normally", wantOpenCalls: 1},
		{name: "broken header with a committed WAL reaches the policy before open", breakHeader: true, wantPolicy: true},
		{name: "broken header that the WAL repairs is left to SQLite", page1InWAL: true, breakHeader: true, wantOpenCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := writeDatabaseWithCommittedWAL(t, tt.page1InWAL)
			if tt.breakHeader {
				raw, err := os.ReadFile(dbPath)
				if err != nil {
					t.Fatalf("read %q: %v", dbPath, err)
				}
				copy(raw, "not a sqlite header!")
				writeTestFile(t, dbPath, string(raw))
			}
			wal, err := os.ReadFile(dbPath + "-wal")
			if err != nil {
				t.Fatalf("read -wal: %v", err)
			}
			typed := errors.New("policy refused")

			policyCalls, openCalls := 0, 0
			err = OpenWithPolicy(dbPath, func(string, error) (bool, error) {
				policyCalls++
				return false, typed
			}, func() error {
				openCalls++
				return openReadWrite(t, dbPath)
			})

			if gotPolicy := policyCalls == 1; gotPolicy != tt.wantPolicy {
				t.Fatalf("policy called %d times, want called=%v", policyCalls, tt.wantPolicy)
			}
			if openCalls != tt.wantOpenCalls {
				t.Fatalf("open called %d times, want %d", openCalls, tt.wantOpenCalls)
			}
			if tt.wantPolicy {
				if err != typed {
					t.Fatalf("OpenWithPolicy error = %v, want the policy error", err)
				}
				assertFileContent(t, dbPath+"-wal", string(wal))
			} else if err != nil {
				t.Fatalf("OpenWithPolicy: unexpected error %v", err)
			}
		})
	}
}

func TestSQLiteCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "extended code is kept", err: sqliteErrorWithCode(10 | (12 << 8)), want: 10 | (12 << 8)},
		{name: "wrapped sqlite error", err: fmt.Errorf("open: %w", sqliteErrorWithCode(26)), want: 26},
		{name: "non-sqlite error", err: errors.New("boom"), want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sqliteCode(tt.err); got != tt.want {
				t.Fatalf("sqliteCode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func sqliteFilePaths(dbPath string) []string {
	return []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"}
}

// writeDatabaseWithCommittedWAL returns the path of a copy of a WAL-mode
// database taken while its last transaction is still only in the -wal.
// Without page1InWAL the schema is checkpointed first, so the WAL holds a
// data page but not page 1.
func writeDatabaseWithCommittedWAL(t *testing.T, page1InWAL bool) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src.db")
	db, err := sql.Open("sqlite", src+"?_pragma=journal_mode(WAL)&_pragma=wal_autocheckpoint(0)")
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	stmts := []string{"CREATE TABLE t(x)", "INSERT INTO t VALUES (1)"}
	if !page1InWAL {
		stmts = append(stmts, "PRAGMA wal_checkpoint(TRUNCATE)")
	}
	stmts = append(stmts, "INSERT INTO t VALUES (2)")
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	dbPath := filepath.Join(t.TempDir(), "copy.db")
	for _, suffix := range []string{"", "-wal"} {
		raw, err := os.ReadFile(src + suffix)
		if err != nil {
			t.Fatalf("read %q: %v", src+suffix, err)
		}
		writeTestFile(t, dbPath+suffix, string(raw))
	}
	return dbPath
}

// openReadWrite opens dbPath the way the stores do and closes it again.
func openReadWrite(t *testing.T, dbPath string) error {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS other(x)")
	return err
}

func setNow(t *testing.T, at time.Time) {
	t.Helper()
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
}

func setRename(t *testing.T, fn func(string, string) error) {
	t.Helper()
	old := rename
	rename = fn
	t.Cleanup(func() { rename = old })
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create %q: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%q = %q, want %q", path, got, want)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %q not to exist, err=%v", path, err)
	}
}

// blockRemoval puts a non-empty directory at path, so os.Remove and os.Rename
// onto or from it fail portably without permission tricks.
func blockRemoval(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "blocker"), 0o755); err != nil {
		t.Fatalf("failed to create blocking directory %q: %v", path, err)
	}
}

func sqliteErrorWithCode(code int) error {
	e := &sqlite.Error{}
	// modernc.org/sqlite does not expose a public constructor for sqlite.Error,
	// so tests set the private `code` field directly to synthesize specific
	// result codes.
	v := reflect.ValueOf(e).Elem().FieldByName("code")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetInt(int64(code))
	return e
}

// TestQuarantineOnCorruption_SecondCorruptionAfterRestart runs the real
// OpenWithPolicy twice on the same path, as two restarts would: the first
// quarantine must survive the second one. Two corruptions within the same
// second must not overwrite the first copy; the second open fails instead and
// leaves the new bytes in place (ADR-0013).
func TestQuarantineOnCorruption_SecondCorruptionAfterRestart(t *testing.T) {
	first := time.Date(2026, 10, 17, 14, 30, 5, 0, time.UTC)
	tests := []struct {
		name       string
		second     time.Time
		wantSecond bool // the second open quarantines and succeeds
	}{
		{name: "later restart keeps both copies", second: first.Add(90 * time.Minute), wantSecond: true},
		{name: "same second refuses to overwrite the first copy", second: first},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "sessions.db")
			open := func() error { return openReadWrite(t, dbPath) }

			writeTestFile(t, dbPath, "first corruption")
			writeTestFile(t, dbPath+"-wal", "first wal")
			setNow(t, first)
			if err := OpenWithPolicy(dbPath, QuarantineOnCorruption, open); err != nil {
				t.Fatalf("first OpenWithPolicy: %v", err)
			}
			firstCopy := dbPath + ".corrupt-" + first.Format(quarantineTimeFormat)

			writeTestFile(t, dbPath, "second corruption")
			writeTestFile(t, dbPath+"-wal", "second wal")
			setNow(t, tt.second)
			err := OpenWithPolicy(dbPath, QuarantineOnCorruption, open)

			assertFileContent(t, firstCopy, "first corruption")
			assertFileContent(t, firstCopy+"-wal", "first wal")
			if !tt.wantSecond {
				var corrupt *CorruptDatabaseError
				if !errors.As(err, &corrupt) {
					t.Fatalf("second OpenWithPolicy error = %v (%T), want *CorruptDatabaseError", err, err)
				}
				if len(corrupt.Stranded) != 0 {
					t.Fatalf("Stranded = %v, want none", corrupt.Stranded)
				}
				assertFileContent(t, dbPath, "second corruption")
				assertFileContent(t, dbPath+"-wal", "second wal")
				return
			}
			if err != nil {
				t.Fatalf("second OpenWithPolicy: %v", err)
			}
			secondCopy := dbPath + ".corrupt-" + tt.second.Format(quarantineTimeFormat)
			assertFileContent(t, secondCopy, "second corruption")
			assertFileContent(t, secondCopy+"-wal", "second wal")
			if err := probeReadOnly(dbPath); err != nil {
				t.Fatalf("database after the second quarantine is not healthy: %v", err)
			}
		})
	}
}

// TestOpenWithPolicy_ReadOnlyDataDir_TouchesNothing runs every policy against
// a corrupt database in a directory where nothing can be renamed or removed:
// the open must fail and every byte must stay in place. With a -wal present
// the read-only probe cannot create the -shm and fails closed before any
// policy runs; without one the policy runs and its rename or remove fails.
func TestOpenWithPolicy_ReadOnlyDataDir_TouchesNothing(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX directory permissions that the current user cannot bypass")
	}
	tests := []struct {
		name        string
		policy      CorruptionPolicy
		withWAL     bool
		wantCorrupt bool // error is a *CorruptDatabaseError
	}{
		{name: "quarantine without sidecar", policy: QuarantineOnCorruption, wantCorrupt: true},
		{name: "fail without sidecar", policy: FailOnCorruption, wantCorrupt: true},
		{name: "rebuild without sidecar", policy: RebuildOnCorruption},
		{name: "quarantine with -wal fails closed in the probe", policy: QuarantineOnCorruption, withWAL: true},
		{name: "fail with -wal fails closed in the probe", policy: FailOnCorruption, withWAL: true},
		{name: "rebuild with -wal fails closed in the probe", policy: RebuildOnCorruption, withWAL: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "favorites.db")
			files := map[string]string{dbPath: "this is not a sqlite database, just bytes!!"}
			if tt.withWAL {
				files[dbPath+"-wal"] = "wal bytes"
			}
			for path, content := range files {
				writeTestFile(t, path, content)
			}
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

			err := OpenWithPolicy(dbPath, tt.policy, func() error { return openReadWrite(t, dbPath) })

			if err == nil {
				t.Fatal("OpenWithPolicy: expected an error in a read-only directory, got nil")
			}
			var corrupt *CorruptDatabaseError
			if got := errors.As(err, &corrupt); got != tt.wantCorrupt {
				t.Fatalf("error = %v (%T), *CorruptDatabaseError = %v, want %v", err, err, got, tt.wantCorrupt)
			}
			if corrupt != nil && len(corrupt.Stranded) != 0 {
				t.Fatalf("Stranded = %v, want none", corrupt.Stranded)
			}
			for path, content := range files {
				assertFileContent(t, path, content)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read dir: %v", err)
			}
			if len(entries) != len(files) {
				t.Fatalf("directory holds %d entries, want %d (nothing created or moved)", len(entries), len(files))
			}
		})
	}
}

// TestOpenWithPolicy_HealthyDatabaseWithWAL_KeepsData pins that the probe
// never costs a healthy database the committed rows in its -wal, whether
// another connection still holds it (live server, restore) or it was left by
// a crash, with an absolute and a relative path (the default ./data).
func TestOpenWithPolicy_HealthyDatabaseWithWAL_KeepsData(t *testing.T) {
	sources := []struct {
		name  string
		setup func(t *testing.T) string // returns the absolute db path
	}{
		{name: "WAL left by a crash", setup: func(t *testing.T) string {
			return writeDatabaseWithCommittedWAL(t, false)
		}},
		{name: "WAL held by a live writer", setup: func(t *testing.T) string {
			dbPath := filepath.Join(t.TempDir(), "live.db")
			db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=wal_autocheckpoint(0)")
			if err != nil {
				t.Fatalf("open writer: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			db.SetMaxOpenConns(1)
			for _, stmt := range []string{"CREATE TABLE t(x)", "INSERT INTO t VALUES (1)", "INSERT INTO t VALUES (2)"} {
				if _, err := db.Exec(stmt); err != nil {
					t.Fatalf("%s: %v", stmt, err)
				}
			}
			if info, err := os.Stat(dbPath + "-wal"); err != nil || info.Size() == 0 {
				t.Fatalf("expected a non-empty -wal, info=%v err=%v", info, err)
			}
			return dbPath
		}},
	}

	for _, relative := range []bool{false, true} {
		for _, src := range sources {
			name := "absolute/" + src.name
			if relative {
				name = "relative/" + src.name
			}
			t.Run(name, func(t *testing.T) {
				dbPath := src.setup(t)
				if relative {
					dir := filepath.Dir(dbPath)
					t.Chdir(filepath.Dir(dir))
					dbPath = "./" + filepath.Base(dir) + "/" + filepath.Base(dbPath)
				}

				var rows int
				err := OpenWithPolicy(dbPath, func(_ string, err error) (bool, error) {
					t.Errorf("policy called for a healthy database: %v", err)
					return false, err
				}, func() error {
					db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
					if err != nil {
						return err
					}
					defer func() { _ = db.Close() }()
					return db.QueryRow("SELECT count(*) FROM t").Scan(&rows)
				})

				if err != nil {
					t.Fatalf("OpenWithPolicy: %v", err)
				}
				if rows != 2 {
					t.Fatalf("rows = %d, want 2 (the committed row in the -wal was lost)", rows)
				}
			})
		}
	}
}

// TestProbeReadOnly_SpecialCharactersInPath pins that the read-only URI
// escapes the data path: a mis-escaped space, '#', '?' or '%' would make the
// probe miss a healthy file (startup fails closed) or open a different one.
func TestProbeReadOnly_SpecialCharactersInPath(t *testing.T) {
	dirs := []string{"with space", "hash#dir", "pct%41dir", "pct%dir", "ümlaut ✓"}
	if runtime.GOOS != "windows" {
		dirs = append(dirs, "q?mark", "amp&mode=rw")
	}

	for _, d := range dirs {
		for _, healthy := range []bool{true, false} {
			name := d + "/corrupt"
			if healthy {
				name = d + "/healthy"
			}
			t.Run(name, func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), d)
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				dbPath := filepath.Join(dir, "users.db")
				if healthy {
					// Create through a URI so the setup itself is not
					// tripped by '?' in the plain path.
					dsn, err := readOnlyDSN(dbPath)
					if err != nil {
						t.Fatalf("readOnlyDSN: %v", err)
					}
					db, err := sql.Open("sqlite", strings.Replace(dsn, "mode=ro", "mode=rwc", 1))
					if err != nil {
						t.Fatalf("open: %v", err)
					}
					if _, err := db.Exec("CREATE TABLE t(x)"); err != nil {
						t.Fatalf("create: %v", err)
					}
					_ = db.Close()
					if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
						t.Fatalf("setup did not create %q: info=%v err=%v", dbPath, info, err)
					}
				} else {
					writeTestFile(t, dbPath, "this is not a sqlite database, just bytes!!")
					writeTestFile(t, dbPath+"-wal", "wal bytes")
				}

				err := probeReadOnly(dbPath)

				if healthy {
					if err != nil {
						t.Fatalf("probeReadOnly(%q) on a healthy database: %v", dbPath, err)
					}
					return
				}
				if !IsSQLiteRecoverableError(err) {
					t.Fatalf("probeReadOnly(%q) = %v, want a recoverable error", dbPath, err)
				}
				err = OpenWithPolicy(dbPath, FailOnCorruption, func() error {
					t.Error("open ran although the probe found corruption")
					return nil
				})
				var corrupt *CorruptDatabaseError
				if !errors.As(err, &corrupt) || corrupt.Path != dbPath {
					t.Fatalf("OpenWithPolicy error = %v, want *CorruptDatabaseError for %q", err, dbPath)
				}
				assertFileContent(t, dbPath, "this is not a sqlite database, just bytes!!")
				assertFileContent(t, dbPath+"-wal", "wal bytes")
			})
		}
	}
}
