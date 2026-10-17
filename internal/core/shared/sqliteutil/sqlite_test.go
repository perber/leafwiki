package sqliteutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "search.db")
	paths := []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"}

	for _, path := range paths {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("failed to create %q: %v", path, err)
		}
	}

	if err := RemoveSQLiteFiles(dbPath); err != nil {
		t.Fatalf("RemoveSQLiteFiles: %v", err)
	}

	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to be removed, err=%v", path, err)
		}
	}
}

func TestRemoveSQLiteFiles_NoOpWhenMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "missing.db")

	if err := RemoveSQLiteFiles(dbPath); err != nil {
		t.Fatalf("RemoveSQLiteFiles: %v", err)
	}

	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to remain absent, err=%v", path, err)
		}
	}
}

func TestRemoveSQLiteFiles_ReportsFailureAndRemovesTheRest(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "search.db")
	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-shm"} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("failed to create %q: %v", path, err)
		}
	}
	blockRemoval(t, dbPath+"-wal")

	if err := RemoveSQLiteFiles(dbPath); err == nil {
		t.Fatal("RemoveSQLiteFiles: expected an error for an undeletable -wal, got nil")
	}

	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-shm"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected %q to be removed despite the -wal failure, err=%v", path, err)
		}
	}
	if _, err := os.Stat(dbPath + "-wal"); err != nil {
		t.Errorf("expected the blocked -wal to remain, err=%v", err)
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
			for _, path := range paths {
				if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
					t.Fatalf("failed to create %q: %v", path, err)
				}
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
	if err := os.WriteFile(dbPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create %q: %v", dbPath, err)
	}
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
		existingQuar  bool     // a quarantined db with the same timestamp already exists
		failRename    bool     // the second rename fails
		wantQuarantin bool
		wantOpenCalls int
	}{
		{name: "moves the db and every sidecar", sidecars: []string{"-journal", "-wal", "-shm"}, wantQuarantin: true, wantOpenCalls: 2},
		{name: "moves only the sidecars that exist", sidecars: []string{"-wal"}, wantQuarantin: true, wantOpenCalls: 2},
		{name: "db without sidecars", wantQuarantin: true, wantOpenCalls: 2},
		{name: "failed rename moves back and does not retry", sidecars: []string{"-wal", "-shm"}, failRename: true, wantOpenCalls: 1},
		{name: "never overwrites an earlier quarantine", sidecars: []string{"-wal"}, existingQuar: true, wantOpenCalls: 1},
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
				writeTestFile(t, quarantined, "earlier quarantine")
			}
			corrupt := sqliteErrorWithCode(26)

			openCalls := 0
			err := OpenWithPolicy(dbPath, QuarantineOnCorruption, func() error {
				openCalls++
				if openCalls == 1 {
					return corrupt
				}
				return nil
			})

			if openCalls != tt.wantOpenCalls {
				t.Fatalf("open called %d times, want %d", openCalls, tt.wantOpenCalls)
			}
			if !tt.wantQuarantin {
				if !errors.Is(err, corrupt) {
					t.Fatalf("OpenWithPolicy error = %v, want it to wrap the original %v", err, corrupt)
				}
				for path, content := range original {
					assertFileContent(t, path, content)
				}
				if tt.existingQuar {
					assertFileContent(t, quarantined, "earlier quarantine")
				} else {
					assertMissing(t, quarantined)
				}
				return
			}

			if err != nil {
				t.Fatalf("OpenWithPolicy: unexpected error %v", err)
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
	assertFileContent(t, dbPath+".corrupt-20261017T143005Z", "db bytes")
	assertFileContent(t, dbPath+"-wal", "wal bytes")
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
