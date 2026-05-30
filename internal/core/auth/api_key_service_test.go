package auth

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func setupTestAPIKeyService(t *testing.T) (*UserService, *APIKeyStore, *APIKeyService, *User) {
	t.Helper()

	storageDir := t.TempDir()
	userStore, err := NewUserStore(storageDir)
	if err != nil {
		t.Fatalf("NewUserStore failed: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(userStore.Close, t) })

	userService := NewUserService(userStore)
	apiKeyStore, err := NewAPIKeyStore(storageDir)
	if err != nil {
		t.Fatalf("NewAPIKeyStore failed: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(apiKeyStore.Close, t) })

	apiKeyService := NewAPIKeyService(apiKeyStore, userService)
	user, err := userService.CreateUser("editor", "editor@example.com", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return userService, apiKeyStore, apiKeyService, user
}

func TestAPIKeyServiceCreateStoresOnlyHashAndListsMetadata(t *testing.T) {
	_, store, service, user := setupTestAPIKeyService(t)

	created, err := service.CreateAPIKey(user.ID, "  Local Codex  ", user.ID)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	if created.Secret == "" || !strings.HasPrefix(created.Secret, "lwk_"+created.Key.ID+"_") {
		t.Fatalf("secret = %q, want lwk_<id>_<secret>", created.Secret)
	}
	if created.Key.Name != "Local Codex" {
		t.Fatalf("key name = %q, want trimmed name", created.Key.Name)
	}
	if got, want := created.Key.UserID, user.ID; got != want {
		t.Fatalf("key user id = %q, want %q", got, want)
	}
	if got, want := created.Key.CreatedByUserID, user.ID; got != want {
		t.Fatalf("createdBy = %q, want %q", got, want)
	}
	if got, want := created.Key.Scopes, []string{MCPAPIKeyScope}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("scopes = %#v, want %#v", got, want)
	}
	if created.Key.Prefix == "" || created.Key.Last4 == "" {
		t.Fatalf("prefix/last4 must be set: %#v", created.Key)
	}
	if created.Key.CreatedAt.IsZero() {
		t.Fatalf("createdAt must be set")
	}
	if created.Key.LastUsedAt != nil || created.Key.RevokedAt != nil {
		t.Fatalf("new key lastUsedAt/revokedAt = %#v/%#v, want nil", created.Key.LastUsedAt, created.Key.RevokedAt)
	}

	var rawSecretCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE secret_hash LIKE '%' || ? || '%'`, created.Secret).Scan(&rawSecretCount); err != nil {
		t.Fatalf("query raw secret count: %v", err)
	}
	if rawSecretCount != 0 {
		t.Fatalf("database contains raw secret")
	}

	listed, err := service.ListAPIKeys(user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("listed keys = %d, want 1", len(listed))
	}
	if listed[0].ID != created.Key.ID || listed[0].Name != "Local Codex" {
		t.Fatalf("listed key = %#v, want created metadata", listed[0])
	}
}

func TestAPIKeyServiceVerifyRejectsMalformedWrongSecretRevokedAndDeletedUser(t *testing.T) {
	userService, _, service, user := setupTestAPIKeyService(t)

	created, err := service.CreateAPIKey(user.ID, "MCP client", user.ID)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	verified, err := service.VerifyAPIKey(created.Secret)
	if err != nil {
		t.Fatalf("VerifyAPIKey failed: %v", err)
	}
	if verified.User.ID != user.ID || verified.User.Role != RoleEditor {
		t.Fatalf("verified user = %#v, want current editor", verified.User)
	}
	if verified.Key.LastUsedAt == nil {
		t.Fatalf("VerifyAPIKey should update lastUsedAt")
	}

	badInputs := []string{
		"",
		"not-an-api-key",
		"lwk_missing_parts",
		"lwk_" + created.Key.ID + "_wrongsecret",
	}
	for _, input := range badInputs {
		if _, err := service.VerifyAPIKey(input); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("VerifyAPIKey(%q) err = %v, want ErrInvalidToken", input, err)
		}
	}

	if err := service.RevokeAPIKey(user.ID, created.Key.ID); err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}
	if _, err := service.VerifyAPIKey(created.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("revoked VerifyAPIKey err = %v, want ErrInvalidToken", err)
	}
	listed, err := service.ListAPIKeys(user.ID)
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("revoked key listed as active: %#v", listed)
	}

	second, err := service.CreateAPIKey(user.ID, "After revoke", user.ID)
	if err != nil {
		t.Fatalf("CreateAPIKey second failed: %v", err)
	}
	if _, err := userService.UpdateUser(user.ID, user.Username, user.Email, "", RoleViewer); err != nil {
		t.Fatalf("UpdateUser role failed: %v", err)
	}
	verified, err = service.VerifyAPIKey(second.Secret)
	if err != nil {
		t.Fatalf("VerifyAPIKey after role update failed: %v", err)
	}
	if verified.User.Role != RoleViewer {
		t.Fatalf("verified role = %q, want current viewer role", verified.User.Role)
	}
	if err := userService.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	if _, err := service.VerifyAPIKey(second.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("deleted-user VerifyAPIKey err = %v, want ErrInvalidToken", err)
	}
}

func TestAPIKeyStoreRevocationIsScopedToUser(t *testing.T) {
	userService, _, service, user := setupTestAPIKeyService(t)
	other, err := userService.CreateUser("other", "other@example.com", "password123", RoleEditor)
	if err != nil {
		t.Fatalf("CreateUser other failed: %v", err)
	}

	created, err := service.CreateAPIKey(other.ID, "Other key", user.ID)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	if err := service.RevokeAPIKey(user.ID, created.Key.ID); !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("wrong-user RevokeAPIKey err = %v, want not found", err)
	}
	if _, err := service.VerifyAPIKey(created.Secret); err != nil {
		t.Fatalf("wrong-user revoke should not revoke key: %v", err)
	}
}

func TestAPIKeyStoreMarkUsedRejectsRevokedKey(t *testing.T) {
	_, store, service, user := setupTestAPIKeyService(t)

	created, err := service.CreateAPIKey(user.ID, "Race key", user.ID)
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}
	if err := service.RevokeAPIKey(user.ID, created.Key.ID); err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}

	if err := store.MarkAPIKeyUsed(created.Key.ID, service.now()); !errors.Is(err, ErrAPIKeyNotFound) {
		t.Fatalf("MarkAPIKeyUsed revoked key err = %v, want ErrAPIKeyNotFound", err)
	}
}
