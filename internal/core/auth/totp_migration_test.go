package auth

import (
	"database/sql"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

// createPreTOTPDatabase creates a users.db using only the pre-TOTP schema,
// simulating a real deployment's database from before this feature existed.
func createPreTOTPDatabase(t *testing.T, storageDir string) {
	t.Helper()

	db, err := sql.Open("sqlite", databasePath(storageDir, "users.db"))
	if err != nil {
		t.Fatalf("failed to open pre-TOTP database: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)

	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			role TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("failed to create pre-TOTP schema: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, password, email, role)
		VALUES ('1', 'admin', 'bcrypt-hash-of-real-password', 'admin@example.com', 'admin');
	`)
	if err != nil {
		t.Fatalf("failed to seed pre-TOTP user: %v", err)
	}
}

func TestUserStore_MigratesExistingDatabaseInPlace(t *testing.T) {
	storageDir := t.TempDir()
	createPreTOTPDatabase(t, storageDir)

	store, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("failed to open pre-TOTP database through UserStore: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user, err := store.GetUserByID("1")
	if err != nil {
		t.Fatalf("failed to read pre-existing user after migration: %v", err)
	}

	if user.Username != "admin" || user.Email != "admin@example.com" || user.Role != RoleAdmin {
		t.Fatalf("pre-existing user fields changed after migration: %+v", user)
	}
	if user.Password != "bcrypt-hash-of-real-password" {
		t.Fatalf("password hash changed after migration: got %q", user.Password)
	}
}

func TestUserStore_MigrationAddsTOTPColumnsWithSafeDefaults(t *testing.T) {
	storageDir := t.TempDir()
	createPreTOTPDatabase(t, storageDir)

	store, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("failed to migrate pre-TOTP database: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user, err := store.GetUserByID("1")
	if err != nil {
		t.Fatalf("failed to read migrated user: %v", err)
	}

	if user.TOTPEnabled {
		t.Errorf("expected TOTPEnabled = false for pre-existing user, got true")
	}
	if user.TOTPSecretEncrypted != "" {
		t.Errorf("expected empty TOTPSecretEncrypted for pre-existing user, got %q", user.TOTPSecretEncrypted)
	}
	if len(user.TOTPRecoveryCodeHashes) != 0 {
		t.Errorf("expected no recovery code hashes for pre-existing user, got %v", user.TOTPRecoveryCodeHashes)
	}
	if user.TOTPEnabledAt != nil {
		t.Errorf("expected nil TOTPEnabledAt for pre-existing user, got %v", user.TOTPEnabledAt)
	}
}

func TestUserStore_MigrationIsIdempotent(t *testing.T) {
	storageDir := t.TempDir()
	createPreTOTPDatabase(t, storageDir)

	store1, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("first migration failed: %v", err)
	}
	if err := store1.Close(); err != nil {
		t.Fatalf("failed to close store after first migration: %v", err)
	}

	// Re-opening (and thus re-running ensureSchema/ensureTOTPColumns) against
	// the already-migrated database must not fail or duplicate/lose data.
	store2, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("second migration run failed: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store2.Close, t)

	count, err := store2.GetUserCount()
	if err != nil {
		t.Fatalf("failed to count users after repeated migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user after repeated migration, got %d", count)
	}

	user, err := store2.GetUserByID("1")
	if err != nil {
		t.Fatalf("failed to read user after repeated migration: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("user data changed after repeated migration: %+v", user)
	}
}

func TestUserStore_FreshDatabaseInitializesWithTOTPSchema(t *testing.T) {
	storageDir := t.TempDir()

	store, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("failed to create fresh database: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user := &User{
		ID:       "1",
		Username: "admin",
		Password: "password",
		Email:    "admin@example.com",
		Role:     RoleAdmin,
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("failed to create user in fresh database: %v", err)
	}

	retrieved, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to read user from fresh database: %v", err)
	}
	if retrieved.TOTPEnabled {
		t.Errorf("expected TOTPEnabled = false by default, got true")
	}
	if retrieved.TOTPSecretEncrypted != "" {
		t.Errorf("expected empty TOTPSecretEncrypted by default, got %q", retrieved.TOTPSecretEncrypted)
	}
}

func TestUserStore_TOTPLifecycle(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user := &User{
		ID:       "1",
		Username: "testuser",
		Password: "password",
		Email:    "testuser@example.com",
		Role:     RoleEditor,
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if err := store.SetPendingTOTPSecret(user.ID, "encrypted-pending-secret"); err != nil {
		t.Fatalf("SetPendingTOTPSecret failed: %v", err)
	}

	pending, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to read user with pending secret: %v", err)
	}
	if pending.TOTPEnabled {
		t.Fatalf("expected TOTP still disabled while pending, got enabled")
	}
	if pending.TOTPSecretEncrypted != "encrypted-pending-secret" {
		t.Fatalf("expected pending secret to be stored, got %q", pending.TOTPSecretEncrypted)
	}

	hashes := []string{"hash1", "hash2", "hash3"}
	if err := store.EnableTOTP(user.ID, "encrypted-confirmed-secret", hashes); err != nil {
		t.Fatalf("EnableTOTP failed: %v", err)
	}

	enabled, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to read user after enabling TOTP: %v", err)
	}
	if !enabled.TOTPEnabled {
		t.Fatalf("expected TOTP enabled after EnableTOTP")
	}
	if enabled.TOTPSecretEncrypted != "encrypted-confirmed-secret" {
		t.Fatalf("expected confirmed secret to be stored, got %q", enabled.TOTPSecretEncrypted)
	}
	if len(enabled.TOTPRecoveryCodeHashes) != 3 {
		t.Fatalf("expected 3 recovery code hashes, got %d", len(enabled.TOTPRecoveryCodeHashes))
	}
	if enabled.TOTPEnabledAt == nil {
		t.Fatalf("expected TOTPEnabledAt to be set after EnableTOTP")
	}

	// Consuming a recovery code removes just that one hash.
	remaining := []string{"hash1", "hash3"}
	if err := store.UpdateRecoveryCodeHashes(user.ID, remaining); err != nil {
		t.Fatalf("UpdateRecoveryCodeHashes failed: %v", err)
	}
	afterConsume, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to read user after consuming recovery code: %v", err)
	}
	if len(afterConsume.TOTPRecoveryCodeHashes) != 2 {
		t.Fatalf("expected 2 remaining recovery code hashes, got %d", len(afterConsume.TOTPRecoveryCodeHashes))
	}

	if err := store.DisableTOTP(user.ID); err != nil {
		t.Fatalf("DisableTOTP failed: %v", err)
	}

	disabled, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("failed to read user after disabling TOTP: %v", err)
	}
	if disabled.TOTPEnabled {
		t.Fatalf("expected TOTP disabled after DisableTOTP")
	}
	if disabled.TOTPSecretEncrypted != "" {
		t.Fatalf("expected secret cleared after DisableTOTP, got %q", disabled.TOTPSecretEncrypted)
	}
	if len(disabled.TOTPRecoveryCodeHashes) != 0 {
		t.Fatalf("expected recovery codes cleared after DisableTOTP, got %v", disabled.TOTPRecoveryCodeHashes)
	}
	if disabled.TOTPLastResetAt == nil {
		t.Fatalf("expected TOTPLastResetAt to be set after DisableTOTP")
	}
}

func TestUserStore_TOTPMethods_NotFoundForUnknownUser(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if err := store.SetPendingTOTPSecret("missing", "secret"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound from SetPendingTOTPSecret, got %v", err)
	}
	if err := store.EnableTOTP("missing", "secret", nil); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound from EnableTOTP, got %v", err)
	}
	if err := store.DisableTOTP("missing"); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound from DisableTOTP, got %v", err)
	}
	if err := store.UpdateRecoveryCodeHashes("missing", nil); err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound from UpdateRecoveryCodeHashes, got %v", err)
	}
}

func TestUserStore_ExistingColumnsDetectsPreTOTPSchema(t *testing.T) {
	storageDir := t.TempDir()
	createPreTOTPDatabase(t, storageDir)

	store := &UserStore{storageDir: storageDir, filename: "users.db"}
	if err := store.Connect(); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	cols, err := store.existingColumns()
	if err != nil {
		t.Fatalf("existingColumns failed: %v", err)
	}
	for _, totpCol := range []string{"totp_secret_encrypted", "totp_enabled", "totp_recovery_codes_json", "totp_enabled_at", "totp_last_reset_at"} {
		if cols[totpCol] {
			t.Fatalf("expected pre-TOTP schema to be missing column %s before migration", totpCol)
		}
	}
	if !cols["id"] || !cols["username"] {
		t.Fatalf("expected base columns to be detected, got %v", cols)
	}
}
