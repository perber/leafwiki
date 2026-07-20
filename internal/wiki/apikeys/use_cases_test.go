package apikeys

import (
	"context"
	"testing"
	"time"

	coreauth "github.com/perber/wiki/internal/core/auth"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func setupAPIKeyUseCases(t *testing.T) (*CreateAPIKeyUseCase, *ListAPIKeysUseCase, *RevokeAPIKeyUseCase, *coreauth.UserService) {
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
	userService := coreauth.NewUserService(userStore)

	keyStore, err := coreauth.NewAPIKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAPIKeyStore: %v", err)
	}
	t.Cleanup(func() {
		if err := keyStore.Close(); err != nil {
			t.Errorf("Close api key store: %v", err)
		}
	})
	keyService := coreauth.NewAPIKeyService(keyStore, userService)

	return NewCreateAPIKeyUseCase(keyService), NewListAPIKeysUseCase(keyService), NewRevokeAPIKeyUseCase(keyService), userService
}

func TestCreateAPIKey_HappyPath(t *testing.T) {
	create, _, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("alice", "alice@example.com", "password123", coreauth.RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	out, err := create.Execute(context.Background(), CreateAPIKeyInput{
		Name: "agent key", UserID: owner.ID, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Key.Role != coreauth.RoleViewer {
		t.Errorf("expected default role %q, got %q", coreauth.RoleViewer, out.Key.Role)
	}
	if out.Secret == "" {
		t.Errorf("expected a non-empty secret")
	}
}

func TestCreateAPIKey_RejectsEmptyName(t *testing.T) {
	create, _, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("bob", "bob@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = create.Execute(context.Background(), CreateAPIKeyInput{Name: "  ", UserID: owner.ID, CreatedBy: "admin1"})
	var ve *sharederrors.ValidationErrors
	if err == nil {
		t.Fatalf("expected a validation error")
	}
	if !isValidationError(err, &ve) {
		t.Fatalf("expected *ValidationErrors, got %T: %v", err, err)
	}
	if !hasFieldError(ve, "name") {
		t.Errorf("expected a validation error on field 'name', got %+v", ve.Errors)
	}
}

func TestCreateAPIKey_RejectsMissingUserID(t *testing.T) {
	create, _, _, _ := setupAPIKeyUseCases(t)

	_, err := create.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: "", CreatedBy: "admin1"})
	var ve *sharederrors.ValidationErrors
	if !isValidationError(err, &ve) {
		t.Fatalf("expected *ValidationErrors, got %T: %v", err, err)
	}
	if !hasFieldError(ve, "userId") {
		t.Errorf("expected a validation error on field 'userId', got %+v", ve.Errors)
	}
}

func TestCreateAPIKey_RejectsInvalidRole(t *testing.T) {
	create, _, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("carol", "carol@example.com", "password123", coreauth.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err = create.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: owner.ID, Role: "superuser", CreatedBy: "admin1"})
	var ve *sharederrors.ValidationErrors
	if !isValidationError(err, &ve) {
		t.Fatalf("expected *ValidationErrors, got %T: %v", err, err)
	}
	if !hasFieldError(ve, "role") {
		t.Errorf("expected a validation error on field 'role', got %+v", ve.Errors)
	}
}

func TestCreateAPIKey_RejectsNonFutureExpiry(t *testing.T) {
	create, _, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("dave2", "dave2@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	past := time.Now().Add(-time.Hour)
	_, err = create.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: owner.ID, ExpiresAt: &past, CreatedBy: "admin1"})
	var ve *sharederrors.ValidationErrors
	if !isValidationError(err, &ve) {
		t.Fatalf("expected *ValidationErrors, got %T: %v", err, err)
	}
	if !hasFieldError(ve, "expiresAt") {
		t.Errorf("expected a validation error on field 'expiresAt', got %+v", ve.Errors)
	}
}

func TestCreateAPIKey_AcceptsFutureExpiry(t *testing.T) {
	create, _, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("dave3", "dave3@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	future := time.Now().Add(time.Hour)
	out, err := create.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: owner.ID, ExpiresAt: &future, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Key.ExpiresAt == nil || !out.Key.ExpiresAt.Equal(future) {
		t.Errorf("expected ExpiresAt %v, got %v", future, out.Key.ExpiresAt)
	}
}

func TestCreateAPIKey_PropagatesUnknownUserError(t *testing.T) {
	create, _, _, _ := setupAPIKeyUseCases(t)

	_, err := create.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: "no-such-user", CreatedBy: "admin1"})
	if err != coreauth.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestListAPIKeys(t *testing.T) {
	create, list, _, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("dave", "dave@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := create.Execute(context.Background(), CreateAPIKeyInput{Name: "k1", UserID: owner.ID, CreatedBy: "admin1"}); err != nil {
		t.Fatalf("Execute create: %v", err)
	}

	out, err := list.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute list: %v", err)
	}
	if len(out.Keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(out.Keys))
	}
}

func TestRevokeAPIKey(t *testing.T) {
	create, _, revoke, users := setupAPIKeyUseCases(t)
	owner, err := users.CreateUser("erin", "erin@example.com", "password123", coreauth.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	out, err := create.Execute(context.Background(), CreateAPIKeyInput{Name: "k1", UserID: owner.ID, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("Execute create: %v", err)
	}

	if err := revoke.Execute(context.Background(), RevokeAPIKeyInput{ID: out.Key.ID}); err != nil {
		t.Fatalf("Execute revoke: %v", err)
	}
}

func TestRevokeAPIKey_PropagatesNotFound(t *testing.T) {
	_, _, revoke, _ := setupAPIKeyUseCases(t)

	err := revoke.Execute(context.Background(), RevokeAPIKeyInput{ID: "missing"})
	if err != coreauth.ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

// ─── nil-service defense in depth (mirrors wiki/auth's ErrAuthDisabled guard) ─
//
// A nil *coreauth.APIKeyService means the Wiki composition root built it
// under --disable-auth (see wiki.go's initAuth). The HTTP routes are already
// RequireAdmin-gated so this should be unreachable via the API, but each use
// case defends its own invariant rather than trusting the caller.

func TestCreateAPIKey_NilServiceReturnsAPIKeysDisabled(t *testing.T) {
	uc := NewCreateAPIKeyUseCase(nil)
	_, err := uc.Execute(context.Background(), CreateAPIKeyInput{Name: "k", UserID: "u1", CreatedBy: "admin1"})
	if err != ErrAPIKeysDisabled {
		t.Fatalf("expected ErrAPIKeysDisabled, got %v", err)
	}
}

func TestListAPIKeys_NilServiceReturnsAPIKeysDisabled(t *testing.T) {
	uc := NewListAPIKeysUseCase(nil)
	_, err := uc.Execute(context.Background())
	if err != ErrAPIKeysDisabled {
		t.Fatalf("expected ErrAPIKeysDisabled, got %v", err)
	}
}

func TestRevokeAPIKey_NilServiceReturnsAPIKeysDisabled(t *testing.T) {
	uc := NewRevokeAPIKeyUseCase(nil)
	err := uc.Execute(context.Background(), RevokeAPIKeyInput{ID: "k1"})
	if err != ErrAPIKeysDisabled {
		t.Fatalf("expected ErrAPIKeysDisabled, got %v", err)
	}
}

// ─── test helpers ────────────────────────────────────────────────────────────

func isValidationError(err error, target **sharederrors.ValidationErrors) bool {
	ve, ok := err.(*sharederrors.ValidationErrors)
	if !ok {
		return false
	}
	*target = ve
	return true
}

func hasFieldError(ve *sharederrors.ValidationErrors, field string) bool {
	for _, fe := range ve.Errors {
		if fe.Field == field {
			return true
		}
	}
	return false
}
