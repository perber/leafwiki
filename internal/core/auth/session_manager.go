package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SessionManager owns everything needed to turn an already-identified *User
// into a LeafWiki session (JWT access+refresh tokens) and to manage that
// session afterward (refresh, revoke, validate) — independent of how the
// user was authenticated. Local password+TOTP login (AuthService) is the
// only caller today; any future non-password identity provider (e.g. an
// OIDC callback handler) would call IssueSession the same way, reusing this
// component rather than reimplementing session handling.
type SessionManager struct {
	sessionStore         *SessionStore
	secretKey            []byte
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration

	// resolveUser fetches the current *User for a subject ID. It is called
	// fresh on every RefreshToken/ValidateToken (never cached here) and is
	// wired up by NewAuthService (to its own hot-swap-safe a.users, see
	// AuthService.ReplaceUserStore) rather than passed in here — at
	// construction time there is no AuthService yet for it to read through.
	resolveUser func(id string) (*User, error)
}

// NewSessionManager constructs a SessionManager backed by sessionStore.
// resolveUser is not set yet at this point — see the field comment; callers
// must arrange for it to be assigned (NewAuthService does this) before
// RefreshToken/ValidateToken are used.
func NewSessionManager(sessionStore *SessionStore, secret string, accessTokenTimeout, refreshTokenTimeout time.Duration) *SessionManager {
	if len(secret) < 32 {
		slog.Warn("JWT secret is too short; a minimum of 32 characters is strongly recommended", "length", len(secret))
	}
	return &SessionManager{
		sessionStore:         sessionStore,
		secretKey:            []byte(secret),
		accessTokenLifetime:  accessTokenTimeout,
		refreshTokenLifetime: refreshTokenTimeout,
	}
}

func (s *SessionManager) Close() error {
	if s.sessionStore == nil {
		return nil
	}
	return s.sessionStore.Close()
}

// IssueSession generates and stores access/refresh tokens for an
// already-authenticated user.
func (s *SessionManager) IssueSession(user *User) (*AuthToken, error) {
	accessToken, _, accessTokenExpiresAt, err := s.generateToken(user, s.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	refreshToken, refreshJTI, _, err := s.generateToken(user, s.refreshTokenLifetime, "refresh")
	if err != nil {
		return nil, err
	}

	if err := s.sessionStore.CreateSession(
		refreshJTI,
		user.ID,
		"refresh",
		time.Now().Add(s.refreshTokenLifetime),
	); err != nil {
		return nil, err
	}

	return &AuthToken{
		Token:                accessToken,
		RefreshToken:         refreshToken,
		AccessTokenExpiresAt: accessTokenExpiresAt,
		User:                 user.ToPublicUser(),
	}, nil
}

// RefreshToken validates refreshToken, resolves the current user, and issues
// a fresh access/refresh token pair. The old refresh token is only revoked
// after the new session has been created successfully: if token generation
// or session creation fails, the old token remains valid and the caller can
// retry. A failure to revoke the old token afterward is logged but does not
// fail the refresh — the old token expires naturally, and having two valid
// tokens briefly is safer than logging the user out.
func (s *SessionManager) RefreshToken(refreshToken string) (*AuthToken, error) {
	claims, err := s.parseClaims(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	typ, ok := claims["typ"].(string)
	if !ok || typ != "refresh" {
		return nil, ErrInvalidToken
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return nil, ErrInvalidToken
	}

	active, err := s.sessionStore.IsActive(jti, userID, "refresh", time.Now())
	if err != nil || !active {
		return nil, ErrInvalidToken
	}

	user, err := s.resolveUser(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	user.Password = ""

	newAccessToken, _, accessTokenExpiresAt, err := s.generateToken(user, s.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	newRefreshToken, newRefreshJTI, _, err := s.generateToken(user, s.refreshTokenLifetime, "refresh")
	if err != nil {
		return nil, err
	}

	if err := s.sessionStore.CreateSession(
		newRefreshJTI,
		user.ID,
		"refresh",
		time.Now().Add(s.refreshTokenLifetime),
	); err != nil {
		return nil, err
	}

	if err := s.sessionStore.RevokeSession(jti); err != nil {
		slog.Warn("failed to revoke used refresh token session", "error", err)
	}

	return &AuthToken{
		Token:                newAccessToken,
		RefreshToken:         newRefreshToken,
		AccessTokenExpiresAt: accessTokenExpiresAt,
		User:                 user.ToPublicUser(),
	}, nil
}

func (s *SessionManager) RevokeRefreshToken(tokenString string) error {
	claims, err := s.parseClaims(tokenString)
	if err != nil {
		return ErrInvalidToken
	}

	typ, ok := claims["typ"].(string)
	if !ok || typ != "refresh" {
		return ErrInvalidToken
	}

	jti, ok := claims["jti"].(string)
	if !ok || jti == "" {
		return ErrInvalidToken
	}

	return s.sessionStore.RevokeSession(jti)
}

func (s *SessionManager) RevokeAllUserSessions(userID string) error {
	return s.sessionStore.RevokeAllSessionsForUser(userID)
}

// RevokeAllUserSessionsExceptCurrent revokes every other session for userID,
// preserving the one identified by currentRefreshToken. If currentRefreshToken
// cannot be parsed (e.g. missing), every session is revoked — a safe fallback
// over silently preserving an unidentified session.
func (s *SessionManager) RevokeAllUserSessionsExceptCurrent(userID, currentRefreshToken string) error {
	var exceptJTI string
	if claims, err := s.parseClaims(currentRefreshToken); err == nil {
		if jti, ok := claims["jti"].(string); ok {
			exceptJTI = jti
		}
	}
	return s.sessionStore.RevokeAllSessionsForUserExcept(userID, exceptJTI)
}

// ValidateToken validates tokenString and re-resolves the current user for
// its subject on every call (rather than trusting the role/email baked into
// the claims), so a role change or hot-swapped user store takes effect
// immediately instead of waiting for the access token to expire.
func (s *SessionManager) ValidateToken(tokenString string) (*User, error) {
	claims, err := s.parseClaims(tokenString)
	if err != nil {
		return nil, ErrInvalidToken
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	return s.resolveUser(userID)
}

func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// signClaims signs an arbitrary claim set with the session secret. Used by
// generateToken below for access/refresh tokens, and directly by
// AuthService's login-challenge (TOTP handshake) tokens, which share the
// same secret and signing method but a different claim shape.
func (s *SessionManager) signClaims(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// parseClaims validates tokenString's signature and signing method and
// returns its claims. Used for access/refresh tokens here, and directly by
// AuthService for login-challenge tokens, which share the same secret.
func (s *SessionManager) parseClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secretKey, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (s *SessionManager) generateToken(user *User, duration time.Duration, typ string) (string, string, int64, error) {
	jti, err := generateJTI()
	if err != nil {
		return "", "", 0, err
	}
	expiresAt := time.Now().Add(duration).Unix()
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"role":  user.Role,
		"email": user.Email,
		"exp":   expiresAt,
		"iat":   time.Now().Unix(),
		"typ":   typ,
		"jti":   jti, // Unique identifier for the token
	}
	signed, err := s.signClaims(claims)
	if err != nil {
		return "", "", 0, err
	}
	return signed, jti, expiresAt, nil
}
