package auth

import (
	"errors"
	"sync"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/test_utils"
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

func TestUserService_UpdateUser_EmptyRolePreservesExistingRole(t *testing.T) {
	service := setupTestUserService(t)

	user, _ := service.CreateUser("bob", "bob@example.com", "initial", RoleEditor)

	updated, err := service.UpdateUser(user.ID, "bobnew", "bobnew@example.com", "", "")
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	if updated.Username != "bobnew" || updated.Email != "bobnew@example.com" {
		t.Errorf("Update did not persist profile fields")
	}
	if updated.Role != RoleEditor {
		t.Errorf("expected role %q, got %q", RoleEditor, updated.Role)
	}
}
func TestUserService_UpdateUser_LastAdminCannotBeDemoted(t *testing.T) {
	service := setupTestUserService(t)

	admin, _ := service.CreateUser("admin", "admin@example.com", "pass", RoleAdmin)

	_, err := service.UpdateUser(admin.ID, admin.Username, admin.Email, "", RoleViewer)
	if err != ErrLastAdminCannotBeDemoted {
		t.Errorf("expected ErrLastAdminCannotBeDemoted, got: %v", err)
	}
}

func TestUserService_UpdateUser_AdminCanBeDemotedWhenAnotherAdminExists(t *testing.T) {
	service := setupTestUserService(t)

	admin1, _ := service.CreateUser("admin1", "admin1@example.com", "pass", RoleAdmin)
	_, _ = service.CreateUser("admin2", "admin2@example.com", "pass", RoleAdmin)

	updated, err := service.UpdateUser(admin1.ID, admin1.Username, admin1.Email, "", RoleViewer)
	if err != nil {
		t.Fatalf("expected demotion to succeed with two admins, got: %v", err)
	}
	if updated.Role != RoleViewer {
		t.Errorf("expected role %q, got %q", RoleViewer, updated.Role)
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

	err := service.InitDefaultAdmin("", "", "password123")
	if err != nil {
		t.Errorf("InitDefaultAdmin failed: %v", err)
	}

	users, err := service.GetUsers()
	if err != nil || len(users) != 1 || users[0].Username != "admin" || users[0].Email != "admin@localhost" {
		t.Errorf("Expected default admin user, got: %+v", users)
	}
}

func TestUserService_InitDefaultAdmin_UsesGivenUsernameAndEmail(t *testing.T) {
	store, _ := NewUserStore(t.TempDir())
	service := NewUserService(store)

	err := service.InitDefaultAdmin("root", "root@example.com", "password123")
	if err != nil {
		t.Errorf("InitDefaultAdmin failed: %v", err)
	}

	users, err := service.GetUsers()
	if err != nil || len(users) != 1 || users[0].Username != "root" || users[0].Email != "root@example.com" {
		t.Errorf("Expected admin user with custom username/email, got: %+v", users)
	}
}

func TestUserService_InitDefaultAdmin_PasswordTooShort_ReturnsError(t *testing.T) {
	store, _ := NewUserStore(t.TempDir())
	service := NewUserService(store)

	err := service.InitDefaultAdmin("root", "root@example.com", "short")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("InitDefaultAdmin() error = %v, want ErrPasswordTooShort", err)
	}

	users, err := service.GetUsers()
	if err != nil || len(users) != 0 {
		t.Errorf("Expected no admin user to be created, got: %+v", users)
	}
}

func TestUserService_InitDefaultAdmin_SkipsWhenAdminAlreadyExists(t *testing.T) {
	store, _ := NewUserStore(t.TempDir())
	service := NewUserService(store)

	if _, err := service.CreateUser("admin", "admin@localhost", "existing", "admin"); err != nil {
		t.Fatalf("Failed to create initial admin user: %v", err)
	}

	err := service.InitDefaultAdmin("root", "root@example.com", "new")
	if err != nil {
		t.Errorf("InitDefaultAdmin failed: %v", err)
	}

	users, err := service.GetUsers()
	if err != nil || len(users) != 1 || users[0].Username != "admin" || users[0].Email != "admin@localhost" {
		t.Errorf("Expected existing admin user to be left untouched, got: %+v", users)
	}
}

func TestUserService_ResetAdminUserPassword(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	// Create initial admin user
	_, err := service.CreateUser("admin", "admin@example.com", "oldpassword", "admin")
	if err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	// Reset admin password
	adminUser, err := service.ResetAdminUserPassword("", "")
	if err != nil {
		t.Fatalf("ResetAdminUserPassword failed: %v", err)
	}

	if adminUser.Username != "admin" {
		t.Errorf("Expected username 'admin', got: %s", adminUser.Username)
	}

	if adminUser.Password == "" {
		t.Errorf("Expected a new password to be generated, got empty string")
	}

	if adminUser.Password == "oldpassword" {
		t.Errorf("Expected password to be different from old password")
	}

	// Verify we can log in with the new password
	_, err = service.GetUserByEmailOrUsernameAndPassword("admin", adminUser.Password)
	if err != nil {
		t.Errorf("Failed to login with new password: %v", err)
	}

	// Verify old password no longer works
	_, err = service.GetUserByEmailOrUsernameAndPassword("admin", "oldpassword")
	if err != ErrUserInvalidCredentials {
		t.Errorf("Expected ErrUserInvalidCredentials for old password, got: %v", err)
	}
}

func TestUserService_ResetAdminUserPassword_NoAdmin(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	// Don't create an admin user first - test should create one

	// Reset admin password (should create new admin)
	adminUser, err := service.ResetAdminUserPassword("", "")
	if err != nil {
		t.Fatalf("ResetAdminUserPassword failed: %v", err)
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
	_, err = service.GetUserByEmailOrUsernameAndPassword("admin", adminUser.Password)
	if err != nil {
		t.Errorf("Failed to login with new password: %v", err)
	}
}

func TestUserService_ResetAdminUserPassword_NoAdmin_UsesGivenUsernameAndEmail(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	// Don't create an admin user first - test should create one with the given identity

	adminUser, err := service.ResetAdminUserPassword("root", "root@example.com")
	if err != nil {
		t.Fatalf("ResetAdminUserPassword failed: %v", err)
	}

	if adminUser.Username != "root" {
		t.Errorf("Expected username 'root', got: %s", adminUser.Username)
	}

	if adminUser.Email != "root@example.com" {
		t.Errorf("Expected email 'root@example.com', got: %s", adminUser.Email)
	}

	// Verify we can log in with the new password
	_, err = service.GetUserByEmailOrUsernameAndPassword("root", adminUser.Password)
	if err != nil {
		t.Errorf("Failed to login with new password: %v", err)
	}
}

// Regression tests for the bug where UserService collapsed a suspended
// store's auth_user_store_unavailable LocalizedError (see
// errUserStoreUnavailable / UserStore.suspend) down to the generic
// ErrUserNotFound, so a request landing in a live restore's brief suspend
// window saw a confusing "user not found" instead of "restore in progress".

func TestUserService_GetUserByID_StoreSuspended_ReturnsStoreUnavailable(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.CreateUser("suspend-user", "suspend-user@example.com", "password1", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	_, err = service.GetUserByID(user.ID)
	if err == nil {
		t.Fatal("expected an error for a suspended store")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected the store-unavailable error, not ErrUserNotFound: %v", err)
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_user_store_unavailable" {
		t.Fatalf("expected auth_user_store_unavailable, got %v", err)
	}
}

func TestUserService_GetUserByID_UnknownID_ReturnsErrUserNotFound(t *testing.T) {
	service := setupTestUserService(t)

	_, err := service.GetUserByID("does-not-exist")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for a genuinely unknown ID, got %v", err)
	}
}

func TestUserService_GetUserByUsername_StoreSuspended_ReturnsStoreUnavailable(t *testing.T) {
	service := setupTestUserService(t)

	_, err := service.CreateUser("suspend-user2", "suspend-user2@example.com", "password1", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	_, err = service.GetUserByUsername("suspend-user2")
	if err == nil {
		t.Fatal("expected an error for a suspended store")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected the store-unavailable error, not ErrUserNotFound: %v", err)
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_user_store_unavailable" {
		t.Fatalf("expected auth_user_store_unavailable, got %v", err)
	}
}

func TestUserService_GetUserByIdentifier_StoreSuspended_ReturnsStoreUnavailable(t *testing.T) {
	service := setupTestUserService(t)

	_, err := service.CreateUser("suspend-user3", "suspend-user3@example.com", "password1", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	_, err = service.GetUserByIdentifier("suspend-user3")
	if err == nil {
		t.Fatal("expected an error for a suspended store")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected the store-unavailable error, not ErrUserNotFound: %v", err)
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_user_store_unavailable" {
		t.Fatalf("expected auth_user_store_unavailable, got %v", err)
	}
}

func TestUserService_DeleteUser_StoreSuspended_ReturnsStoreUnavailable(t *testing.T) {
	service := setupTestUserService(t)

	user, err := service.CreateUser("suspend-user4", "suspend-user4@example.com", "password1", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	err = service.DeleteUser(user.ID)
	if err == nil {
		t.Fatal("expected an error for a suspended store")
	}
	if errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected the store-unavailable error, not ErrUserNotFound: %v", err)
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_user_store_unavailable" {
		t.Fatalf("expected auth_user_store_unavailable, got %v", err)
	}
}

func TestUserService_GetOrCreateRemoteUser_CreatesNewUser(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	user, err := service.GetOrCreateRemoteUser("carol", "carol@example.com", RoleViewer)
	if err != nil {
		t.Fatalf("GetOrCreateRemoteUser failed: %v", err)
	}

	if user.Username != "carol" || user.Email != "carol@example.com" || user.Role != RoleViewer {
		t.Errorf("User not created with expected data: %+v", user)
	}
}

func TestUserService_GetOrCreateRemoteUser_ReturnsExistingUserOnSecondCall(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	first, err := service.GetOrCreateRemoteUser("carol", "carol@example.com", RoleViewer)
	if err != nil {
		t.Fatalf("first GetOrCreateRemoteUser failed: %v", err)
	}

	second, err := service.GetOrCreateRemoteUser("carol", "carol@example.com", RoleViewer)
	if err != nil {
		t.Fatalf("second GetOrCreateRemoteUser failed: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("expected same user on second call, got different IDs: %s vs %s", first.ID, second.ID)
	}

	users, err := service.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	count := 0
	for _, u := range users {
		if u.Username == "carol" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one 'carol' user, got %d", count)
	}
}

func TestUserService_GetOrCreateRemoteUser_ReturnsExistingUserByEmail(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	preexisting, err := service.CreateUser("dave", "dave@example.com", "secure", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Proxy asserts dave's email, not his username; GetOrCreateRemoteUser must
	// resolve to the pre-existing account rather than creating a duplicate.
	resolved, err := service.GetOrCreateRemoteUser("dave@example.com", "", RoleViewer)
	if err != nil {
		t.Fatalf("GetOrCreateRemoteUser failed: %v", err)
	}

	if resolved.ID != preexisting.ID {
		t.Errorf("expected to resolve pre-existing user %s, got %s", preexisting.ID, resolved.ID)
	}
	if resolved.Role != RoleEditor {
		t.Errorf("expected existing role to be preserved, got %s", resolved.Role)
	}
}

func TestUserService_GetOrCreateRemoteUser_SynthesizesPlaceholderEmailWhenNoneGiven(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	user, err := service.GetOrCreateRemoteUser("erin", "", RoleViewer)
	if err != nil {
		t.Fatalf("GetOrCreateRemoteUser failed: %v", err)
	}

	want := "erin@remote-user.invalid"
	if user.Email != want {
		t.Errorf("expected synthesized email %q, got %q", want, user.Email)
	}
}

func TestUserService_GetOrCreateRemoteUser_InvalidRolePropagates(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	_, err := service.GetOrCreateRemoteUser("frank", "frank@example.com", "guest")
	if err != ErrUserInvalidRole {
		t.Errorf("expected ErrUserInvalidRole, got: %v", err)
	}
}

func TestUserService_LookupRemoteUserByIdentifier_StoreErrorPropagates(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	// Simulate a transient store failure (e.g. a DB hiccup), not a genuine
	// "user doesn't exist yet". GetOrCreateRemoteUser treats ErrUserNotFound
	// as the auto-create trigger, so this lookup must surface the real store
	// error instead of being misclassified as ErrUserNotFound — otherwise a
	// transient failure would cause a spurious account to be provisioned.
	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	_, err := service.lookupRemoteUserByIdentifier("henry")
	if err == nil || errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected the underlying store error, not ErrUserNotFound, got: %v", err)
	}
}

func TestUserService_GetOrCreateRemoteUser_EmailConflictWithDifferentUserReturnsDistinctError(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	// A pre-existing, unrelated user already owns this email.
	_, err := service.CreateUser("dave", "shared@example.com", "secure", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// A brand-new identifier ("newperson") whose asserted email collides with
	// dave's — not a race on the same identifier, so this must not silently
	// resolve to dave (account confusion) nor hang the caller with a generic
	// "not found" after the failed create.
	_, err = service.GetOrCreateRemoteUser("newperson", "shared@example.com", RoleViewer)
	if !errors.Is(err, ErrRemoteUserEmailConflict) {
		t.Errorf("expected ErrRemoteUserEmailConflict, got: %v", err)
	}

	// "newperson" must not have been created as a side effect.
	if _, err := service.GetUserByUsername("newperson"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected 'newperson' to not exist, got err: %v", err)
	}
}

func TestUserService_GetOrCreateRemoteUser_ConcurrentCallsCreateExactlyOneUser(t *testing.T) {
	service := setupTestUserService(t)
	defer test_utils.WrapCloseWithErrorCheck(service.Close, t)

	const goroutines = 20
	results := make(chan *User, goroutines)
	errs := make(chan error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			user, err := service.GetOrCreateRemoteUser("grace", "grace@example.com", RoleViewer)
			if err != nil {
				errs <- err
				return
			}
			results <- user
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("GetOrCreateRemoteUser returned error: %v", err)
	}

	var firstID string
	count := 0
	for user := range results {
		count++
		if firstID == "" {
			firstID = user.ID
		} else if user.ID != firstID {
			t.Errorf("expected all concurrent calls to resolve to the same user ID, got %s and %s", firstID, user.ID)
		}
	}
	if count != goroutines {
		t.Errorf("expected %d results, got %d", goroutines, count)
	}

	users, err := service.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	graceCount := 0
	for _, u := range users {
		if u.Username == "grace" {
			graceCount++
		}
	}
	if graceCount != 1 {
		t.Errorf("expected exactly one 'grace' user to have been created, got %d", graceCount)
	}
}

func setupTestUserServiceWithEditorLimit(t *testing.T, limit int) *UserService {
	t.Helper()
	store, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to setup user store: %v", err)
	}
	service := NewUserService(store)
	service.SetEditorLimit(limit)
	return service
}

func TestUserService_CreateUser_BootstrapFirstAdminUnderLimitOne_Succeeds(t *testing.T) {
	service := setupTestUserServiceWithEditorLimit(t, 1)

	if err := service.InitDefaultAdmin("", "", "password123"); err != nil {
		t.Fatalf("InitDefaultAdmin failed under editor limit 1: %v", err)
	}
}

func TestUserService_CreateUser_EditorLimitReached_ReturnsError(t *testing.T) {
	service := setupTestUserServiceWithEditorLimit(t, 1)

	if _, err := service.CreateUser("admin", "admin@example.com", "secure123", RoleAdmin); err != nil {
		t.Fatalf("CreateUser (first editor) failed: %v", err)
	}

	if _, err := service.CreateUser("bob", "bob@example.com", "secure123", RoleEditor); !errors.Is(err, ErrEditorLimitReached) {
		t.Errorf("Expected ErrEditorLimitReached for second editor under limit 1, got: %v", err)
	}
}

func TestUserService_CreateUser_ViewerRoleNeverBlockedByLimit(t *testing.T) {
	service := setupTestUserServiceWithEditorLimit(t, 1)

	if _, err := service.CreateUser("admin", "admin@example.com", "secure123", RoleAdmin); err != nil {
		t.Fatalf("CreateUser (admin) failed: %v", err)
	}

	if _, err := service.CreateUser("viewer1", "viewer1@example.com", "secure123", RoleViewer); err != nil {
		t.Errorf("Expected viewer creation to succeed regardless of editor limit, got: %v", err)
	}
	if _, err := service.CreateUser("viewer2", "viewer2@example.com", "secure123", RoleViewer); err != nil {
		t.Errorf("Expected a second viewer creation to succeed regardless of editor limit, got: %v", err)
	}
}

func TestUserService_CreateUser_UnlimitedByDefault(t *testing.T) {
	service := setupTestUserService(t)

	for i, username := range []string{"admin", "editor1", "editor2"} {
		role := RoleAdmin
		if i > 0 {
			role = RoleEditor
		}
		if _, err := service.CreateUser(username, username+"@example.com", "secure123", role); err != nil {
			t.Fatalf("CreateUser(%q) failed with default (unlimited) editor limit: %v", username, err)
		}
	}
}

func TestUserService_UpdateUser_PromoteViewerToEditor_EditorLimitReached_ReturnsError(t *testing.T) {
	service := setupTestUserServiceWithEditorLimit(t, 1)

	if _, err := service.CreateUser("admin", "admin@example.com", "secure123", RoleAdmin); err != nil {
		t.Fatalf("CreateUser (admin) failed: %v", err)
	}
	viewer, err := service.CreateUser("viewer", "viewer@example.com", "secure123", RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser (viewer) failed: %v", err)
	}

	if _, err := service.UpdateUser(viewer.ID, viewer.Username, viewer.Email, "", RoleEditor); !errors.Is(err, ErrEditorLimitReached) {
		t.Errorf("Expected ErrEditorLimitReached when promoting viewer to editor beyond limit 1, got: %v", err)
	}
}

func TestUserService_UpdateUser_EditorToAdmin_NotBlockedByLimit(t *testing.T) {
	service := setupTestUserServiceWithEditorLimit(t, 2)

	if _, err := service.CreateUser("admin", "admin@example.com", "secure123", RoleAdmin); err != nil {
		t.Fatalf("CreateUser (admin) failed: %v", err)
	}
	editor, err := service.CreateUser("editor", "editor@example.com", "secure123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser (editor) failed: %v", err)
	}

	// Both editor slots are taken (admin+editor); promoting the existing
	// editor to admin must stay neutral (2 editors before, 2 after), not be
	// blocked as if it were a third new editor.
	if _, err := service.UpdateUser(editor.ID, editor.Username, editor.Email, "", RoleAdmin); err != nil {
		t.Errorf("Expected editor->admin role change to stay neutral under the editor limit, got: %v", err)
	}
}
