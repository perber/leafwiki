package auth

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

// loginChallengeLifetime bounds how long a "password verified, TOTP pending"
// challenge stays valid. Kept short since it only bridges the two login steps.
const loginChallengeLifetime = 5 * time.Minute

const loginChallengeTokenType = "login_challenge"

type AuthService struct {
	// mu guards only userService — it's the one field ReplaceUserStore swaps
	// after a restore (users.db is part of the snapshot ZIP). sessions'
	// underlying session store is never swapped, so it needs no lock.
	mu          sync.RWMutex
	userService *UserService
	sessions    *SessionManager
	totp        *TOTPService
	attempts    *loginAttemptTracker
	dummyHash   []byte
}

// users returns the current *UserService under a read lock. Callers use the
// returned pointer directly for the rest of their operation — the lock is
// only held long enough to copy the pointer, never across a query or a
// bcrypt hash, so ReplaceUserStore swapping it mid-request never serializes
// unrelated logins/refreshes on this mutex.
func (a *AuthService) users() *UserService {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.userService
}

// ReplaceUserStore opens a fresh UserStore/UserService against
// storageDir/users.db and swaps it in, closing the previous one afterward.
// Used by a live restore after users.db has been swapped in on disk. Requests
// already in flight against the old *UserService when the swap happens just
// finish against it (a query on an about-to-close *sql.DB may transiently
// fail) — acceptable given the write-gate has already stopped new mutating
// traffic for the duration of the restore, and GET requests self-heal on
// their next call.
func (a *AuthService) ReplaceUserStore(storageDir string) error {
	newStore, err := NewUserStore(storageDir)
	if err != nil {
		return err
	}
	newUserService := NewUserService(newStore)

	a.mu.Lock()
	old := a.userService
	a.userService = newUserService
	a.mu.Unlock()

	if old != nil {
		if err := old.Close(); err != nil {
			slog.Warn("failed to close previous user store after restore", "error", err)
		}
	}
	return nil
}

// InvalidateAllSessions revokes every active session on this instance —
// every logged-in user (including whoever triggered this) must log back in.
// Used after a restore: the restored users.db may have entirely different
// user IDs/passwords than the sessions currently trusting this process.
func (a *AuthService) InvalidateAllSessions() error {
	return a.sessions.sessionStore.DeleteAllSessions()
}

func NewAuthService(userService *UserService, sessions *SessionManager, totpService *TOTPService) *AuthService {
	// Pre-compute a dummy hash to equalize Login() timing for non-existent users,
	// preventing username enumeration via response-time differences.
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte("leafwiki-dummy-password"), bcrypt.DefaultCost)
	a := &AuthService{
		userService: userService,
		sessions:    sessions,
		totp:        totpService,
		attempts:    newLoginAttemptTracker(),
		dummyHash:   dummyHash,
	}
	// Wired here rather than passed into NewSessionManager: a.users() reads
	// through the mutex ReplaceUserStore swaps, so RefreshToken/ValidateToken
	// stay correct across a live-restore hot-swap — this closure calls
	// a.users() fresh on every invocation rather than capturing one
	// *UserService at construction time.
	sessions.resolveUser = func(id string) (*User, error) {
		return a.users().GetUserByID(id)
	}
	return a
}

func (a *AuthService) Close() error {
	var errs []error

	if a.sessions != nil {
		if err := a.sessions.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if users := a.users(); users != nil {
		if err := users.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

type AuthToken struct {
	Token                string      `json:"token"`
	RefreshToken         string      `json:"refresh_token"`
	AccessTokenExpiresAt int64       `json:"accessTokenExpiresAt"`
	User                 *PublicUser `json:"user"`

	// RequiresTOTP and LoginChallengeToken are set instead of Token/RefreshToken/User
	// when Login succeeds on password but the account has TOTP enabled: no cookies
	// may be issued yet, and the caller must complete the handshake via
	// CompleteTOTPLogin before real tokens exist. Never marshaled directly (the
	// HTTP layer builds its own response shapes for both cases).
	RequiresTOTP        bool   `json:"-"`
	LoginChallengeToken string `json:"-"`
}

// Login verifies identifier/password. If the account has TOTP disabled, it
// behaves exactly as before: real access/refresh tokens are issued and auth
// cookies may be set immediately. If TOTP is enabled, no tokens are issued;
// instead a short-lived login challenge is returned, and CompleteTOTPLogin
// must be called with the resulting LoginChallengeToken and a valid TOTP or
// recovery code before cookies may be set.
func (a *AuthService) Login(identifier, password string) (*AuthToken, error) {
	user, err := a.users().GetUserByIdentifier(identifier)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword(a.dummyHash, []byte(password))
		return nil, ErrUserInvalidCredentials
	}

	if !a.attempts.recordAttempt(user.ID) {
		return nil, ErrUserAccountLocked
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrUserInvalidCredentials
	}

	user.Password = ""

	if user.TOTPEnabled {
		if a.totp == nil {
			// Config drift: TOTP was enabled for this user while an encryption
			// key was configured, but the server is currently running without
			// one. Fail here, at the password step, rather than issuing a
			// challenge CompleteTOTPLogin can never redeem (it requires
			// a.totp too), which would otherwise strand the user on a code
			// prompt with no valid code to enter.
			return nil, errTOTPNotConfigured()
		}
		// Do not reset the attempt counter yet: a correct password is not a
		// complete login while TOTP is still required. CompleteTOTPLogin
		// shares this same per-user counter to rate-limit TOTP/recovery-code
		// guesses; resetting it here would let anyone who already knows the
		// password wipe out failed TOTP attempts by simply resubmitting the
		// password, defeating the lockout on TOTP code brute-forcing. The
		// counter is only reset once CompleteTOTPLogin fully succeeds.
		return a.beginTOTPChallenge(user)
	}

	a.attempts.reset(user.ID)
	return a.sessions.IssueSession(user)
}

// CompleteTOTPLogin finishes a login handshake started by Login when a user
// has TOTP enabled. It validates challengeToken (single use, short-lived) and
// then code as either a current TOTP code or an unused recovery code; only on
// success are the same access/refresh tokens issued that a password-only
// login would have produced.
func (a *AuthService) CompleteTOTPLogin(challengeToken, code string) (*AuthToken, error) {
	if a.totp == nil {
		return nil, errTOTPNotConfigured()
	}

	userID, jti, err := a.parseLoginChallenge(challengeToken)
	if err != nil {
		return nil, err
	}

	active, err := a.sessions.sessionStore.IsActive(jti, userID, loginChallengeTokenType, time.Now())
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, errInvalidLoginChallenge()
	}

	if !a.attempts.recordAttempt(userID) {
		return nil, ErrUserAccountLocked
	}

	// Captured once and threaded through to verifyTOTPOrRecoveryCode below,
	// so the whole handshake (fetch + verify + consume-recovery-code) reads
	// and writes against the same underlying user store even if a live
	// restore hot-swaps AuthService.userService partway through — instead of
	// each call independently re-reading whatever store is current at that
	// instant.
	users := a.users()
	user, err := users.GetUserByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if !user.TOTPEnabled || user.TOTPSecretEncrypted == "" {
		// TOTP was disabled after the challenge was issued; it can no longer be completed.
		return nil, errInvalidLoginChallenge()
	}

	valid, err := a.verifyTOTPOrRecoveryCode(users, user, code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errTOTPInvalidCode()
	}

	a.attempts.reset(userID)

	// Single-use: revoke the challenge now that it has been consumed successfully.
	if err := a.sessions.sessionStore.RevokeSession(jti); err != nil {
		slog.Warn("failed to revoke used login challenge session", "error", err)
	}

	user.Password = ""
	return a.sessions.IssueSession(user)
}

// StartTOTPSetup verifies the user's current password, generates a fresh TOTP
// secret, and stores it as pending (TOTP stays disabled until ConfirmTOTPSetup
// verifies a code against it). Returns the manual-entry secret and otpauth://
// URI for the frontend to render as a QR code; the plaintext secret is never
// persisted.
func (a *AuthService) StartTOTPSetup(userID, currentPassword string) (*GeneratedSecret, error) {
	if a.totp == nil {
		return nil, errTOTPNotConfigured()
	}
	// Captured once so both calls below hit the same store even across a
	// concurrent live-restore swap — see CompleteTOTPLogin's comment.
	users := a.users()
	user, err := users.DoesIDAndPasswordMatch(userID, currentPassword)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, errTOTPAlreadyEnabled()
	}

	generated, err := a.totp.GenerateSecret(user.Email)
	if err != nil {
		return nil, err
	}
	if err := users.SetPendingTOTPSecret(userID, generated.EncryptedSecret); err != nil {
		return nil, err
	}
	return generated, nil
}

// ConfirmTOTPSetup verifies code against the pending secret set by
// StartTOTPSetup and, on success, enables TOTP and returns freshly generated
// recovery codes in plaintext (shown to the user exactly once; only their
// hashes are persisted). Every other session for the user is revoked;
// currentRefreshToken identifies the caller's own session, which is left intact.
func (a *AuthService) ConfirmTOTPSetup(userID, code, currentRefreshToken string) ([]string, error) {
	if a.totp == nil {
		return nil, errTOTPNotConfigured()
	}
	// Captured once so both calls below hit the same store even across a
	// concurrent live-restore swap — see CompleteTOTPLogin's comment.
	users := a.users()
	user, err := users.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user.TOTPEnabled {
		return nil, errTOTPAlreadyEnabled()
	}
	if user.TOTPSecretEncrypted == "" {
		return nil, errTOTPSetupNotStarted()
	}

	valid, err := a.totp.VerifyCode(user.TOTPSecretEncrypted, code)
	if err != nil {
		return nil, errTOTPVerificationFailed(err)
	}
	if !valid {
		return nil, errTOTPInvalidCode()
	}

	codes, hashes, err := a.totp.GenerateRecoveryCodes(0)
	if err != nil {
		return nil, err
	}
	if err := users.EnableTOTP(userID, user.TOTPSecretEncrypted, hashes); err != nil {
		return nil, err
	}

	if err := a.sessions.RevokeAllUserSessionsExceptCurrent(userID, currentRefreshToken); err != nil {
		slog.Warn("failed to revoke other sessions after enabling TOTP", "userID", userID, "error", err)
	}

	return codes, nil
}

// DisableTOTP verifies the user's current password plus a valid TOTP or
// recovery code, then disables TOTP and clears the stored secret and recovery
// codes. Every other session for the user is revoked; currentRefreshToken
// identifies the caller's own session, which is left intact.
func (a *AuthService) DisableTOTP(userID, currentPassword, code, currentRefreshToken string) error {
	if a.totp == nil {
		return errTOTPNotConfigured()
	}
	// Captured once so every call below hits the same store even across a
	// concurrent live-restore swap — see CompleteTOTPLogin's comment.
	users := a.users()
	user, err := users.DoesIDAndPasswordMatch(userID, currentPassword)
	if err != nil {
		return err
	}
	if !user.TOTPEnabled {
		return errTOTPNotEnabled()
	}

	valid, err := a.verifyTOTPOrRecoveryCode(users, user, code)
	if err != nil {
		return err
	}
	if !valid {
		return errTOTPInvalidCode()
	}

	if err := users.DisableTOTP(userID); err != nil {
		return err
	}

	if err := a.sessions.RevokeAllUserSessionsExceptCurrent(userID, currentRefreshToken); err != nil {
		slog.Warn("failed to revoke other sessions after disabling TOTP", "userID", userID, "error", err)
	}

	return nil
}

// TOTPStatus is the non-secret TOTP status exposed to the user themselves.
type TOTPStatus struct {
	Enabled                bool
	RecoveryCodesRemaining int
}

// GetTOTPStatus returns userID's current TOTP status. Never exposes the
// secret or the recovery codes themselves, only whether TOTP is enabled and
// how many recovery codes remain unused.
func (a *AuthService) GetTOTPStatus(userID string) (*TOTPStatus, error) {
	user, err := a.users().GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	return &TOTPStatus{
		Enabled:                user.TOTPEnabled,
		RecoveryCodesRemaining: len(user.TOTPRecoveryCodeHashes),
	}, nil
}

// maxRecoveryCodeConsumeAttempts bounds the compare-and-swap retry loop in
// verifyTOTPOrRecoveryCode. Contention here only ever comes from concurrent
// requests racing to consume the same code, so a handful of retries is ample;
// exhausting them denies the request rather than risking a double-consume.
const maxRecoveryCodeConsumeAttempts = 3

// verifyTOTPOrRecoveryCode checks code against user's current TOTP secret or,
// failing that, their remaining recovery codes — consuming (removing) a
// matched recovery code so it cannot be reused. Shared by the login handshake
// and the self-service disable flow.
//
// users is the *UserService the caller already fetched user from — threaded
// through explicitly (rather than calling a.users() again in here) so the
// whole fetch-verify-consume-and-retry sequence reads and writes against one
// consistent store even if a live restore hot-swaps AuthService.userService
// partway through.
//
// Recovery-code consumption uses optimistic concurrency (compare-and-swap on
// the stored hash list) rather than a plain read-then-write, so that two
// concurrent requests presenting the same recovery code cannot both succeed:
// only the first writer's compare-and-swap can match the row's current state;
// the loser re-reads the now-current hashes and retries against those.
func (a *AuthService) verifyTOTPOrRecoveryCode(users *UserService, user *User, code string) (bool, error) {
	valid, err := a.totp.VerifyCode(user.TOTPSecretEncrypted, code)
	if err != nil {
		return false, errTOTPVerificationFailed(err)
	}
	if valid {
		return true, nil
	}

	hashes := user.TOTPRecoveryCodeHashes
	for attempt := 0; attempt < maxRecoveryCodeConsumeAttempts; attempt++ {
		idx, matched := VerifyRecoveryCode(code, hashes)
		if !matched {
			return false, nil
		}

		remaining := make([]string, 0, len(hashes)-1)
		remaining = append(remaining, hashes[:idx]...)
		remaining = append(remaining, hashes[idx+1:]...)

		swapped, err := users.ConsumeRecoveryCodeHash(user.ID, hashes, remaining)
		if err != nil {
			return false, err
		}
		if swapped {
			return true, nil
		}

		// Lost the race: a concurrent request already changed the stored
		// hashes (e.g. consumed the same or a different code first).
		// Re-read the current state and retry against it.
		refreshed, err := users.GetUserByID(user.ID)
		if err != nil {
			return false, err
		}
		hashes = refreshed.TOTPRecoveryCodeHashes
	}

	return false, nil
}

func errInvalidLoginChallenge() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_challenge_invalid",
		"Invalid or expired login challenge",
		"invalid or expired TOTP login challenge",
		nil,
	)
}

func errTOTPNotConfigured() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_not_configured",
		"Two-factor authentication is not available on this server",
		"TOTP requested but no TOTP encryption key is configured",
		nil,
	)
}

func errTOTPInvalidCode() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_invalid_code",
		"Invalid authentication code",
		"invalid TOTP or recovery code",
		nil,
	)
}

func errTOTPAlreadyEnabled() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_already_enabled",
		"Two-factor authentication is already enabled",
		"TOTP is already enabled for this account",
		nil,
	)
}

func errTOTPSetupNotStarted() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_setup_not_started",
		"Two-factor authentication setup was not started",
		"no pending TOTP setup for this account",
		nil,
	)
}

func errTOTPNotEnabled() error {
	return sharederrors.NewLocalizedError(
		"auth_totp_not_enabled",
		"Two-factor authentication is not enabled",
		"TOTP is not enabled for this account",
		nil,
	)
}

// errTOTPVerificationFailed wraps an unexpected failure to even attempt TOTP
// verification (e.g. the stored secret could not be decrypted, typically
// after a TOTP encryption key was rotated or corrupted) as a LocalizedError,
// per this repo's convention that domain errors reaching HTTP handlers must
// be *sharederrors.LocalizedError, not a bare fmt.Errorf/errors.New.
func errTOTPVerificationFailed(cause error) error {
	return sharederrors.NewLocalizedError(
		"auth_totp_verification_failed",
		"Two-factor authentication is temporarily unavailable",
		"failed to verify TOTP code",
		cause,
	)
}

// parseLoginChallenge validates challengeToken's signature, type, and required
// claims, returning the subject user ID and challenge jti. Reuses
// SessionManager's low-level JWT parsing since login-challenge tokens share
// the same secret and signing method as access/refresh tokens.
func (a *AuthService) parseLoginChallenge(challengeToken string) (userID, jti string, err error) {
	claims, err := a.sessions.parseClaims(challengeToken)
	if err != nil {
		return "", "", errInvalidLoginChallenge()
	}
	typ, ok := claims["typ"].(string)
	if !ok || typ != loginChallengeTokenType {
		return "", "", errInvalidLoginChallenge()
	}
	userID, ok = claims["sub"].(string)
	if !ok || userID == "" {
		return "", "", errInvalidLoginChallenge()
	}
	jti, ok = claims["jti"].(string)
	if !ok || jti == "" {
		return "", "", errInvalidLoginChallenge()
	}
	return userID, jti, nil
}

// beginTOTPChallenge issues a short-lived, single-use login challenge for a
// user who passed the password check but still needs to prove TOTP. It signs
// its own claim shape (no role/email, unlike an access/refresh token) via
// SessionManager's shared signing key, and records the challenge in the same
// session store access/refresh sessions live in, disambiguated by
// loginChallengeTokenType.
func (a *AuthService) beginTOTPChallenge(user *User) (*AuthToken, error) {
	jti, err := generateJTI()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(loginChallengeLifetime)
	claims := jwt.MapClaims{
		"sub": user.ID,
		"typ": loginChallengeTokenType,
		"jti": jti,
		"exp": expiresAt.Unix(),
		"iat": time.Now().Unix(),
	}
	signed, err := a.sessions.signClaims(claims)
	if err != nil {
		return nil, err
	}

	if err := a.sessions.sessionStore.CreateSession(jti, user.ID, loginChallengeTokenType, expiresAt); err != nil {
		return nil, err
	}

	return &AuthToken{
		RequiresTOTP:        true,
		LoginChallengeToken: signed,
	}, nil
}

// RefreshToken, RevokeRefreshToken, RevokeAllUserSessions, and ValidateToken
// delegate to SessionManager, which owns the JWT/session-store mechanics.
// They stay exposed on AuthService because internal/wiki/auth's use cases and
// internal/http/middleware/auth call them here, not on SessionManager directly.

func (a *AuthService) RefreshToken(refreshToken string) (*AuthToken, error) {
	return a.sessions.RefreshToken(refreshToken)
}

func (a *AuthService) RevokeRefreshToken(tokenString string) error {
	return a.sessions.RevokeRefreshToken(tokenString)
}

func (a *AuthService) RevokeAllUserSessions(userID string) error {
	return a.sessions.RevokeAllUserSessions(userID)
}

func (a *AuthService) ValidateToken(tokenString string) (*User, error) {
	return a.sessions.ValidateToken(tokenString)
}
