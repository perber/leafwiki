package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userService          *UserService
	sessionStore         *SessionStore
	secretKey            []byte
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration
	attempts             *loginAttemptTracker
	dummyHash            []byte
}

func NewAuthService(userService *UserService, sessionStore *SessionStore, secret string, accessTokenTimeout, refreshTokenTimeout time.Duration) *AuthService {
	if len(secret) < 32 {
		slog.Warn("JWT secret is too short; a minimum of 32 characters is strongly recommended", "length", len(secret))
	}
	// Pre-compute a dummy hash to equalize Login() timing for non-existent users,
	// preventing username enumeration via response-time differences.
	dummyHash, _ := bcrypt.GenerateFromPassword([]byte("leafwiki-dummy-password"), bcrypt.DefaultCost)
	return &AuthService{
		userService:          userService,
		sessionStore:         sessionStore,
		secretKey:            []byte(secret),
		accessTokenLifetime:  accessTokenTimeout,
		refreshTokenLifetime: refreshTokenTimeout,
		attempts:             newLoginAttemptTracker(),
		dummyHash:            dummyHash,
	}
}

type AuthToken struct {
	Token                string      `json:"token"`
	RefreshToken         string      `json:"refresh_token"`
	AccessTokenExpiresAt int64       `json:"accessTokenExpiresAt"`
	User                 *PublicUser `json:"user"`
}

func (a *AuthService) Login(identifier, password string) (*AuthToken, error) {
	user, err := a.userService.GetUserByIdentifier(identifier)
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

	a.attempts.reset(user.ID)
	user.Password = ""

	accessToken, _, accessTokenExpiresAt, err := a.generateToken(user, a.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	refreshToken, refreshJTI, _, err := a.generateToken(user, a.refreshTokenLifetime, "refresh")
	if err != nil {
		return nil, err
	}

	// store refresh token session
	if err := a.sessionStore.CreateSession(
		refreshJTI,
		user.ID,
		"refresh",
		time.Now().Add(a.refreshTokenLifetime),
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

func (a *AuthService) RefreshToken(refreshToken string) (*AuthToken, error) {
	claims, err := a.parseClaims(refreshToken)
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

	// Check if the refresh token session is active
	active, err := a.sessionStore.IsActive(jti, userID, "refresh", time.Now())
	if err != nil || !active {
		return nil, ErrInvalidToken
	}

	user, err := a.userService.GetUserByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user.Password = "" // Clear password from user object

	newAccessToken, _, accessTokenExpiresAt, err := a.generateToken(user, a.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	newRefreshToken, newRefreshJTI, _, err := a.generateToken(user, a.refreshTokenLifetime, "refresh")
	if err != nil {
		return nil, err
	}

	if err := a.sessionStore.CreateSession(
		newRefreshJTI,
		user.ID,
		"refresh",
		time.Now().Add(a.refreshTokenLifetime),
	); err != nil {
		return nil, err
	}

	// Revoke the old refresh token only after successfully creating the new session.
	// This ensures that if token generation or session creation fails, the old token
	// remains valid and the user can retry. If revocation fails, we log a warning but
	// don't fail the refresh operation - the old token will expire naturally, and
	// having two valid tokens temporarily is safer than logging the user out.
	err = a.sessionStore.RevokeSession(jti)
	if err != nil {
		slog.Warn("failed to revoke used refresh token session", "error", err)
	}

	return &AuthToken{
		Token:                newAccessToken,
		RefreshToken:         newRefreshToken,
		AccessTokenExpiresAt: accessTokenExpiresAt,
		User:                 user.ToPublicUser(),
	}, nil
}

func (a *AuthService) RevokeRefreshToken(tokenString string) error {
	claims, err := a.parseClaims(tokenString)
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

	return a.sessionStore.RevokeSession(jti)
}

func (a *AuthService) RevokeAllUserSessions(userID string) error {
	return a.sessionStore.RevokeAllSessionsForUser(userID)
}

func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *AuthService) parseClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secretKey, nil
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

func (a *AuthService) generateToken(user *User, duration time.Duration, typ string) (string, string, int64, error) {
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secretKey)
	if err != nil {
		return "", "", 0, err
	}
	return signed, jti, expiresAt, nil
}

func (a *AuthService) ValidateToken(tokenString string) (*User, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		// Ensure signing method is correct
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secretKey, nil
	})

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, ok := claims["sub"].(string)
	if !ok {
		return nil, ErrInvalidToken
	}

	return a.userService.GetUserByID(userID)
}
