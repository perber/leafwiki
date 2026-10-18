package test_utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corruptSQLiteBytes is 43 bytes that SQLite rejects with SQLITE_NOTADB.
const corruptSQLiteBytes = "this is not a sqlite database, just bytes!!"

// WriteCorruptSQLiteDB writes an unreadable database at dbPath plus a fake
// -wal sidecar and returns the content of both by path, so a test can check
// that a corruption policy kept every byte (ADR-0013).
func WriteCorruptSQLiteDB(t *testing.T, dbPath string) map[string]string {
	t.Helper()
	files := map[string]string{
		dbPath:          corruptSQLiteBytes,
		dbPath + "-wal": "fake wal sidecar that may hold committed pages",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %q: %v", path, err)
		}
	}
	return files
}

// AssertFilesUnchanged fails unless every path still holds its content.
func AssertFilesUnchanged(t *testing.T, files map[string]string) {
	t.Helper()
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %q to be left untouched: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%q changed: got %q, want %q", path, got, want)
		}
	}
}

// AssertSQLiteQuarantined fails unless the database at dbPath was moved to
// exactly one "<name>.corrupt-<timestamp>" copy that keeps the original
// bytes of the database and each sidecar in files (as returned by
// WriteCorruptSQLiteDB). It returns the quarantined database path.
func AssertSQLiteQuarantined(t *testing.T, dbPath string, files map[string]string) string {
	t.Helper()
	matches, err := filepath.Glob(dbPath + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var quarantined []string
	for _, m := range matches {
		if !strings.HasSuffix(m, "-wal") && !strings.HasSuffix(m, "-shm") && !strings.HasSuffix(m, "-journal") {
			quarantined = append(quarantined, m)
		}
	}
	if len(quarantined) != 1 {
		t.Fatalf("expected exactly one quarantined copy of %q, got %v", dbPath, matches)
	}
	moved := make(map[string]string, len(files))
	for path, content := range files {
		moved[quarantined[0]+strings.TrimPrefix(path, dbPath)] = content
	}
	AssertFilesUnchanged(t, moved)
	return quarantined[0]
}

// AssertNoSQLiteQuarantine fails if any quarantined copy of dbPath exists.
func AssertNoSQLiteQuarantine(t *testing.T, dbPath string) {
	t.Helper()
	matches, err := filepath.Glob(dbPath + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no quarantined copy of %q, got %v", dbPath, matches)
	}
}
