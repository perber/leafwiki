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

func sqliteErrorWithCode(code int) error {
	e := &sqlite.Error{}
	v := reflect.ValueOf(e).Elem().FieldByName("code")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetInt(int64(code))
	return e
}
