package auth

import (
	"testing"
)

func setupTestUserService(t *testing.T) *UserService {
	t.Helper()
	store, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to setup user store: %v", err)
	}
	return NewUserService(store)
}

func TestUserService_CreateUser(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.CreateUser("alice", "alice@example.com", "secure", "admin")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.Username != "alice" || user.Email != "alice@example.com" || user.Role != "admin" {
		t.Errorf("User not created with correct data")
	}
}

func TestUserService_CreateUser_Duplicate(t *testing.T) {
	service := setupTestUserService(t)

	_, _ = service.CreateUser("alice", "alice@example.com", "secure", "editor")

	_, err := service.CreateUser("alice", "alice2@example.com", "secure", "editor")
	if err != ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists for username, got: %v", err)
	}

	_, err = service.CreateUser("bob", "alice@example.com", "secure", "editor")
	if err != ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists for email, got: %v", err)
	}
}

func TestUserService_CreateUser_InvalidRole(t *testing.T) {
	service := setupTestUserService(t)

	_, err := service.CreateUser("bob", "bob@example.com", "secure", "guest")
	if err != ErrUserInvalidRole {
		t.Errorf("Expected ErrUserInvalidRole, got: %v", err)
	}
}

func TestUserService_GetUserByEmailOrUsernameAndPassword(t *testing.T) {
	service := setupTestUserService(t)
	_, _ = service.CreateUser("alice", "alice@example.com", "mypassword", "editor")

	_, err := service.GetUserByEmailOrUsernameAndPassword("alice", "mypassword")
	if err != nil {
		t.Errorf("Valid login failed: %v", err)
	}

	_, err = service.GetUserByEmailOrUsernameAndPassword("alice@example.com", "mypassword")
	if err != nil {
		t.Errorf("Valid login by email failed: %v", err)
	}

	_, err = service.GetUserByEmailOrUsernameAndPassword("alice", "wrongpass")
	if err != ErrUserInvalidCredentials {
		t.Errorf("Expected ErrUserInvalidCredentials, got: %v", err)
	}
}

func TestUserService_UpdateUser(t *testing.T) {
	service := setupTestUserService(t)

	user, _ := service.CreateUser("bob", "bob@example.com", "initial", "editor")

	updated, err := service.UpdateUser(user.ID, "bobnew", "bobnew@example.com", "newpass", "admin")
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	if updated.Username != "bobnew" || updated.Email != "bobnew@example.com" || updated.Role != "admin" {
		t.Errorf("Update did not persist values")
	}
}

func TestUserService_DeleteUser(t *testing.T) {
	service := setupTestUserService(t)

	// admin should not be deletable
	admin, _ := service.CreateUser("admin", "admin@example.com", "secret", "admin")
	err := service.DeleteUser(admin.ID)
	if err != ErrUserAdminCannotBeDeleted {
		t.Errorf("Expected ErrUserAdminCannotBeDeleted when deleting admin, got: %v", err)
	}

	editor, _ := service.CreateUser("editor", "editor@example.com", "secret", "editor")
	err = service.DeleteUser(editor.ID)
	if err != nil {
		t.Errorf("Failed to delete editor: %v", err)
	}
}

func TestUserService_InitDefaultAdmin(t *testing.T) {
	store, _ := NewUserStore(t.TempDir())
	service := NewUserService(store)

	err := service.InitDefaultAdmin("")
	if err != nil {
		t.Errorf("InitDefaultAdmin failed: %v", err)
	}

	users, err := service.GetUsers()
	if err != nil || len(users) != 1 || users[0].Username != "admin" {
		t.Errorf("Expected default admin user, got: %+v", users)
	}
}
