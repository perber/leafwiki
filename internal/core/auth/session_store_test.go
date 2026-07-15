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

func TestSessionStore_RevokeAllSessionsForUserExcept(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	expiresAt := time.Now().Add(time.Hour)
	if err := store.CreateSession("keep", "u1", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession(keep) err: %v", err)
	}
	if err := store.CreateSession("revoke1", "u1", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession(revoke1) err: %v", err)
	}
	if err := store.CreateSession("revoke2", "u1", "login_challenge", expiresAt); err != nil {
		t.Fatalf("CreateSession(revoke2) err: %v", err)
	}
	// A different user's session must never be touched by userID-scoped revocation.
	if err := store.CreateSession("other-user", "u2", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession(other-user) err: %v", err)
	}

	if err := store.RevokeAllSessionsForUserExcept("u1", "keep"); err != nil {
		t.Fatalf("RevokeAllSessionsForUserExcept err: %v", err)
	}

	active, err := store.IsActive("keep", "u1", "refresh", time.Now())
	if err != nil || !active {
		t.Fatalf("expected excepted session to remain active, active=%v err=%v", active, err)
	}
	if active, _ := store.IsActive("revoke1", "u1", "refresh", time.Now()); active {
		t.Fatal("expected revoke1 to be revoked")
	}
	if active, _ := store.IsActive("revoke2", "u1", "login_challenge", time.Now()); active {
		t.Fatal("expected revoke2 to be revoked")
	}
	if active, _ := store.IsActive("other-user", "u2", "refresh", time.Now()); !active {
		t.Fatal("expected other user's session to be unaffected")
	}
}

func TestSessionStore_RevokeAllSessionsForUserExcept_EmptyExceptRevokesAll(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	expiresAt := time.Now().Add(time.Hour)
	if err := store.CreateSession("s1", "u1", "refresh", expiresAt); err != nil {
		t.Fatalf("CreateSession err: %v", err)
	}

	if err := store.RevokeAllSessionsForUserExcept("u1", ""); err != nil {
		t.Fatalf("RevokeAllSessionsForUserExcept err: %v", err)
	}
	if active, _ := store.IsActive("s1", "u1", "refresh", time.Now()); active {
		t.Fatal("expected all sessions revoked when exceptID is empty")
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
