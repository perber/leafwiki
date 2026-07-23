package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func setupTestAuthService(t *testing.T) *AuthService {
	t.Helper()
	store, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	userService := NewUserService(store)

	// Create test user
	_, err = userService.CreateUser("testuser", "test@example.com", "securepass", "admin")
	if err != nil {
		t.Fatal(err)
	}

	// Create Session store
	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	sessions := NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", 1*time.Hour, 24*time.Hour*7)
	authService := NewAuthService(userService, sessions, nil)
	return authService
}

// totpTestFixture wires an AuthService to a real TOTPService and a "testuser"
// with TOTP already enabled, for exercising the two-step login handshake.
type totpTestFixture struct {
	authService  *AuthService
	store        *UserStore
	totpService  *TOTPService
	userID       string
	plainSecret  string
	recoveryCode string // one of the plaintext recovery codes, valid until consumed
}

func setupTOTPTestFixture(t *testing.T) *totpTestFixture {
	t.Helper()
	store, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	userService := NewUserService(store)

	user, err := userService.CreateUser("testuser", "test@example.com", "securepass", "admin")
	if err != nil {
		t.Fatal(err)
	}

	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	totpService, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}

	generated, err := totpService.GenerateSecret(user.Email)
	if err != nil {
		t.Fatal(err)
	}
	plainSecret, err := totpService.decrypt(generated.EncryptedSecret)
	if err != nil {
		t.Fatal(err)
	}

	codes, hashes, err := totpService.GenerateRecoveryCodes(3)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.EnableTOTP(user.ID, generated.EncryptedSecret, hashes); err != nil {
		t.Fatal(err)
	}

	sessions := NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", 1*time.Hour, 24*time.Hour*7)
	authService := NewAuthService(userService, sessions, totpService)

	return &totpTestFixture{
		authService:  authService,
		store:        store,
		totpService:  totpService,
		userID:       user.ID,
		plainSecret:  plainSecret,
		recoveryCode: codes[0],
	}
}

func (f *totpTestFixture) currentCode(t *testing.T) string {
	t.Helper()
	code, err := totp.GenerateCode(f.plainSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate reference TOTP code: %v", err)
	}
	return code
}

func TestAuthService_Login_TOTPEnabled_ReturnsChallengeInsteadOfTokens(t *testing.T) {
	f := setupTOTPTestFixture(t)

	result, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if !result.RequiresTOTP {
		t.Fatal("expected RequiresTOTP = true for a TOTP-enabled user")
	}
	if result.LoginChallengeToken == "" {
		t.Fatal("expected a non-empty login challenge token")
	}
	if result.Token != "" || result.RefreshToken != "" {
		t.Fatal("expected no access/refresh tokens before the TOTP step completes")
	}
}

func TestAuthService_Login_TOTPEnabled_WrongPasswordStillRejected(t *testing.T) {
	f := setupTOTPTestFixture(t)

	if _, err := f.authService.Login("testuser", "wrong-password"); err != ErrUserInvalidCredentials {
		t.Fatalf("expected ErrUserInvalidCredentials, got %v", err)
	}
}

func TestAuthService_CompleteTOTPLogin_ValidCodeIssuesTokens(t *testing.T) {
	f := setupTOTPTestFixture(t)

	challenge, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	final, err := f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, f.currentCode(t))
	if err != nil {
		t.Fatalf("CompleteTOTPLogin failed: %v", err)
	}
	if final.Token == "" || final.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens after a valid TOTP code")
	}

	user, err := f.authService.ValidateToken(final.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}
}

func TestAuthService_CompleteTOTPLogin_InvalidCodeRejected(t *testing.T) {
	f := setupTOTPTestFixture(t)

	challenge, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, err = f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, "000000")
	if err == nil {
		t.Fatal("expected error for wrong TOTP code")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_invalid_code" {
		t.Fatalf("expected auth_totp_invalid_code, got %#v", err)
	}
}

func TestAuthService_CompleteTOTPLogin_ChallengeIsSingleUse(t *testing.T) {
	f := setupTOTPTestFixture(t)

	challenge, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	code := f.currentCode(t)
	if _, err := f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, code); err != nil {
		t.Fatalf("first CompleteTOTPLogin failed: %v", err)
	}

	// Replaying the same challenge token, even with a still-valid code, must fail.
	_, err = f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, code)
	if err == nil {
		t.Fatal("expected error when reusing an already-consumed login challenge")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_challenge_invalid" {
		t.Fatalf("expected auth_totp_challenge_invalid, got %#v", err)
	}
}

func TestAuthService_CompleteTOTPLogin_RecoveryCodeWorksOnceAndOnlyOnce(t *testing.T) {
	f := setupTOTPTestFixture(t)

	challenge1, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if _, err := f.authService.CompleteTOTPLogin(challenge1.LoginChallengeToken, f.recoveryCode); err != nil {
		t.Fatalf("CompleteTOTPLogin with recovery code failed: %v", err)
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if len(user.TOTPRecoveryCodeHashes) != 2 {
		t.Fatalf("expected 1 recovery code hash to be consumed, 2 remaining, got %d", len(user.TOTPRecoveryCodeHashes))
	}

	// A second, fresh login challenge must reject the now-consumed recovery code.
	challenge2, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("second Login failed: %v", err)
	}
	_, err = f.authService.CompleteTOTPLogin(challenge2.LoginChallengeToken, f.recoveryCode)
	if err == nil {
		t.Fatal("expected error when reusing an already-consumed recovery code")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_invalid_code" {
		t.Fatalf("expected auth_totp_invalid_code, got %#v", err)
	}
}

// TestAuthService_TOTPGuessing_CannotBypassLockoutViaRepeatedCorrectPasswordLogin
// is the regression test for the login-lockout bypass: Login() must not reset
// the shared per-user attempt counter while TOTP is still required, or an
// attacker who already knows the password could keep resetting the guess
// budget by resubmitting it, turning the 6-digit TOTP code into an
// unlimited-attempt guessing target. Alternating a correct-password login
// with a wrong TOTP guess must still hit the account lock within
// loginMaxFailures combined attempts.
func TestAuthService_TOTPGuessing_CannotBypassLockoutViaRepeatedCorrectPasswordLogin(t *testing.T) {
	f := setupTOTPTestFixture(t)

	locked := false
	for i := 0; i < 10; i++ {
		challenge, err := f.authService.Login("testuser", "securepass")
		if err != nil {
			if err == ErrUserAccountLocked {
				locked = true
				break
			}
			t.Fatalf("Login failed unexpectedly on iteration %d: %v", i, err)
		}

		_, err = f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, "000000")
		if err == ErrUserAccountLocked {
			locked = true
			break
		}
	}

	if !locked {
		t.Fatal("expected repeated correct-password logins interleaved with wrong TOTP guesses to eventually lock the account")
	}
}

// TestAuthService_CompleteTOTPLogin_ConcurrentRecoveryCodeUseOnlyOneSucceeds
// is the regression test for the recovery-code double-redemption race:
// concurrent login attempts presenting the same still-valid recovery code
// must not all succeed — the code must be consumable exactly once even under
// concurrency, not just when used sequentially. Reuses a single challenge
// token across all attempts (rather than one per attempt) since Login() no
// longer resets the shared lockout counter for TOTP-enabled accounts —
// several sequential Login() calls would otherwise trip the account lock
// before any CompleteTOTPLogin call runs, which is exactly the bypass that
// change closed.
func TestAuthService_CompleteTOTPLogin_ConcurrentRecoveryCodeUseOnlyOneSucceeds(t *testing.T) {
	f := setupTOTPTestFixture(t)

	challenge, err := f.authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Stay safely under loginMaxFailures so this test only exercises the
	// recovery-code race, not the (separately tested) account lockout.
	const attempts = 3
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.authService.CompleteTOTPLogin(challenge.LoginChallengeToken, f.recoveryCode)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	for err := range results {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 of %d concurrent logins with the same recovery code to succeed, got %d", attempts, successCount)
	}
}

func TestAuthService_CompleteTOTPLogin_NotConfiguredWhenNoEncryptionKey(t *testing.T) {
	f := setupTOTPTestFixture(t)

	// Simulate an operator unsetting --totp-encryption-key after TOTP was
	// already enabled for this user: rebuild the AuthService without a TOTPService.
	noTOTPUserService := NewUserService(f.store)
	noTOTPSessions := NewSessionManager(mustNewSessionStore(t), "test-secret-key-for-unit-tests-1", 1*time.Hour, 24*time.Hour*7)
	authServiceNoTOTP := NewAuthService(noTOTPUserService, noTOTPSessions, nil)

	// Login itself must fail fast with a clear error here, rather than issuing
	// a challenge CompleteTOTPLogin could never redeem (which would strand the
	// user on a code prompt with no valid code to enter).
	_, err := authServiceNoTOTP.Login("testuser", "securepass")
	if err == nil {
		t.Fatal("expected error when TOTP service is not configured")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_not_configured" {
		t.Fatalf("expected auth_totp_not_configured, got %#v", err)
	}
}

func mustNewSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	s, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// totpSetupFixture wires an AuthService to a real TOTPService and a fresh
// "testuser" with TOTP not yet enabled, for exercising the self-service
// setup/confirm/disable/status flows from scratch.
type totpSetupFixture struct {
	authService *AuthService
	store       *UserStore
	sessionStr  *SessionStore
	totpService *TOTPService
	userID      string
}

func setupTOTPSetupFixture(t *testing.T) *totpSetupFixture {
	t.Helper()
	store, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	userService := NewUserService(store)

	user, err := userService.CreateUser("testuser", "test@example.com", "securepass", "admin")
	if err != nil {
		t.Fatal(err)
	}

	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	totpService, err := NewTOTPService(testEncryptionKey())
	if err != nil {
		t.Fatal(err)
	}

	sessions := NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", 1*time.Hour, 24*time.Hour*7)
	authService := NewAuthService(userService, sessions, totpService)

	return &totpSetupFixture{
		authService: authService,
		store:       store,
		sessionStr:  sessionStore,
		totpService: totpService,
		userID:      user.ID,
	}
}

func TestAuthService_StartTOTPSetup_WrongPasswordRejected(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	if _, err := f.authService.StartTOTPSetup(f.userID, "wrong-password"); err != ErrUserInvalidCredentials {
		t.Fatalf("expected ErrUserInvalidCredentials, got %v", err)
	}
}

func TestAuthService_StartTOTPSetup_ReturnsSecretAndStoresPending(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	generated, err := f.authService.StartTOTPSetup(f.userID, "securepass")
	if err != nil {
		t.Fatalf("StartTOTPSetup failed: %v", err)
	}
	if generated.Secret == "" || generated.OTPAuthURL == "" {
		t.Fatal("expected a non-empty manual-entry secret and otpauth URL")
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if user.TOTPEnabled {
		t.Fatal("TOTP must stay disabled until ConfirmTOTPSetup succeeds")
	}
	if user.TOTPSecretEncrypted == "" {
		t.Fatal("expected pending encrypted secret to be stored")
	}
}

func TestAuthService_StartTOTPSetup_AlreadyEnabledRejected(t *testing.T) {
	f := setupTOTPTestFixture(t)

	_, err := f.authService.StartTOTPSetup(f.userID, "securepass")
	if err == nil {
		t.Fatal("expected error when TOTP is already enabled")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_already_enabled" {
		t.Fatalf("expected auth_totp_already_enabled, got %#v", err)
	}
}

func TestAuthService_ConfirmTOTPSetup_NoPendingSetupRejected(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	_, err := f.authService.ConfirmTOTPSetup(f.userID, "123456", "")
	if err == nil {
		t.Fatal("expected error when no setup was started")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_setup_not_started" {
		t.Fatalf("expected auth_totp_setup_not_started, got %#v", err)
	}
}

func TestAuthService_ConfirmTOTPSetup_InvalidCodeRejected(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	if _, err := f.authService.StartTOTPSetup(f.userID, "securepass"); err != nil {
		t.Fatalf("StartTOTPSetup failed: %v", err)
	}

	_, err := f.authService.ConfirmTOTPSetup(f.userID, "000000", "")
	if err == nil {
		t.Fatal("expected error for wrong code")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_invalid_code" {
		t.Fatalf("expected auth_totp_invalid_code, got %#v", err)
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if user.TOTPEnabled {
		t.Fatal("TOTP must not be enabled after a failed confirmation")
	}
}

func TestAuthService_ConfirmTOTPSetup_ValidCodeEnablesAndReturnsRecoveryCodes(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	generated, err := f.authService.StartTOTPSetup(f.userID, "securepass")
	if err != nil {
		t.Fatalf("StartTOTPSetup failed: %v", err)
	}
	plainSecret, err := f.totpService.decrypt(generated.EncryptedSecret)
	if err != nil {
		t.Fatalf("failed to decrypt secret for test setup: %v", err)
	}
	code, err := totp.GenerateCode(plainSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate reference TOTP code: %v", err)
	}

	codes, err := f.authService.ConfirmTOTPSetup(f.userID, code, "")
	if err != nil {
		t.Fatalf("ConfirmTOTPSetup failed: %v", err)
	}
	if len(codes) == 0 {
		t.Fatal("expected non-empty plaintext recovery codes")
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !user.TOTPEnabled {
		t.Fatal("expected TOTP enabled after successful confirmation")
	}
	if len(user.TOTPRecoveryCodeHashes) != len(codes) {
		t.Fatalf("expected %d stored recovery code hashes, got %d", len(codes), len(user.TOTPRecoveryCodeHashes))
	}
}

func TestAuthService_ConfirmTOTPSetup_RevokesOtherSessionsButKeepsCurrent(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	// Simulate an existing "other device" session, plus the session performing this request.
	if err := f.sessionStr.CreateSession("other-device", f.userID, "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to seed other-device session: %v", err)
	}
	currentRefreshToken, currentJTI, _, err := f.authService.sessions.generateToken(&User{ID: f.userID}, time.Hour, "refresh")
	if err != nil {
		t.Fatalf("failed to generate current refresh token: %v", err)
	}
	if err := f.sessionStr.CreateSession(currentJTI, f.userID, "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to seed current session: %v", err)
	}

	generated, err := f.authService.StartTOTPSetup(f.userID, "securepass")
	if err != nil {
		t.Fatalf("StartTOTPSetup failed: %v", err)
	}
	plainSecret, err := f.totpService.decrypt(generated.EncryptedSecret)
	if err != nil {
		t.Fatalf("failed to decrypt secret: %v", err)
	}
	code, err := totp.GenerateCode(plainSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate reference TOTP code: %v", err)
	}

	if _, err := f.authService.ConfirmTOTPSetup(f.userID, code, currentRefreshToken); err != nil {
		t.Fatalf("ConfirmTOTPSetup failed: %v", err)
	}

	if active, _ := f.sessionStr.IsActive("other-device", f.userID, "refresh", time.Now()); active {
		t.Fatal("expected other-device session to be revoked after enabling TOTP")
	}
	if active, _ := f.sessionStr.IsActive(currentJTI, f.userID, "refresh", time.Now()); !active {
		t.Fatal("expected the session performing setup to remain active")
	}
}

func TestAuthService_DisableTOTP_WrongPasswordRejected(t *testing.T) {
	f := setupTOTPTestFixture(t)

	err := f.authService.DisableTOTP(f.userID, "wrong-password", f.currentCode(t), "")
	if err != ErrUserInvalidCredentials {
		t.Fatalf("expected ErrUserInvalidCredentials, got %v", err)
	}
}

func TestAuthService_DisableTOTP_WrongCodeRejected(t *testing.T) {
	f := setupTOTPTestFixture(t)

	err := f.authService.DisableTOTP(f.userID, "securepass", "000000", "")
	if err == nil {
		t.Fatal("expected error for wrong code")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_invalid_code" {
		t.Fatalf("expected auth_totp_invalid_code, got %#v", err)
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !user.TOTPEnabled {
		t.Fatal("TOTP must remain enabled after a failed disable attempt")
	}
}

// TestAuthService_DisableTOTP_NotConfiguredWhenNoEncryptionKey is the
// regression test for the DisableTOTP nil-pointer panic: an operator running
// without --totp-encryption-key (a.totp == nil) while a user still has TOTP
// enabled must get a clean auth_totp_not_configured error from DisableTOTP,
// not a panic.
func TestAuthService_DisableTOTP_NotConfiguredWhenNoEncryptionKey(t *testing.T) {
	f := setupTOTPTestFixture(t)

	noTOTPUserService := NewUserService(f.store)
	noTOTPSessions := NewSessionManager(mustNewSessionStore(t), "test-secret-key-for-unit-tests-1", 1*time.Hour, 24*time.Hour*7)
	authServiceNoTOTP := NewAuthService(noTOTPUserService, noTOTPSessions, nil)

	err := authServiceNoTOTP.DisableTOTP(f.userID, "securepass", "000000", "")
	if err == nil {
		t.Fatal("expected error when TOTP service is not configured")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_not_configured" {
		t.Fatalf("expected auth_totp_not_configured, got %#v", err)
	}
}

func TestAuthService_DisableTOTP_NotEnabledRejected(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	err := f.authService.DisableTOTP(f.userID, "securepass", "000000", "")
	if err == nil {
		t.Fatal("expected error when TOTP is not enabled")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "auth_totp_not_enabled" {
		t.Fatalf("expected auth_totp_not_enabled, got %#v", err)
	}
}

func TestAuthService_DisableTOTP_ValidCodeDisablesAndRevokesOtherSessions(t *testing.T) {
	f := setupTOTPTestFixture(t)

	if err := f.authService.sessions.sessionStore.CreateSession("other-device", f.userID, "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to seed other-device session: %v", err)
	}

	if err := f.authService.DisableTOTP(f.userID, "securepass", f.currentCode(t), ""); err != nil {
		t.Fatalf("DisableTOTP failed: %v", err)
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if user.TOTPEnabled {
		t.Fatal("expected TOTP disabled")
	}
	if user.TOTPSecretEncrypted != "" {
		t.Fatal("expected secret cleared")
	}
	if len(user.TOTPRecoveryCodeHashes) != 0 {
		t.Fatal("expected recovery codes cleared")
	}
	if active, _ := f.authService.sessions.sessionStore.IsActive("other-device", f.userID, "refresh", time.Now()); active {
		t.Fatal("expected other-device session to be revoked after disabling TOTP")
	}
}

func TestAuthService_DisableTOTP_RecoveryCodeAlsoAccepted(t *testing.T) {
	f := setupTOTPTestFixture(t)

	if err := f.authService.DisableTOTP(f.userID, "securepass", f.recoveryCode, ""); err != nil {
		t.Fatalf("DisableTOTP with recovery code failed: %v", err)
	}

	user, err := f.store.GetUserByID(f.userID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if user.TOTPEnabled {
		t.Fatal("expected TOTP disabled")
	}
}

func TestAuthService_GetTOTPStatus(t *testing.T) {
	f := setupTOTPTestFixture(t)

	status, err := f.authService.GetTOTPStatus(f.userID)
	if err != nil {
		t.Fatalf("GetTOTPStatus failed: %v", err)
	}
	if !status.Enabled {
		t.Fatal("expected enabled = true")
	}
	if status.RecoveryCodesRemaining != 3 {
		t.Fatalf("expected 3 remaining recovery codes, got %d", status.RecoveryCodesRemaining)
	}
}

func TestAuthService_GetTOTPStatus_NotEnabled(t *testing.T) {
	f := setupTOTPSetupFixture(t)

	status, err := f.authService.GetTOTPStatus(f.userID)
	if err != nil {
		t.Fatalf("GetTOTPStatus failed: %v", err)
	}
	if status.Enabled {
		t.Fatal("expected enabled = false")
	}
	if status.RecoveryCodesRemaining != 0 {
		t.Fatalf("expected 0 remaining recovery codes, got %d", status.RecoveryCodesRemaining)
	}
}

func TestAuthService_LoginAndValidateToken(t *testing.T) {
	authService := setupTestAuthService(t)

	tokens, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if tokens.Token == "" || tokens.RefreshToken == "" {
		t.Fatal("Expected access and refresh token")
	}

	user, err := authService.ValidateToken(tokens.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}
}

func TestAuthService_RevokeRefreshToken(t *testing.T) {
	authService := setupTestAuthService(t)

	// Login to get a refresh token
	tokens, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if tokens.RefreshToken == "" {
		t.Fatal("Expected refresh token")
	}

	// Refresh token should work before revocation
	newTokens, err := authService.RefreshToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken should work before revocation: %v", err)
	}

	if newTokens.Token == "" || newTokens.RefreshToken == "" {
		t.Fatal("Expected new access and refresh tokens")
	}

	// Revoke the new refresh token
	err = authService.RevokeRefreshToken(newTokens.RefreshToken)
	if err != nil {
		t.Fatalf("RevokeRefreshToken failed: %v", err)
	}

	// Try to use the revoked refresh token - should fail
	_, err = authService.RefreshToken(newTokens.RefreshToken)
	if err == nil {
		t.Fatal("Expected error when using revoked refresh token, got nil")
	}

	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_RevokeRefreshToken_InvalidToken(t *testing.T) {
	authService := setupTestAuthService(t)

	// Try to revoke an invalid token
	err := authService.RevokeRefreshToken("invalid-token")
	if err == nil {
		t.Fatal("Expected error when revoking invalid token, got nil")
	}

	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_RevokeRefreshToken_AccessToken(t *testing.T) {
	authService := setupTestAuthService(t)

	// Login to get tokens
	tokens, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Try to revoke an access token (should fail as it's not a refresh token)
	err = authService.RevokeRefreshToken(tokens.Token)
	if err == nil {
		t.Fatal("Expected error when revoking access token, got nil")
	}

	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthService_RevokeAllUserSessions(t *testing.T) {
	authService := setupTestAuthService(t)

	// Create multiple sessions by logging in multiple times
	tokens1, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("First login failed: %v", err)
	}

	tokens2, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("Second login failed: %v", err)
	}

	// Both refresh tokens should work before revocation
	_, err = authService.RefreshToken(tokens1.RefreshToken)
	if err != nil {
		t.Fatalf("First refresh token should work: %v", err)
	}

	_, err = authService.RefreshToken(tokens2.RefreshToken)
	if err != nil {
		t.Fatalf("Second refresh token should work: %v", err)
	}

	// Get user ID
	user, err := authService.ValidateToken(tokens1.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	// Revoke all sessions for the user
	err = authService.RevokeAllUserSessions(user.ID)
	if err != nil {
		t.Fatalf("RevokeAllUserSessions failed: %v", err)
	}

	// Both refresh tokens should now fail
	_, err = authService.RefreshToken(tokens1.RefreshToken)
	if err == nil {
		t.Fatal("Expected error when using first revoked refresh token, got nil")
	}

	_, err = authService.RefreshToken(tokens2.RefreshToken)
	if err == nil {
		t.Fatal("Expected error when using second revoked refresh token, got nil")
	}
}

func TestAuthService_RevokeAllUserSessions_MultipleUsers(t *testing.T) {
	authService := setupTestAuthService(t)

	// Create a second user
	userService := authService.userService
	_, err := userService.CreateUser("testuser2", "test2@example.com", "securepass2", "admin")
	if err != nil {
		t.Fatalf("Failed to create second user: %v", err)
	}

	// Login both users
	tokens1, err := authService.Login("testuser", "securepass")
	if err != nil {
		t.Fatalf("First user login failed: %v", err)
	}

	tokens2, err := authService.Login("testuser2", "securepass2")
	if err != nil {
		t.Fatalf("Second user login failed: %v", err)
	}

	// Get first user's ID
	user1, err := authService.ValidateToken(tokens1.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	// Revoke all sessions for first user only
	err = authService.RevokeAllUserSessions(user1.ID)
	if err != nil {
		t.Fatalf("RevokeAllUserSessions failed: %v", err)
	}

	// First user's refresh token should fail
	_, err = authService.RefreshToken(tokens1.RefreshToken)
	if err == nil {
		t.Fatal("Expected error when using first user's revoked refresh token, got nil")
	}

	// Second user's refresh token should still work
	_, err = authService.RefreshToken(tokens2.RefreshToken)
	if err != nil {
		t.Fatalf("Second user's refresh token should still work: %v", err)
	}
}

func TestAuthService_Close_ClosesSessionAndUserStores(t *testing.T) {
	authService := setupTestAuthService(t)

	if authService.sessions == nil || authService.sessions.sessionStore == nil {
		t.Fatal("expected session store to be initialized")
	}
	if authService.sessions.sessionStore.db == nil {
		t.Fatal("expected session store db to be initialized")
	}
	if authService.userService == nil || authService.userService.store == nil {
		t.Fatal("expected user service store to be initialized")
	}
	if authService.userService.store.db == nil {
		t.Fatal("expected user store db to be initialized")
	}

	if err := authService.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if authService.sessions.sessionStore.db != nil {
		t.Fatal("expected session store db to be closed")
	}
	if authService.userService.store.db != nil {
		t.Fatal("expected user store db to be closed")
	}
}
