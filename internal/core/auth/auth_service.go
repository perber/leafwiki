package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthService struct {
	userService *UserService
	secretKey   []byte
}

func NewAuthService(userService *UserService, secret string) *AuthService {
	return &AuthService{
		userService: userService,
		secretKey:   []byte(secret),
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

	accessToken, err := a.generateToken(user, time.Hour*1, "access")
	if err != nil {
		return nil, err
	}

	refreshToken, err := a.generateToken(user, time.Hour*24*7, "refresh")
	if err != nil {
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

	user, err := a.userService.GetUserByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Clear sensitive information from user object
	user.Password = "" // Clear password from user object

	newAccessToken, err := a.generateToken(user, time.Hour*1, "access")
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := a.generateToken(user, time.Hour*24*7, "refresh")
	if err != nil {
		return nil, err
	}

	return &AuthToken{
		Token:        newAccessToken,
		RefreshToken: newRefreshToken,
		User:         user.ToPublicUser(),
	}, nil
}

func generateJTI() string {
	b := make([]byte, 16)
	rand.Read(b)
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

func (a *AuthService) generateToken(user *User, duration time.Duration, typ string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"role":  user.Role,
		"email": user.Email,
		"exp":   time.Now().Add(duration).Unix(),
		"iat":   time.Now().Unix(),
		"typ":   typ,
		"jti":   generateJTI(), // Unique identifier for the token
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.secretKey)
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
