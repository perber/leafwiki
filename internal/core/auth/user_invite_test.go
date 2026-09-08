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

// An admin can also complete an invite out-of-band by setting a real password
// through the "Change Password" dialog (PUT /api/users/:id with a password)
// instead of the invitee clicking the emailed link. That must clear the
// pending state too, otherwise the account shows "Invitation pending" forever
// even though it now has a usable password.
func TestUserService_UpdateUser_AdminSetsPasswordForInvitedUser_ClearsMustSetPassword(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	updated, err := service.UpdateUser(user.ID, user.Username, user.Email, "an-admin-set-password", "")
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if updated.MustSetPassword {
		t.Fatalf("expected MustSetPassword false on the returned user after an admin set a password")
	}

	stored, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if stored.MustSetPassword {
		t.Fatalf("expected MustSetPassword to be cleared in the store after an admin set a password")
	}

	if _, err := service.DoesIDAndPasswordMatch(user.ID, "an-admin-set-password"); err != nil {
		t.Fatalf("expected the admin-set password to authenticate, got error: %v", err)
	}
}

// A profile/role edit that does not set a password must leave an outstanding
// invite pending — only an explicit password set completes it.
func TestUserService_UpdateUser_NoPassword_KeepsInvitedUserPending(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.InviteUser("bob", "bob@example.com", RoleEditor)
	if err != nil {
		t.Fatalf("InviteUser returned error: %v", err)
	}

	updated, err := service.UpdateUser(user.ID, "bobby", user.Email, "", RoleViewer)
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if !updated.MustSetPassword {
		t.Fatalf("expected a password-less edit to leave MustSetPassword true")
	}

	stored, err := service.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID returned error: %v", err)
	}
	if !stored.MustSetPassword {
		t.Fatalf("expected MustSetPassword to persist as true after a password-less edit")
	}
}
