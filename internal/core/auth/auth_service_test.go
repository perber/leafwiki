package auth

import (
	"testing"
	"time"
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

	authService := NewAuthService(userService, sessionStore, "mysecretkey", 1*time.Hour, 24*time.Hour*7)
	return authService
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
