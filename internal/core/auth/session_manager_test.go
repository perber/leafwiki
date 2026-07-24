package auth

import (
	"database/sql"
	"testing"
	"time"
)

// readSessionExpiresAt reads back the raw expires_at column stored for jti,
// so tests can compare it exactly against a token's own exp claim.
func readSessionExpiresAt(t *testing.T, store *SessionStore, jti string) int64 {
	t.Helper()
	var expiresAt int64
	err := store.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT expires_at FROM sessions WHERE id = ?`, jti).Scan(&expiresAt)
	})
	if err != nil {
		t.Fatalf("failed to read session expires_at: %v", err)
	}
	return expiresAt
}

// setupTestSessionManager builds a SessionManager backed by a real (temp-dir)
// SessionStore and an in-memory resolver over users, so tests can exercise
// session issuance/refresh/revocation/validation in isolation from password
// or TOTP login.
func setupTestSessionManager(t *testing.T, users map[string]*User) *SessionManager {
	t.Helper()
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour*7)
	// Production code wires this up via NewAuthService (bound to its
	// hot-swap-safe a.users); tests exercising SessionManager in isolation
	// wire a plain in-memory lookup directly since it's the same package.
	sm.resolveUser = func(id string) (*User, error) {
		u, ok := users[id]
		if !ok {
			return nil, ErrUserNotFound
		}
		return u, nil
	}
	return sm
}

func TestSessionManager_IssueSession_ReturnsAccessAndRefreshTokens(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Email: "alice@example.com", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	if tokens.Token == "" || tokens.RefreshToken == "" {
		t.Fatal("expected access and refresh token")
	}
	if tokens.User == nil || tokens.User.Username != "alice" {
		t.Fatalf("expected public user alice in response, got %+v", tokens.User)
	}
}

func TestSessionManager_ValidateToken_ReturnsCurrentUser(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleEditor}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	got, err := sm.ValidateToken(tokens.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if got.ID != "u1" || got.Username != "alice" {
		t.Fatalf("unexpected user from ValidateToken: %+v", got)
	}
}

func TestSessionManager_ValidateToken_InvalidTokenRejected(t *testing.T) {
	sm := setupTestSessionManager(t, nil)
	if _, err := sm.ValidateToken("not-a-token"); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

// Regression test: ValidateToken must only accept typ=="access" tokens. It
// previously accepted any validly-signed token regardless of typ, meaning a
// refresh token (long-lived, and not checked against the session store here)
// could be used as an access token.
func TestSessionManager_ValidateToken_RejectsRefreshToken(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	if _, err := sm.ValidateToken(tokens.RefreshToken); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken when validating a refresh token as an access token, got %v", err)
	}
}

// A SessionManager constructed directly (not via NewAuthService, which wires
// resolveUser) must fail loudly rather than panic on a nil call.
func TestSessionManager_ValidateToken_ResolveUserNotWired_ReturnsErrorInsteadOfPanicking(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)

	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm.resolveUser = func(id string) (*User, error) { return user, nil }
	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	// Simulate the not-yet-wired state ValidateToken/RefreshToken must guard against.
	sm.resolveUser = nil

	if _, err := sm.ValidateToken(tokens.Token); err == nil {
		t.Fatal("expected an error, not a panic or nil, when resolveUser is unset")
	}
}

func TestSessionManager_RefreshToken_ResolveUserNotWired_ReturnsErrorInsteadOfPanicking(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)

	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm.resolveUser = func(id string) (*User, error) { return user, nil }
	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	sm.resolveUser = nil

	if _, err := sm.RefreshToken(tokens.RefreshToken); err == nil {
		t.Fatal("expected an error, not a panic or nil, when resolveUser is unset")
	}
}

// ValidateToken must re-resolve the user on every call rather than trusting
// the role/email baked into the token's claims, so a role change takes
// effect immediately instead of waiting for the access token to expire.
func TestSessionManager_ValidateToken_ReflectsCurrentResolvedUser(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleViewer}
	users := map[string]*User{user.ID: user}
	sm := setupTestSessionManager(t, users)

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	users["u1"] = &User{ID: "u1", Username: "alice", Role: RoleAdmin}

	got, err := sm.ValidateToken(tokens.Token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if got.Role != RoleAdmin {
		t.Fatalf("expected freshly resolved role %q, got %q", RoleAdmin, got.Role)
	}
}

func TestSessionManager_RefreshToken_IssuesNewTokensAndRevokesOld(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	newTokens, err := sm.RefreshToken(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if newTokens.Token == "" || newTokens.RefreshToken == "" {
		t.Fatal("expected new access and refresh tokens")
	}

	if _, err := sm.RefreshToken(tokens.RefreshToken); err == nil {
		t.Fatal("expected the old refresh token to be revoked after use")
	}
}

func TestSessionManager_RefreshToken_RevokedTokenRejected(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	if err := sm.RevokeRefreshToken(tokens.RefreshToken); err != nil {
		t.Fatalf("RevokeRefreshToken failed: %v", err)
	}
	if _, err := sm.RefreshToken(tokens.RefreshToken); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionManager_RefreshToken_AccessTokenRejected(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	if _, err := sm.RefreshToken(tokens.Token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken when refreshing with an access token, got %v", err)
	}
}

func TestSessionManager_RefreshToken_InvalidTokenRejected(t *testing.T) {
	sm := setupTestSessionManager(t, nil)
	if _, err := sm.RefreshToken("invalid"); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionManager_RevokeRefreshToken_InvalidToken(t *testing.T) {
	sm := setupTestSessionManager(t, nil)
	if err := sm.RevokeRefreshToken("invalid"); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionManager_RevokeRefreshToken_AccessTokenRejected(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})
	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	if err := sm.RevokeRefreshToken(tokens.Token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestSessionManager_RevokeAllUserSessions(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens1, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	tokens2, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	if err := sm.RevokeAllUserSessions(user.ID); err != nil {
		t.Fatalf("RevokeAllUserSessions failed: %v", err)
	}

	if _, err := sm.RefreshToken(tokens1.RefreshToken); err == nil {
		t.Fatal("expected first session to be revoked")
	}
	if _, err := sm.RefreshToken(tokens2.RefreshToken); err == nil {
		t.Fatal("expected second session to be revoked")
	}
}

func TestSessionManager_RevokeAllUserSessionsExceptCurrent_PreservesCallersSession(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	current, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}
	other, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	if err := sm.RevokeAllUserSessionsExceptCurrent(user.ID, current.RefreshToken); err != nil {
		t.Fatalf("RevokeAllUserSessionsExceptCurrent failed: %v", err)
	}

	if _, err := sm.RefreshToken(current.RefreshToken); err != nil {
		t.Fatalf("expected the current session to survive: %v", err)
	}
	if _, err := sm.RefreshToken(other.RefreshToken); err == nil {
		t.Fatal("expected the other session to be revoked")
	}
}

func TestSessionManager_RevokeAllUserSessionsExceptCurrent_UnparsableCurrentRevokesEverything(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	sm := setupTestSessionManager(t, map[string]*User{user.ID: user})

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	if err := sm.RevokeAllUserSessionsExceptCurrent(user.ID, "not-a-token"); err != nil {
		t.Fatalf("RevokeAllUserSessionsExceptCurrent failed: %v", err)
	}

	if _, err := sm.RefreshToken(tokens.RefreshToken); err == nil {
		t.Fatal("expected every session to be revoked when the current token can't be identified")
	}
}

// fakeAdvancingClock returns a clock that moves forward by step on every
// call, so a bug that recomputes an expiry from a second, independent now()
// call (instead of reusing an already-computed one) produces a deterministic,
// detectable drift instead of an unreliable real-time race.
func fakeAdvancingClock(step time.Duration) func() time.Time {
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	return func() time.Time {
		now := base.Add(time.Duration(calls) * step)
		calls++
		return now
	}
}

// Regression test: IssueSession previously stored the refresh session's
// expiry via its own now().Add(lifetime) call, separate from the exp already
// embedded in the refresh token's claims moments earlier — the two could
// drift, leaving a token still cryptographically valid while its session row
// was already (or not yet) considered expired.
func TestSessionManager_IssueSession_SessionExpiryMatchesRefreshTokenExp(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	sm.resolveUser = func(id string) (*User, error) { return user, nil }
	sm.now = fakeAdvancingClock(3 * time.Second)

	tokens, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	claims, err := sm.parseClaims(tokens.RefreshToken)
	if err != nil {
		t.Fatalf("failed to parse refresh token claims: %v", err)
	}
	tokenExp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("expected exp claim, got %#v", claims["exp"])
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		t.Fatalf("expected jti claim, got %#v", claims["jti"])
	}

	storedExpiresAt := readSessionExpiresAt(t, store, jti)
	if int64(tokenExp) != storedExpiresAt {
		t.Fatalf("session-store expiry (%d) does not match refresh token's own exp claim (%d)", storedExpiresAt, int64(tokenExp))
	}
}

// Same regression, exercised via RefreshToken's newly-issued refresh session.
func TestSessionManager_RefreshToken_SessionExpiryMatchesNewRefreshTokenExp(t *testing.T) {
	user := &User{ID: "u1", Username: "alice", Role: RoleAdmin}
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	sm.resolveUser = func(id string) (*User, error) { return user, nil }
	sm.now = fakeAdvancingClock(3 * time.Second)

	initial, err := sm.IssueSession(user)
	if err != nil {
		t.Fatalf("IssueSession failed: %v", err)
	}

	refreshed, err := sm.RefreshToken(initial.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	claims, err := sm.parseClaims(refreshed.RefreshToken)
	if err != nil {
		t.Fatalf("failed to parse refresh token claims: %v", err)
	}
	tokenExp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("expected exp claim, got %#v", claims["exp"])
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		t.Fatalf("expected jti claim, got %#v", claims["jti"])
	}

	storedExpiresAt := readSessionExpiresAt(t, store, jti)
	if int64(tokenExp) != storedExpiresAt {
		t.Fatalf("session-store expiry (%d) does not match refresh token's own exp claim (%d)", storedExpiresAt, int64(tokenExp))
	}
}

func TestSessionManager_Close_ClosesSessionStore(t *testing.T) {
	store, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sm := NewSessionManager(store, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)

	if err := sm.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if store.db != nil {
		t.Fatal("expected session store db to be closed")
	}
}
