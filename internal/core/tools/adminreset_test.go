package tools

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/test_utils"
)

func TestResetAdminPassword(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// First, create an admin user
	store, err := auth.NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}

	userService := auth.NewUserService(store)
	_, err = userService.CreateUser("admin", "admin@example.com", "oldpassword", "admin")
	if err != nil {
		_ = store.Close()
		t.Fatalf("Failed to create admin user: %v", err)
	}

	// Close the store before ResetAdminPassword opens it
	err = store.Close()
	if err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Now test ResetAdminPassword
	adminUser, err := ResetAdminPassword(tempDir, "", "")
	if err != nil {
		t.Fatalf("ResetAdminPassword failed: %v", err)
	}

	if adminUser.Username != "admin" {
		t.Errorf("Expected username 'admin', got: %s", adminUser.Username)
	}

	if adminUser.Password == "" {
		t.Errorf("Expected a new password to be generated, got empty string")
	}

	// Verify we can log in with the new password
	store2, err := auth.NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}
	defer func() {
		if err := store2.Close(); err != nil {
			t.Fatalf("Failed to close user store: %v", err)
		}
	}()

	userService2 := auth.NewUserService(store2)
	_, err = userService2.GetUserByEmailOrUsernameAndPassword("admin", adminUser.Password)
	if err != nil {
		t.Errorf("Failed to login with new password: %v", err)
	}
}

func TestResetAdminPassword_NoAdmin(t *testing.T) {
	// Create a temporary directory for the test (no admin user exists)
	tempDir := t.TempDir()

	// Test ResetAdminPassword when no admin exists
	adminUser, err := ResetAdminPassword(tempDir, "", "")
	if err != nil {
		t.Fatalf("ResetAdminPassword failed: %v", err)
	}

	if adminUser.Username != "admin" {
		t.Errorf("Expected username 'admin', got: %s", adminUser.Username)
	}

	if adminUser.Email != "admin@localhost" {
		t.Errorf("Expected email 'admin@localhost', got: %s", adminUser.Email)
	}

	if adminUser.Password == "" {
		t.Errorf("Expected a new password to be generated, got empty string")
	}

	// Verify we can log in with the new password
	store, err := auth.NewUserStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Failed to close user store: %v", err)
		}
	}()

	userService := auth.NewUserService(store)
	_, err = userService.GetUserByEmailOrUsernameAndPassword("admin", adminUser.Password)
	if err != nil {
		t.Errorf("Failed to login with new password: %v", err)
	}
}

func TestResetAdminPassword_NoAdmin_UsesGivenUsernameAndEmail(t *testing.T) {
	// Create a temporary directory for the test (no admin user exists)
	tempDir := t.TempDir()

	adminUser, err := ResetAdminPassword(tempDir, "root", "root@example.com")
	if err != nil {
		t.Fatalf("ResetAdminPassword failed: %v", err)
	}

	if adminUser.Username != "root" {
		t.Errorf("Expected username 'root', got: %s", adminUser.Username)
	}

	if adminUser.Email != "root@example.com" {
		t.Errorf("Expected email 'root@example.com', got: %s", adminUser.Email)
	}
}

// A corrupt users.db must stop the reset instead of being replaced by an
// empty one with a freshly created admin (ADR-0013).
func TestResetAdminPassword_CorruptUsersDB_FailsAndCreatesNoAdmin(t *testing.T) {
	dir := t.TempDir()
	files := test_utils.WriteCorruptSQLiteDB(t, filepath.Join(dir, "users.db"))

	user, err := ResetAdminPassword(dir, "", "")

	if err == nil {
		t.Fatalf("ResetAdminPassword: expected an error, got user %+v", user)
	}
	var corrupt *sqliteutil.CorruptDatabaseError
	if !errors.As(err, &corrupt) {
		t.Fatalf("ResetAdminPassword error = %v (%T), want it to wrap *sqliteutil.CorruptDatabaseError", err, err)
	}
	test_utils.AssertFilesUnchanged(t, files)
}
