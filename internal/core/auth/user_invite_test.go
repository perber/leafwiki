package auth

import "testing"

func TestUserService_InviteUser_CreatesUserWithMustSetPasswordTrue(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	if !user.MustSetPassword {
		t.Fatalf("expected MustSetPassword true for a freshly invited user")
	}

	stored, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if !stored.MustSetPassword {
		t.Fatalf("expected MustSetPassword to persist as true")
	}
	if stored.Password == "" {
		t.Fatalf("expected an invited user to still have a (random, unusable) password hash set")
	}
}

func TestUserService_InviteUser_DuplicateUsername_ReturnsAlreadyExists(t *testing.T) {
	service := setupTestUserService(t)

	if _, err := service.InviteUser("bob", "bob@example.com", RoleEditor); err != nil {
		t.Fatalf("first InviteUser returned error: %v", err)
	}

	if _, err := service.InviteUser("bob", "someone-else@example.com", RoleEditor); err != ErrUserAlreadyExists {
		t.Fatalf("expected ErrUserAlreadyExists for a duplicate username, got %v", err)
	}
}

func TestUserService_CompleteInvite_ClearsMustSetPasswordAndSetsPassword(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	if err := service.CompleteInvite(user.ID, "a-real-password"); err != nil {
		t.Fatalf("CompleteInvite returned error: %v", err)
	}

	stored, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if stored.MustSetPassword {
		t.Fatalf("expected MustSetPassword false after CompleteInvite")
	}

	if _, err := service.DoesIDAndPasswordMatch(user.ID, "a-real-password"); err != nil {
		t.Fatalf("expected the new password to authenticate, got error: %v", err)
	}
}
