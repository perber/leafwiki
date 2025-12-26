package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	userService          *UserService
	sessionStore         *SessionStore
	secretKey            []byte
	accessTokenLifetime  time.Duration
	refreshTokenLifetime time.Duration
}

func NewAuthService(userService *UserService, sessionStore *SessionStore, secret string, accessTokenTimeout, refreshTokenTimeout time.Duration) *AuthService {
	return &AuthService{
		userService:          userService,
		sessionStore:         sessionStore,
		secretKey:            []byte(secret),
		accessTokenLifetime:  accessTokenTimeout,
		refreshTokenLifetime: refreshTokenTimeout,
	}
}

type AuthToken struct {
	Token        string      `json:"token"`
	RefreshToken string      `json:"refresh_token"`
	User         *PublicUser `json:"user"`
}

func (a *AuthService) Login(identifier, password string) (*AuthToken, error) {
	user, err := a.userService.GetUserByEmailOrUsernameAndPassword(identifier, password)
	if err != nil {
		return nil, ErrUserInvalidCredentials
	}

	// Clear sensitive information from user object
	user.Password = "" // Clear password from user object

	accessToken, _, err := a.generateToken(user, a.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	refreshToken, refreshJTI, err := a.generateToken(user, a.refreshTokenLifetime, "refresh")
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
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user.ToPublicUser(),
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

	newAccessToken, _, err := a.generateToken(user, a.accessTokenLifetime, "access")
	if err != nil {
		return nil, err
	}

	newRefreshToken, newRefreshJTI, err := a.generateToken(user, a.refreshTokenLifetime, "refresh")
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

	// Revoke the old refresh token only after successfully creating the new session
	err = a.sessionStore.RevokeSession(jti)
	if err != nil {
		log.Printf("Warning: failed to revoke used refresh token session: %v", err)
	}

	return &AuthToken{
		Token:        newAccessToken,
		RefreshToken: newRefreshToken,
		User:         user.ToPublicUser(),
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

func generateJTI() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
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

func (a *AuthService) generateToken(user *User, duration time.Duration, typ string) (string, string, error) {
	jti := generateJTI()
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"role":  user.Role,
		"email": user.Email,
		"exp":   time.Now().Add(duration).Unix(),
		"iat":   time.Now().Unix(),
		"typ":   typ,
		"jti":   jti, // Unique identifier for the token
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(a.secretKey)
	if err != nil {
		return "", "", err
	}
	return signed, jti, nil
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
