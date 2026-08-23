package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	coreauth "github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/favorites"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/usersettings"
)

func metricsBody(t *testing.T, metrics *httpmetrics.HTTPMetrics) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.HTTPHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected metrics endpoint to return 200, got %d", rec.Code)
	}
	return rec.Body.String()
}

// testTOTPEncryptionKey is a throwaway 32-byte key for TOTPService in tests —
// it never needs to match production, only to satisfy NewTOTPService's
// minimum-length requirement.
const testTOTPEncryptionKey = "test-totp-encryption-key-32byte!"

// setupAuthUseCases builds a real AuthService (backed by temp-dir SQLite
// stores, matching internal/core/auth's own test fixtures) together with a
// fresh metrics registry, for exercising the wiki-layer use cases end to end.
func setupAuthUseCases(t *testing.T, withTOTP bool) (*coreauth.AuthService, *coreauth.UserService, *httpmetrics.HTTPMetrics) {
	t.Helper()

	userStore, err := coreauth.NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore: %v", err)
	}
	t.Cleanup(func() {
		if err := userStore.Close(); err != nil {
			t.Errorf("Close user store: %v", err)
		}
	})
	userSvc := coreauth.NewUserService(userStore)

	sessionStore, err := coreauth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour*7)

	var totpSvc *coreauth.TOTPService
	if withTOTP {
		totpSvc, err = coreauth.NewTOTPService([]byte(testTOTPEncryptionKey))
		if err != nil {
			t.Fatalf("NewTOTPService: %v", err)
		}
	}

	authSvc := coreauth.NewAuthService(userSvc, sessions, totpSvc)
	return authSvc, userSvc, httpmetrics.NewHTTPMetrics("test")
}

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
	userSvcFn := func() *coreauth.UserService { return userSvc }
	resolver, err := coreauth.NewUserResolver(userSvcFn)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}
	return NewUpdateUserUseCase(userSvcFn, resolver, slog.Default()), userSvc
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

// TestLoginUseCase_EmitsSuccessMetric verifies that a successful password
// login (no TOTP) records both the login-attempt outcome and the
// session-issued event.
func TestLoginUseCase_EmitsSuccessMetric(t *testing.T) {
	authSvc, userSvc, metrics := setupAuthUseCases(t, false)
	if _, err := userSvc.CreateUser("alice", "alice@example.com", "securepass", coreauth.RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	uc := NewLoginUseCase(authSvc, metrics)
	if _, err := uc.Execute(context.Background(), LoginInput{Identifier: "alice", Password: "securepass"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_auth_login_attempts_total{outcome="success"} 1`) {
		t.Fatalf("expected login success metric, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_auth_sessions_total{event="issued"} 1`) {
		t.Fatalf("expected session issued metric, got: %s", body)
	}
}

// TestLoginUseCase_EmitsLockedMetric verifies that once the per-user failure
// threshold trips, a subsequent attempt is recorded under outcome="locked"
// (distinct from the "invalid_credentials" outcome of the failures that
// triggered the lock itself — see loginAttemptTracker.recordAttempt).
func TestLoginUseCase_EmitsLockedMetric(t *testing.T) {
	authSvc, userSvc, metrics := setupAuthUseCases(t, false)
	if _, err := userSvc.CreateUser("alice", "alice@example.com", "securepass", coreauth.RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	uc := NewLoginUseCase(authSvc, metrics)
	const maxFailures = 5
	for i := 0; i < maxFailures; i++ {
		if _, err := uc.Execute(context.Background(), LoginInput{Identifier: "alice", Password: "wrong"}); err == nil {
			t.Fatalf("attempt %d: expected an error for wrong password", i)
		}
	}

	_, err := uc.Execute(context.Background(), LoginInput{Identifier: "alice", Password: "wrong"})
	if !errors.Is(err, coreauth.ErrUserAccountLocked) {
		t.Fatalf("expected ErrUserAccountLocked once locked, got: %v", err)
	}

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_auth_login_attempts_total{outcome="locked"} 1`) {
		t.Fatalf("expected locked outcome metric, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_auth_login_attempts_total{outcome="invalid_credentials"} 5`) {
		t.Fatalf("expected 5 invalid_credentials attempts before the lock, got: %s", body)
	}
}

// TestLogoutUseCase_EmitsSessionRevokedMetric verifies a normal logout after a
// successful login records the session as revoked.
func TestLogoutUseCase_EmitsSessionRevokedMetric(t *testing.T) {
	authSvc, userSvc, metrics := setupAuthUseCases(t, false)
	if _, err := userSvc.CreateUser("alice", "alice@example.com", "securepass", coreauth.RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	loginUC := NewLoginUseCase(authSvc, metrics)
	out, err := loginUC.Execute(context.Background(), LoginInput{Identifier: "alice", Password: "securepass"})
	if err != nil {
		t.Fatalf("Login Execute: %v", err)
	}

	logoutUC := NewLogoutUseCase(authSvc, metrics)
	if err := logoutUC.Execute(context.Background(), LogoutInput{RefreshToken: out.Token.RefreshToken}); err != nil {
		t.Fatalf("Logout Execute: %v", err)
	}

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_auth_sessions_total{event="revoked"} 1`) {
		t.Fatalf("expected session revoked metric, got: %s", body)
	}
}

// TestCompleteTOTPLoginUseCase_EmitsVerificationMetric verifies a successful
// TOTP login handshake records both the verification outcome and the
// resulting session-issued event.
func TestCompleteTOTPLoginUseCase_EmitsVerificationMetric(t *testing.T) {
	authSvc, userSvc, metrics := setupAuthUseCases(t, true)
	user, err := userSvc.CreateUser("alice", "alice@example.com", "securepass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	generated, err := authSvc.StartTOTPSetup(user.ID, "securepass")
	if err != nil {
		t.Fatalf("StartTOTPSetup: %v", err)
	}
	setupCode, err := totp.GenerateCode(generated.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if _, err := authSvc.ConfirmTOTPSetup(user.ID, setupCode, ""); err != nil {
		t.Fatalf("ConfirmTOTPSetup: %v", err)
	}

	loginUC := NewLoginUseCase(authSvc, metrics)
	loginOut, err := loginUC.Execute(context.Background(), LoginInput{Identifier: "alice", Password: "securepass"})
	if err != nil {
		t.Fatalf("Login Execute: %v", err)
	}
	if !loginOut.Token.RequiresTOTP {
		t.Fatal("expected RequiresTOTP = true once TOTP is enabled")
	}

	loginCode, err := totp.GenerateCode(generated.Secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	completeUC := NewCompleteTOTPLoginUseCase(authSvc, metrics)
	if _, err := completeUC.Execute(context.Background(), CompleteTOTPLoginInput{
		LoginChallengeToken: loginOut.Token.LoginChallengeToken,
		Code:                loginCode,
	}); err != nil {
		t.Fatalf("CompleteTOTPLogin Execute: %v", err)
	}

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_auth_totp_verifications_total{result="success"} 1`) {
		t.Fatalf("expected totp verification success metric, got: %s", body)
	}
	if !strings.Contains(body, `leafwiki_auth_sessions_total{event="issued"} 1`) {
		t.Fatalf("expected session issued metric, got: %s", body)
	}
}

// TestCompleteTOTPLoginUseCase_AuthDisabled verifies the use case refuses to
// run (rather than nil-pointer-dereferencing into AuthService) when auth is
// disabled, matching every other use case's ErrAuthDisabled guard.
func TestCompleteTOTPLoginUseCase_AuthDisabled(t *testing.T) {
	uc := NewCompleteTOTPLoginUseCase(nil, nil)

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
	if _, err := NewConfirmTOTPSetupUseCase(nil, nil).Execute(context.Background(), ConfirmTOTPSetupInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("ConfirmTOTPSetupUseCase: expected ErrAuthDisabled, got %v", err)
	}
	if err := NewDisableTOTPUseCase(nil, nil).Execute(context.Background(), DisableTOTPInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("DisableTOTPUseCase: expected ErrAuthDisabled, got %v", err)
	}
	if _, err := NewGetTOTPStatusUseCase(nil).Execute(context.Background(), GetTOTPStatusInput{}); !errors.Is(err, ErrAuthDisabled) {
		t.Fatalf("GetTOTPStatusUseCase: expected ErrAuthDisabled, got %v", err)
	}
}

// TestGetUsersUseCase_DoesNotExposeTOTPSecretOrRecoveryCodes verifies that the
// admin user list only ever surfaces the TOTPEnabled flag: the encrypted TOTP
// secret and recovery code hashes on the internal User must never reach the
// PublicUser the admin API serializes to JSON.
func TestGetUsersUseCase_DoesNotExposeTOTPSecretOrRecoveryCodes(t *testing.T) {
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
	user, err := userSvc.CreateUser("alice", "alice@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	const secretMarker = "super-secret-encrypted-totp-blob"
	const hashMarker = "super-secret-recovery-code-hash"
	if err := store.EnableTOTP(user.ID, secretMarker, []string{hashMarker}); err != nil {
		t.Fatalf("EnableTOTP: %v", err)
	}

	uc := NewGetUsersUseCase(func() *coreauth.UserService { return userSvc })
	out, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var found *coreauth.PublicUser
	for _, u := range out.Users {
		if u.ID == user.ID {
			found = u
		}
	}
	if found == nil {
		t.Fatal("expected the created user to be present in GetUsers output")
	}
	if !found.TOTPEnabled {
		t.Error("expected TOTPEnabled = true")
	}

	data, err := json.Marshal(found)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), secretMarker) || strings.Contains(string(data), hashMarker) {
		t.Fatalf("PublicUser JSON leaked TOTP secret material: %s", data)
	}
}

// TestDeleteUser_RemovesFavoritesForUser verifies that deleting a user cascades
// to clean up their favorites.db, usersettings.db, and api_keys.db rows, even
// though sessions.db does not have the same cleanup today (deliberately not
// copying that gap, see plans/favorites.md).
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
	userSvcFn := func() *coreauth.UserService { return userSvc }
	resolver, err := coreauth.NewUserResolver(userSvcFn)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}

	favoritesStore, err := favorites.NewFavoritesStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() {
		if err := favoritesStore.Close(); err != nil {
			t.Errorf("Close favorites store: %v", err)
		}
	})

	settingsStore, err := usersettings.NewUserSettingsStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() {
		if err := settingsStore.Close(); err != nil {
			t.Errorf("Close user settings store: %v", err)
		}
	})
	settingsSvc := usersettings.NewUserSettingsService(settingsStore)

	sessionStore, err := coreauth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authSvc := coreauth.NewAuthService(userSvc, sessions, nil)

	apiKeyStore, err := coreauth.NewAPIKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAPIKeyStore: %v", err)
	}
	t.Cleanup(func() {
		if err := apiKeyStore.Close(); err != nil {
			t.Errorf("Close api key store: %v", err)
		}
	})
	apiKeySvc := coreauth.NewAPIKeyService(apiKeyStore, authSvc)

	user, err := userSvc.CreateUser("bob", "bob@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := favoritesStore.Add(user.ID, "page-1"); err != nil {
		t.Fatalf("failed to seed favorite: %v", err)
	}
	autoSave := false
	if _, err := settingsSvc.Update(user.ID, usersettings.UserSettingsPatch{AutoSave: &autoSave}); err != nil {
		t.Fatalf("failed to seed user settings: %v", err)
	}
	if _, _, err := apiKeySvc.CreateAPIKey(coreauth.CreateAPIKeyParams{Name: "bob's key", UserID: user.ID, CreatedBy: "admin"}); err != nil {
		t.Fatalf("failed to seed api key: %v", err)
	}

	uc := NewDeleteUserUseCase(userSvcFn, resolver, favoritesStore, settingsSvc, apiKeySvc, slog.Default())
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

	settings, err := settingsSvc.Get(user.ID)
	if err != nil {
		t.Fatalf("Get user settings: %v", err)
	}
	if settings.AutoSave != true {
		t.Errorf("expected user settings for deleted user to be cleaned up (back to default AutoSave=true), got %v", settings.AutoSave)
	}

	keys, err := apiKeySvc.ListAPIKeys()
	if err != nil {
		t.Fatalf("ListAPIKeys: %v", err)
	}
	for _, k := range keys {
		if k.UserID == user.ID {
			t.Errorf("expected api keys for deleted user to be cleaned up, found key %s still owned by %s", k.ID, k.UserID)
		}
	}
}

// TestDeleteUser_APIKeysDisabled_SkipsCleanupWithoutError verifies the
// nil-guard around API key cleanup: when API key management is disabled
// (apiKeys is nil, the default), deleting a user must not panic or error.
func TestDeleteUser_APIKeysDisabled_SkipsCleanupWithoutError(t *testing.T) {
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
	userSvcFn := func() *coreauth.UserService { return userSvc }
	resolver, err := coreauth.NewUserResolver(userSvcFn)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}

	favoritesStore, err := favorites.NewFavoritesStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewFavoritesStore: %v", err)
	}
	t.Cleanup(func() {
		if err := favoritesStore.Close(); err != nil {
			t.Errorf("Close favorites store: %v", err)
		}
	})

	settingsStore, err := usersettings.NewUserSettingsStore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewUserSettingsStore: %v", err)
	}
	t.Cleanup(func() {
		if err := settingsStore.Close(); err != nil {
			t.Errorf("Close user settings store: %v", err)
		}
	})
	settingsSvc := usersettings.NewUserSettingsService(settingsStore)

	user, err := userSvc.CreateUser("carol", "carol@example.com", "pass", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	uc := NewDeleteUserUseCase(userSvcFn, resolver, favoritesStore, settingsSvc, nil, slog.Default())
	if err := uc.Execute(context.Background(), DeleteUserInput{ID: user.ID}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}

// TestGetUsersUseCase_ReflectsLiveRestore is the regression test for
// "User-Management Routes Go Stale After Live Restore": wiki.go wires this use
// case straight from w.user (the UserService captured once at boot), bypassing
// AuthService entirely. A live restore's AuthService.ReplaceUserStore swaps
// AuthService's own internal pointer and closes the old UserService's store —
// but never touches whatever a caller captured directly, so a use case built
// that way is left holding a closed store forever after a restore.
func TestGetUsersUseCase_ReflectsLiveRestore(t *testing.T) {
	preDir := t.TempDir()
	preStore, err := coreauth.NewUserStore(preDir)
	if err != nil {
		t.Fatalf("NewUserStore(pre): %v", err)
	}
	preSvc := coreauth.NewUserService(preStore)

	sessionStore, err := coreauth.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := coreauth.NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authSvc := coreauth.NewAuthService(preSvc, sessions, nil)
	t.Cleanup(func() { _ = authSvc.Close() })

	uc := NewGetUsersUseCase(authSvc.UserService)

	postDir := t.TempDir()
	postStore, err := coreauth.NewUserStore(postDir)
	if err != nil {
		t.Fatalf("NewUserStore(post): %v", err)
	}
	postSvc := coreauth.NewUserService(postStore)
	postUser, err := postSvc.CreateUser("post-restore-admin", "post-restore-admin@example.com", "password123", coreauth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser(post): %v", err)
	}
	if err := postStore.Close(); err != nil {
		t.Fatalf("Close(postStore): %v", err)
	}

	// Simulates what a live restore does to AuthService.
	if err := authSvc.ReplaceUserStore(postDir); err != nil {
		t.Fatalf("ReplaceUserStore: %v", err)
	}

	out, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("GetUsersUseCase.Execute failed after a live restore swapped the user store: %v", err)
	}
	found := false
	for _, u := range out.Users {
		if u.ID == postUser.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("GetUsersUseCase did not see the post-restore user set — it's still bound to the pre-restore UserService captured at construction time")
	}
}
