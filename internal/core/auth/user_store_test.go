package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
)

func TestDatabasePath_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(databasePath(`C:\wiki\data`, "users.db"), `\`, `/`)
	want := `C:/wiki/data/users.db`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func setupTestUserStore(t *testing.T) *UserStore {
	t.Helper()
	// Create a temporary directory for the database
	storageDir := t.TempDir()
	userStore, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}
	return userStore
}

func TestUserStore_UsesWALJournalMode(t *testing.T) {
	userStore := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(userStore.Close, t)

	if err := userStore.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	var mode string
	if err := userStore.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("failed to read journal_mode: %v", err)
	}
	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want %q", mode, "wal")
	}
}

// Pins the invariant restore's live-swap path depends on: UserStore.suspend
// (which this test approximates via the exported Close) must leave users.db
// self-consistent on its own, since restore/swap.go's removeStaleWALSidecars
// deletes any leftover -wal/-shm before a fresh store reopens the file — if
// WAL content weren't checkpointed into the main file on close, that data
// would be silently lost the moment the sidecar is removed.
func TestUserStore_DataSurvivesCloseAndFreshReopen(t *testing.T) {
	storageDir := t.TempDir()
	userStore, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}

	user := &User{ID: "u1", Username: "alice", Password: "hash", Email: "alice@example.com", Role: RoleEditor}
	if err := userStore.CreateUser(user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if err := userStore.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Same sequence AuthService.ReplaceUserStore follows after a restore
	// swap: a brand new UserStore over the same storage dir.
	reopened, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("NewUserStore (reopen): %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(reopened.Close, t)

	got, err := reopened.GetUserByID("u1")
	if err != nil {
		t.Fatalf("GetUserByID after close+reopen: %v", err)
	}
	if got.Username != "alice" {
		t.Fatalf("Username = %q, want alice", got.Username)
	}
}

func TestUserStore_CreatesDatabaseInStorageDir(t *testing.T) {
	storageDir := t.TempDir()
	userStore, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("Failed to create user store: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(userStore.Close, t)

	if _, err := os.Stat(filepath.Join(storageDir, "users.db")); err != nil {
		t.Fatalf("expected users.db in storage dir, got err: %v", err)
	}
}

func TestUserStore_CreateUser(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user := &User{
		ID:       "1",
		Username: "testuser",
		Password: "password",
		Email:    "user1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify the user was created
	retrievedUser, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, retrievedUser.ID)
	}
	if retrievedUser.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrievedUser.Username)
	}
	if retrievedUser.Email != user.Email {
		t.Errorf("Expected email %s, got %s", user.Email, retrievedUser.Email)
	}
	if retrievedUser.Role != user.Role {
		t.Errorf("Expected role %s, got %s", user.Role, retrievedUser.Role)
	}
	if retrievedUser.Password != user.Password {
		t.Errorf("Expected password %s, got %s", user.Password, retrievedUser.Password)
	}
}

func TestUserStore_CreateUser_EmailAlreadyExists(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}
	err = store.CreateUser(user2)
	if err == nil {
		t.Fatalf("Expected error for duplicate email, got nil")
	}

	if err != ErrUserAlreadyExists {
		t.Fatalf("Expected ErrUserAlreadyExists, got %v", err)
	}

}

func TestUserStore_CreateUser_UsernameAlreadyExists(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	user2 := &User{
		ID:       "2",
		Username: "testuser1",
		Password: "password2",
		Email:    "testuser2@example.com",
	}

	err = store.CreateUser(user2)
	if err == nil {
		t.Fatalf("Expected error for duplicate username, got nil")
	}

	if err != ErrUserAlreadyExists {
		t.Fatalf("Expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserStore_GetUserByID_NotExisting(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	user := &User{
		ID:       "1",
		Username: "testuser",
		Password: "password",
		Email:    "testuser@example.com",
		Role:     "admin",
	}
	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Attempt to retrieve a non-existing user
	_, err = store.GetUserByID("non-existing-id")
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}
	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}

	// Attempt to retrieve an existing user
	retrievedUser, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}
	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, retrievedUser.ID)
	}
}

func TestUserStore_UpdateUser(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	user := &User{
		ID:       "1",
		Username: "testuser",
		Password: "password",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update the user
	user.Username = "updateduser"
	user.Password = "newpassword"

	err = store.UpdateUser(user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify the user was updated
	retrievedUser, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}

	if retrievedUser.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrievedUser.Username)
	}

	if retrievedUser.Password != user.Password {
		t.Errorf("Expected password %s, got %s", user.Password, retrievedUser.Password)
	}

	// Verify error message when the user does not exist
	nonExistingUser := &User{
		ID:       "non-existing-id",
		Username: "nonexistinguser",
		Password: "nonexistingpassword",
	}
	err = store.UpdateUser(nonExistingUser)
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}
}

func TestUserStore_UpdateUser_LastAdminCannotBeDemoted(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	admin := &User{
		ID:       "1",
		Username: "admin",
		Password: "password",
		Email:    "admin@example.com",
		Role:     RoleAdmin,
	}
	if err := store.CreateUser(admin); err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	admin.Role = RoleViewer
	err := store.UpdateUser(admin)
	if err != ErrLastAdminCannotBeDemoted {
		t.Fatalf("expected ErrLastAdminCannotBeDemoted, got %v", err)
	}
}

func TestUserStore_UpdateUser_EMailAlreadyExists(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	updateUser := &User{
		ID:       user2.ID,
		Username: user2.Username,
		Password: user2.Password,
		Email:    user1.Email, // This email already exists
		Role:     user2.Role,
	}

	err = store.UpdateUser(updateUser)
	if err == nil {
		t.Fatalf("Expected error for duplicate email, got nil")
	}

	if err != ErrUserAlreadyExists {
		t.Fatalf("Expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserStore_UpdateUser_UsernameAlreadyExists(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	updateUser := &User{
		ID:       user2.ID,
		Username: user1.Username, // This username already exists
		Password: user2.Password,
		Email:    user2.Email,
		Role:     user2.Role,
	}

	err = store.UpdateUser(updateUser)
	if err == nil {
		t.Fatalf("Expected error for duplicate email, got nil")
	}

	if err != ErrUserAlreadyExists {
		t.Fatalf("Expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserStore_DeleteUser(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)
	user := &User{
		ID:       "1",
		Username: "testuser",
		Password: "password",
		Email:    "testuser@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Count the number of users before deletion
	users, err := store.GetAllUsers()
	if err != nil {
		t.Fatalf("Failed to get all users: %v", err)
	}
	initialCount := len(users)
	// Delete the user
	err = store.DeleteUser(user.ID)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify the user was deleted
	_, err = store.GetUserByID(user.ID)
	if err == nil {
		t.Fatalf("Expected error for deleted user, got nil")
	}

	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}

	// Count the number of users after deletion
	users, err = store.GetAllUsers()
	if err != nil {
		t.Fatalf("Failed to get all users: %v", err)
	}

	finalCount := len(users)
	if finalCount != initialCount-1 {
		t.Errorf("Expected user count %d, got %d", initialCount-1, finalCount)
	}

}
func TestUserStore_DeleteUser_NotExisting(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	// Attempt to delete a non-existing user
	err := store.DeleteUser("non-existing-id")
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}
	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetAllUsers(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Retrieve all users
	users, err := store.GetAllUsers()
	if err != nil {
		t.Fatalf("Failed to get all users: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(users))
	}

	if users[0].ID != user1.ID && users[1].ID != user2.ID {
		t.Fatalf("Expected user IDs %s and %s, got %s and %s", user1.ID, user2.ID, users[0].ID, users[1].ID)
	}

	if users[0].Username != user1.Username && users[1].Username != user2.Username {
		t.Fatalf("Expected usernames %s and %s, got %s and %s", user1.Username, user2.Username, users[0].Username, users[1].Username)
	}

	if users[0].Email != user1.Email && users[1].Email != user2.Email {
		t.Fatalf("Expected emails %s and %s, got %s and %s", user1.Email, user2.Email, users[0].Email, users[1].Email)
	}

	if users[0].Role != user1.Role && users[1].Role != user2.Role {
		t.Fatalf("Expected roles %s and %s, got %s and %s", user1.Role, user2.Role, users[0].Role, users[1].Role)
	}
	if users[0].Password != user1.Password && users[1].Password != user2.Password {
		t.Fatalf("Expected passwords %s and %s, got %s and %s", user1.Password, user2.Password, users[0].Password, users[1].Password)
	}

}
func TestUserStore_GetUserCount(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Retrieve the user count
	count, err := store.GetUserCount()
	if err != nil {
		t.Fatalf("Failed to get user count: %v", err)
	}

	if count != 2 {
		t.Fatalf("Expected user count 2, got %d", count)
	}
}

func TestUserStore_GetUserByEmail(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}
	// Retrieve user by email
	retrievedUser, err := store.GetUserByEmail(user1.Email)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}
	if retrievedUser.ID != user1.ID {
		t.Errorf("Expected user ID %s, got %s", user1.ID, retrievedUser.ID)
	}
	if retrievedUser.Username != user1.Username {
		t.Errorf("Expected username %s, got %s", user1.Username, retrievedUser.Username)
	}
	if retrievedUser.Email != user1.Email {
		t.Errorf("Expected email %s, got %s", user1.Email, retrievedUser.Email)
	}
	if retrievedUser.Role != user1.Role {
		t.Errorf("Expected role %s, got %s", user1.Role, retrievedUser.Role)
	}
	if retrievedUser.Password != user1.Password {
		t.Errorf("Expected password %s, got %s", user1.Password, retrievedUser.Password)
	}
}

func TestUserStore_GetUserByUsername(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		ID:       "2",
		Username: "testuser2",
		Password: "password2",
		Email:    "testuser2@example.com",
		Role:     "admin",
	}

	err = store.CreateUser(user2)
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}
	// Retrieve user by email
	retrievedUser, err := store.GetUserByUsername(user1.Username)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}
	if retrievedUser.ID != user1.ID {
		t.Errorf("Expected user ID %s, got %s", user1.ID, retrievedUser.ID)
	}
	if retrievedUser.Username != user1.Username {
		t.Errorf("Expected username %s, got %s", user1.Username, retrievedUser.Username)
	}
	if retrievedUser.Email != user1.Email {
		t.Errorf("Expected email %s, got %s", user1.Email, retrievedUser.Email)
	}
	if retrievedUser.Role != user1.Role {
		t.Errorf("Expected role %s, got %s", user1.Role, retrievedUser.Role)
	}
	if retrievedUser.Password != user1.Password {
		t.Errorf("Expected password %s, got %s", user1.Password, retrievedUser.Password)
	}
}

func TestUserStoreUpdatePassword(t *testing.T) {
	store := setupTestUserStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	user1 := &User{
		ID:       "1",
		Username: "testuser1",
		Password: "password1",
		Email:    "testuser1@example.com",
		Role:     "admin",
	}

	err := store.CreateUser(user1)
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	// Update the user's password
	err = store.UpdatePassword(user1.ID, "newpassword")
	if err != nil {
		t.Fatalf("Failed to update password: %v", err)
	}

	// Verify the password was updated
	retrievedUser, err := store.GetUserByID(user1.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve user: %v", err)
	}
	if retrievedUser.Password != "newpassword" {
		t.Errorf("Expected password %s, got %s", "newpassword", retrievedUser.Password)
	}

}

// TestUserStore_Suspend_ClosesDBAndBlocksReconnect is the regression test for
// the root cause of the Windows live-restore bug: a plain Close() lets the
// very next query silently reopen a new *sql.DB (Connect() reopens whenever
// f.db is nil), which would race a reconnect against restore.SwapAll's
// os.Rename of users.db. suspend() must close the connection AND make
// Connect() refuse to reopen it, so a query landing during the swap window
// fails fast instead of grabbing a fresh OS-level file handle.
func TestUserStore_Suspend_ClosesDBAndBlocksReconnect(t *testing.T) {
	store := setupTestUserStore(t)

	user := &User{
		ID:       "1",
		Username: "suspend-test-user",
		Password: "password1",
		Email:    "suspend-test-user@example.com",
		Role:     "admin",
	}
	if err := store.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := store.suspend(); err != nil {
		t.Fatalf("suspend failed: %v", err)
	}
	if store.db != nil {
		t.Fatal("expected db to be nil immediately after suspend")
	}

	_, err := store.GetUserByUsername("suspend-test-user")
	if err == nil {
		t.Fatal("expected a query against a suspended store to fail, not silently reconnect")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_user_store_unavailable" {
		t.Fatalf("expected auth_user_store_unavailable, got %v", err)
	}
	if store.db != nil {
		t.Fatal("expected the failed query to NOT have reopened db — that's exactly the race this fix prevents")
	}

	// suspend must be idempotent — Manager.rollbackOrIntervene's cleanup
	// paths may end up calling code that touches an already-suspended store.
	if err := store.suspend(); err != nil {
		t.Fatalf("expected a second suspend call to be a safe no-op, got: %v", err)
	}
}
