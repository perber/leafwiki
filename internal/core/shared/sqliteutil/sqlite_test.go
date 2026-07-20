package sqliteutil

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

	RemoveSQLiteFiles(dbPath)

	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to be removed, err=%v", path, err)
		}
	}
}

func TestRemoveSQLiteFiles_NoOpWhenMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "missing.db")

	RemoveSQLiteFiles(dbPath)

	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to remain absent, err=%v", path, err)
		}
	}
}

func TestRetryOnCorruption_SucceedsOnFirstTry(t *testing.T) {
	calls := 0
	err := RetryOnCorruption("unused.db", func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("RetryOnCorruption: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected open to be called once, got %d", calls)
	}
}

func TestRetryOnCorruption_NonRecoverableError_ReturnsImmediately(t *testing.T) {
	calls := 0
	wantErr := errors.New("boom")
	err := RetryOnCorruption("unused.db", func() error {
		calls++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected open to be called once (no retry), got %d", calls)
	}
}

func TestRetryOnCorruption_RecoverableError_RetriesOnceAndRemovesFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "corrupt.db")
	paths := []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("failed to create %q: %v", path, err)
		}
	}

	calls := 0
	err := RetryOnCorruption(dbPath, func() error {
		calls++
		if calls == 1 {
			return sqliteErrorWithCode(11) // SQLITE_CORRUPT
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RetryOnCorruption: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected open to be called twice (initial + retry), got %d", calls)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %q to have been removed before retry, err=%v", path, err)
		}
	}
}

func TestRetryOnCorruption_RecoverableError_SecondFailureReturnsThatError(t *testing.T) {
	calls := 0
	secondErr := errors.New("still broken")
	err := RetryOnCorruption(t.TempDir()+"/corrupt.db", func() error {
		calls++
		if calls == 1 {
			return sqliteErrorWithCode(11) // SQLITE_CORRUPT
		}
		return secondErr
	})
	if !errors.Is(err, secondErr) {
		t.Fatalf("expected secondErr, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected open to be called twice, got %d", calls)
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
