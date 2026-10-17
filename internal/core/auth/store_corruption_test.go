package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/test_utils"
)

// Regression test for the startup path that replaced an unreadable users.db
// with an empty one, after which InitDefaultAdmin created a fresh admin from
// the startup flags (ADR-0013). users.db must fail and keep every byte.
func TestNewUserStore_CorruptFile_FailsAndPreservesOriginal(t *testing.T) {
	dir := t.TempDir()
	files := test_utils.WriteCorruptSQLiteDB(t, filepath.Join(dir, "users.db"))

	store, err := NewUserStore(dir)
	if err == nil {
		_ = store.Close()
		t.Fatal("NewUserStore: expected an error for a corrupt users.db, got nil")
	}

	var corrupt *sqliteutil.CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		t.Fatalf("NewUserStore error = %v (%T), want a *sqliteutil.CorruptDatabaseError", err, err)
	}
	if corrupt.Path != filepath.Join(dir, "users.db") {
		t.Errorf("CorruptDatabaseError.Path = %q, want %q", corrupt.Path, filepath.Join(dir, "users.db"))
	}
	test_utils.AssertFilesUnchanged(t, files)
	test_utils.AssertNoSQLiteQuarantine(t, filepath.Join(dir, "users.db"))
}

// Sessions and email tokens are short-lived: a corrupt file is moved aside
// with every sidecar and the store starts empty instead of keeping the wiki
// offline (ADR-0013).
func TestShortLivedAuthStores_CorruptFile_QuarantinesAndContinues(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		// openAndUse opens the store in dir, proves it works and closes it.
		openAndUse func(t *testing.T, dir string)
	}{
		{
			name:     "sessions",
			filename: "sessions.db",
			openAndUse: func(t *testing.T, dir string) {
				store, err := NewSessionStore(dir)
				if err != nil {
					t.Fatalf("NewSessionStore: %v", err)
				}
				defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
				if err := store.CreateSession("s1", "u1", "refresh", time.Now().Add(time.Hour)); err != nil {
					t.Fatalf("CreateSession after quarantine: %v", err)
				}
			},
		},
		{
			name:     "email tokens",
			filename: "email_tokens.db",
			openAndUse: func(t *testing.T, dir string) {
				store, err := NewEmailTokenStore(dir)
				if err != nil {
					t.Fatalf("NewEmailTokenStore: %v", err)
				}
				defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
				if _, err := store.Issue("u1", PurposePasswordReset, time.Hour); err != nil {
					t.Fatalf("Issue after quarantine: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, tt.filename)
			files := test_utils.WriteCorruptSQLiteDB(t, dbPath)

			tt.openAndUse(t, dir)

			test_utils.AssertSQLiteQuarantined(t, dbPath, files)
		})
	}
}

// The default --data-dir is the relative ./data. The read-only probe must
// resolve such a path itself; otherwise it cannot open the file, the
// read-write open runs and SQLite discards the -wal before any policy could
// keep it (ADR-0013).
func TestRealStateStores_RelativeDataDir_KeepWAL(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		open           func(dir string) (func() error, error)
		wantQuarantine bool
	}{
		{name: "users fails", filename: "users.db", open: func(dir string) (func() error, error) {
			s, err := NewUserStore(dir)
			if err != nil {
				return nil, err
			}
			return s.Close, nil
		}},
		{name: "sessions quarantine", filename: "sessions.db", wantQuarantine: true, open: func(dir string) (func() error, error) {
			s, err := NewSessionStore(dir)
			if err != nil {
				return nil, err
			}
			return s.Close, nil
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.Mkdir("data", 0o755); err != nil {
				t.Fatalf("mkdir data: %v", err)
			}
			dbPath := filepath.Join("data", tt.filename)
			files := test_utils.WriteCorruptSQLiteDB(t, dbPath)

			closeStore, err := tt.open("./data")

			if !tt.wantQuarantine {
				var corrupt *sqliteutil.CorruptDatabaseError
				if !errors.As(err, &corrupt) {
					t.Fatalf("open error = %v, want *sqliteutil.CorruptDatabaseError", err)
				}
				test_utils.AssertFilesUnchanged(t, files)
				return
			}
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			test_utils.WrapCloseWithErrorCheck(closeStore, t)
			test_utils.AssertSQLiteQuarantined(t, dbPath, files)
		})
	}
}

// A missing file and a 0-byte file are both valid empty databases, not
// corruption: no store may fail or quarantine on them.
func TestAuthStores_MissingOrEmptyFile_OpensWithoutQuarantine(t *testing.T) {
	stores := []struct {
		name     string
		filename string
		open     func(dir string) (func() error, error)
	}{
		{name: "users", filename: "users.db", open: func(dir string) (func() error, error) {
			s, err := NewUserStore(dir)
			if err != nil {
				return nil, err
			}
			return s.Close, nil
		}},
		{name: "sessions", filename: "sessions.db", open: func(dir string) (func() error, error) {
			s, err := NewSessionStore(dir)
			if err != nil {
				return nil, err
			}
			return s.Close, nil
		}},
		{name: "email tokens", filename: "email_tokens.db", open: func(dir string) (func() error, error) {
			s, err := NewEmailTokenStore(dir)
			if err != nil {
				return nil, err
			}
			return s.Close, nil
		}},
	}
	files := []struct {
		name  string
		setup func(t *testing.T, dbPath string)
	}{
		{name: "missing file", setup: func(*testing.T, string) {}},
		{name: "0-byte file", setup: func(t *testing.T, dbPath string) {
			if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
				t.Fatalf("write %q: %v", dbPath, err)
			}
		}},
	}

	for _, s := range stores {
		for _, f := range files {
			t.Run(s.name+"/"+f.name, func(t *testing.T) {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, s.filename)
				f.setup(t, dbPath)

				closeStore, err := s.open(dir)
				if err != nil {
					t.Fatalf("open: %v", err)
				}
				test_utils.WrapCloseWithErrorCheck(closeStore, t)

				if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
					t.Fatalf("expected %q to hold a fresh database, info=%v err=%v", dbPath, info, err)
				}
				test_utils.AssertNoSQLiteQuarantine(t, dbPath)
			})
		}
	}
}

// A healthy users.db in a data dir whose name needs URI escaping must reopen
// with its users: the read-only probe builds a URI while the store opens a
// plain path, and a mismatch would fail startup closed on a healthy file.
func TestNewUserStore_SpecialCharactersInDataDir_KeepsUsers(t *testing.T) {
	dirs := []string{"with space", "hash#dir", "pct%41dir", "pct%dir", "ümlaut ✓"}
	for _, d := range dirs {
		t.Run(d, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), d)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			first, err := NewUserStore(dir)
			if err != nil {
				t.Fatalf("first NewUserStore: %v", err)
			}
			if err := first.CreateUser(&User{ID: "u1", Username: "alice", Email: "alice@example.com", Password: "pw", Role: RoleEditor}); err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			test_utils.WrapCloseWithErrorCheck(first.Close, t)

			second, err := NewUserStore(dir)
			if err != nil {
				t.Fatalf("second NewUserStore on a healthy users.db: %v", err)
			}
			defer test_utils.WrapCloseWithErrorCheck(second.Close, t)
			if _, err := second.GetUserByUsername("alice"); err != nil {
				t.Fatalf("user lost after reopen: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "users.db")); err != nil {
				t.Fatalf("users.db not at the expected path: %v", err)
			}
			test_utils.AssertNoSQLiteQuarantine(t, filepath.Join(dir, "users.db"))
		})
	}
}
