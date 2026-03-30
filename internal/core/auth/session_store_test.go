package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

func TestSessionStore_CreateAndValidateSession(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	expiresAt := time.Now().Add(time.Hour)
	if err := store.CreateSession("s1", "u1", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	active, err := store.IsActive("s1", "u1", "refresh", time.Now())
	if err != nil {
		t.Fatalf("IsActive err: %v", err)
	}
	if !active {
		t.Fatalf("expected session to be active")
	}
}

func TestSessionDatabasePath_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(sessionDatabasePath(`C:\wiki\data`, "sessions.db"), `\`, `/`)
	want := `C:/wiki/data/sessions.db`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestSessionStore_CreatesDatabaseInStorageDir(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSessionStore(tmp)
	if err != nil {
		t.Fatalf("NewSessionStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(tmp, "sessions.db")); err != nil {
		t.Fatalf("expected sessions.db in storage dir, got err: %v", err)
	}
}
