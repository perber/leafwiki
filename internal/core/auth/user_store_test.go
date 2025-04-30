package auth

import "testing"

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

func TestUserStore_CreateUser(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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
	defer store.Close()
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
	defer store.Close()
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
	defer store.Close()
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
	defer store.Close()
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

func TestUserStore_UpdateUser_EMailAlreadyExists(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()
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
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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

func TestUserStore_GetUserByUsernameAndPassword(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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

	// Retrieve user by username and password
	retrievedUser, err := store.GetUserByUsernameAndPassword(user1.Username, user1.Password)
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

	// Attempt to retrieve a user with incorrect password
	_, err = store.GetUserByUsernameAndPassword(user1.Username, "wrongpassword")
	if err == nil {
		t.Fatalf("Expected error for incorrect password, got nil")
	}

	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}

	// Attempt to retrieve a non-existing user
	_, err = store.GetUserByUsernameAndPassword("non-existing-username", "password")
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}

	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetUserByEmailAndPassword(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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
	// Retrieve user by email and password
	retrievedUser, err := store.GetUserByEmailAndPassword(user1.Email, user1.Password)
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
	// Attempt to retrieve a user with incorrect password
	_, err = store.GetUserByEmailAndPassword(user1.Email, "wrongpassword")
	if err == nil {
		t.Fatalf("Expected error for incorrect password, got nil")
	}
	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
	// Attempt to retrieve a non-existing user
	_, err = store.GetUserByEmailAndPassword("non-existing-email", "password")
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}

	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetUserByUsernameOrEmailAndPassword(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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

	// Retrieve user by username or email and password
	retrievedUser, err := store.GetUserByUsernameOrEmailAndPassword(user1.Username, user1.Password)
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
	// Retrieve user by email and password
	retrievedUser, err = store.GetUserByUsernameOrEmailAndPassword(user1.Email, user1.Password)
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
	// Attempt to retrieve a user with incorrect password
	_, err = store.GetUserByUsernameOrEmailAndPassword(user1.Username, "wrongpassword")
	if err == nil {
		t.Fatalf("Expected error for incorrect password, got nil")
	}
	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
	// Attempt to retrieve a non-existing user
	_, err = store.GetUserByUsernameOrEmailAndPassword("non-existing-username", "password")
	if err == nil {
		t.Fatalf("Expected error for non-existing user, got nil")
	}
	if err != ErrUserNotFound {
		t.Fatalf("Expected ErrUserNotFound, got %v", err)
	}
}

func TestUserStore_GetUserByEmail(t *testing.T) {
	store := setupTestUserStore(t)
	defer store.Close()

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
	defer store.Close()

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
	defer store.Close()

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
