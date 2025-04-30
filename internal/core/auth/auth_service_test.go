package auth

import (
	"testing"
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

	authService := NewAuthService(userService, "mysecretkey")
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
