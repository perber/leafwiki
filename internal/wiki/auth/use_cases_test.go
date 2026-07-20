package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/favorites"
)

func setupUpdateUserUseCase(t *testing.T) (*UpdateUserUseCase, *coreauth.UserService) {
	t.Helper()
	store, err := coreauth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})

	userSvc := coreauth.NewUserService(store)
	resolver, err := coreauth.NewUserResolver(userSvc)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}
	return NewUpdateUserUseCase(userSvc, resolver, slog.Default()), userSvc
}

// TestUpdateUser_AdminCanChangeRole verifies that an admin requester can promote
// or demote another user's role.
func TestUpdateUser_AdminCanChangeRole(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	viewer, err := svc.CreateUser("viewer", "viewer@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := uc.Execute(context.Background(), UpdateUserInput{
		ID:               viewer.ID,
		Username:         viewer.Username,
		Email:            viewer.Email,
		Role:             coreauth.RoleAdmin,
		RequesterIsAdmin: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.User.Role != coreauth.RoleAdmin {
		t.Errorf("expected role %q, got %q", coreauth.RoleAdmin, out.User.Role)
	}
}

// TestUpdateUser_AdminCanUpdateProfileWithoutRole verifies that an admin can
// update username/email without sending a role and the existing role is kept.
func TestUpdateUser_AdminCanUpdateProfileWithoutRole(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	editor, err := svc.CreateUser("ed", "ed@example.com", "pass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := uc.Execute(context.Background(), UpdateUserInput{
		ID:               editor.ID,
		Username:         "ed-admin-updated",
		Email:            "ed-admin-updated@example.com",
		Role:             "",
		RequesterIsAdmin: true,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.User.Username != "ed-admin-updated" {
		t.Errorf("expected username %q, got %q", "ed-admin-updated", out.User.Username)
	}
	if out.User.Email != "ed-admin-updated@example.com" {
		t.Errorf("expected email %q, got %q", "ed-admin-updated@example.com", out.User.Email)
	}
	if out.User.Role != coreauth.RoleEditor {
		t.Errorf("expected role %q, got %q", coreauth.RoleEditor, out.User.Role)
	}
}

// TestUpdateUser_NonAdminCannotEscalateRole is the regression test for
// GHSA-jj4r-587p-r5h5: a viewer calling PUT /api/users/:id on their own account
// must not be able to promote themselves to admin.
func TestUpdateUser_NonAdminCannotEscalateRole(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	viewer, err := svc.CreateUser("viewer", "viewer@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := uc.Execute(context.Background(), UpdateUserInput{
		ID:               viewer.ID,
		Username:         viewer.Username,
		Email:            viewer.Email,
		Role:             coreauth.RoleAdmin, // attacker sends "admin"
		RequesterIsAdmin: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.User.Role != coreauth.RoleViewer {
		t.Errorf("role escalation succeeded: expected %q, got %q", coreauth.RoleViewer, out.User.Role)
	}
}

// TestUpdateUser_NonAdminCanUpdateOwnProfile verifies that non-admin users can
// still change their username and email while their role stays unchanged.
func TestUpdateUser_NonAdminCanUpdateOwnProfile(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	editor, err := svc.CreateUser("ed", "ed@example.com", "pass", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := uc.Execute(context.Background(), UpdateUserInput{
		ID:               editor.ID,
		Username:         "ed-updated",
		Email:            "ed-updated@example.com",
		Role:             coreauth.RoleAdmin, // should be silently ignored
		RequesterIsAdmin: false,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.User.Username != "ed-updated" {
		t.Errorf("expected username %q, got %q", "ed-updated", out.User.Username)
	}
	if out.User.Email != "ed-updated@example.com" {
		t.Errorf("expected email %q, got %q", "ed-updated@example.com", out.User.Email)
	}
	if out.User.Role != coreauth.RoleEditor {
		t.Errorf("role must not change: expected %q, got %q", coreauth.RoleEditor, out.User.Role)
	}
}

// TestUpdateUser_LastAdminCannotSelfDemote verifies that the last admin cannot
// demote themselves, which would leave the system with no admins.
func TestUpdateUser_LastAdminCannotSelfDemote(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	admin, err := svc.CreateUser("admin", "admin@example.com", "pass", coreauth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = uc.Execute(context.Background(), UpdateUserInput{
		ID:               admin.ID,
		Username:         admin.Username,
		Email:            admin.Email,
		Role:             coreauth.RoleViewer,
		RequesterIsAdmin: true,
	})
	if !errors.Is(err, coreauth.ErrLastAdminCannotBeDemoted) {
		t.Errorf("expected ErrLastAdminCannotBeDemoted, got: %v", err)
	}
}

// TestUpdateUser_AdminCanBeDemotedWhenAnotherExists verifies that an admin can
// lose their role as long as at least one other admin remains.
func TestUpdateUser_AdminCanBeDemotedWhenAnotherExists(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	admin1, _ := svc.CreateUser("admin1", "admin1@example.com", "pass", coreauth.RoleAdmin)
	_, _ = svc.CreateUser("admin2", "admin2@example.com", "pass", coreauth.RoleAdmin)

	out, err := uc.Execute(context.Background(), UpdateUserInput{
		ID:               admin1.ID,
		Username:         admin1.Username,
		Email:            admin1.Email,
		Role:             coreauth.RoleViewer,
		RequesterIsAdmin: true,
	})
	if err != nil {
		t.Fatalf("expected demotion to succeed, got: %v", err)
	}
	if out.User.Role != coreauth.RoleViewer {
		t.Errorf("expected role %q, got %q", coreauth.RoleViewer, out.User.Role)
	}
}

// TestUpdateUser_AdminInvalidRole checks that an admin supplying an unknown role
// gets a validation error rather than storing garbage.
func TestUpdateUser_AdminInvalidRole(t *testing.T) {
	uc, svc := setupUpdateUserUseCase(t)

	user, err := svc.CreateUser("alice", "alice@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = uc.Execute(context.Background(), UpdateUserInput{
		ID:               user.ID,
		Username:         user.Username,
		Email:            user.Email,
		Role:             "superuser", // not a valid role
		RequesterIsAdmin: true,
	})
	if err == nil {
		t.Fatal("expected validation error for invalid role, got nil")
	}
}

// TestCompleteTOTPLoginUseCase_AuthDisabled verifies the use case refuses to
// run (rather than nil-pointer-dereferencing into AuthService) when auth is
// disabled, matching every other use case's ErrAuthDisabled guard.
func TestCompleteTOTPLoginUseCase_AuthDisabled(t *testing.T) {
	uc := NewCompleteTOTPLoginUseCase(nil)

	_, err := uc.Execute(context.Background(), CompleteTOTPLoginInput{
		LoginChallengeToken: "token",
		Code:                "123456",
	})
	if !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("expected ErrAuthDisabled, got %v", err)
	}
}

// TestTOTPUseCases_AuthDisabled verifies every self-service TOTP use case
// refuses to run when auth is disabled, matching every other use case's
// ErrAuthDisabled guard.
func TestTOTPUseCases_AuthDisabled(t *testing.T) {
	if _, err := NewStartTOTPSetupUseCase(nil).Execute(context.Background(), StartTOTPSetupInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("StartTOTPSetupUseCase: expected ErrAuthDisabled, got %v", err)
	}
	if _, err := NewConfirmTOTPSetupUseCase(nil).Execute(context.Background(), ConfirmTOTPSetupInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("ConfirmTOTPSetupUseCase: expected ErrAuthDisabled, got %v", err)
	}
	if err := NewDisableTOTPUseCase(nil).Execute(context.Background(), DisableTOTPInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("DisableTOTPUseCase: expected ErrAuthDisabled, got %v", err)
	}
	if _, err := NewGetTOTPStatusUseCase(nil).Execute(context.Background(), GetTOTPStatusInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("GetTOTPStatusUseCase: expected ErrAuthDisabled, got %v", err)
	}
}

// TestDeleteUser_RemovesFavoritesForUser verifies that deleting a user cascades
// to clean up their favorites.db rows, even though sessions.db does not have
// the same cleanup today (deliberately not copying that gap, see plans/favorites.md).
func TestDeleteUser_RemovesFavoritesForUser(t *testing.T) {
	store, err := coreauth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close user store: %v", err)
		}
	})
	userSvc := coreauth.NewUserService(store)
	resolver, err := coreauth.NewUserResolver(userSvc)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}

	favoritesStore, err := favorites.NewFavoritesStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() {
		if err := favoritesStore.Close(); err != nil {
			t.Errorf("Close favorites store: %v", err)
		}
	})

	user, err := userSvc.CreateUser("bob", "bob@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := favoritesStore.Add(user.ID, "page-1"); err != nil {
		t.Fatalf("failed to seed favorite: %v", err)
	}

	uc := NewDeleteUserUseCase(userSvc, resolver, favoritesStore, slog.Default())
	if err := uc.Execute(context.Background(), DeleteUserInput{ID: user.ID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	ids, err := favoritesStore.ListPageIDsForUser(user.ID)
	if err != nil {
		t.Fatalf("ListPageIDsForUser: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected favorites for deleted user to be cleaned up, got %v", ids)
	}
}
