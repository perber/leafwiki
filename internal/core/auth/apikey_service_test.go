package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

func setupTestAPIKeyService(t *testing.T) (*APIKeyService, *UserService) {
	t.Helper()

	userStore, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore err: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(userStore.Close, t) })
	userService := NewUserService(userStore)

	keyStore, err := NewAPIKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewAPIKeyStore err: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(keyStore.Close, t) })

	return NewAPIKeyService(keyStore, userService), userService
}

func mustCreateUser(t *testing.T, users *UserService, username, role string) *User {
	t.Helper()
	user, err := users.CreateUser(username, username+"@example.com", "password123", role)
	if err != nil {
		t.Fatalf("CreateUser(%s) err: %v", username, err)
	}
	return user
}

// ─── hashSecret / timing-equalization ───────────────────────────────────────

func TestHashSecret_DeterministicAndDistinct(t *testing.T) {
	h1 := hashSecret("secret-a")
	h2 := hashSecret("secret-a")
	h3 := hashSecret("secret-b")
	if h1 != h2 {
		t.Fatalf("expected hashSecret to be deterministic, got %q vs %q", h1, h2)
	}
	if h1 == h3 {
		t.Fatalf("expected different secrets to hash differently")
	}
	if h1 == dummySecretHash {
		t.Fatalf("real secret hash must not collide with the dummy hash used for timing equalization")
	}
}

func TestAPIKeyService_Resolve_UnknownPrefixAndWrongSecretBothInvalid(t *testing.T) {
	// Both cases must go through the same hash-and-compare path (unknown
	// prefix compares against dummySecretHash) so a caller can't distinguish
	// them by response shape or timing. This asserts the observable contract;
	// see hashSecret's doc comment for why the timing property matters.
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "carol2", RoleViewer)

	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	prefix, _, _ := parseKeyToken(token)

	unknownPrefixToken := apiKeyTokenPrefix + "deadbeef_" + strings.Repeat("a", 64)
	wrongSecretToken := apiKeyTokenPrefix + prefix + "_" + strings.Repeat("b", 64)

	if _, err := svc.Resolve(unknownPrefixToken); err != ErrAPIKeyInvalid {
		t.Fatalf("unknown prefix: expected ErrAPIKeyInvalid, got %v", err)
	}
	if _, err := svc.Resolve(wrongSecretToken); err != ErrAPIKeyInvalid {
		t.Fatalf("wrong secret: expected ErrAPIKeyInvalid, got %v", err)
	}
}

// ─── token generation / parsing ─────────────────────────────────────────────

func TestGenerateKeyToken_ProducesNonEmptyDistinctValues(t *testing.T) {
	prefix1, secret1, err := generateKeyToken()
	if err != nil {
		t.Fatalf("generateKeyToken err: %v", err)
	}
	prefix2, secret2, err := generateKeyToken()
	if err != nil {
		t.Fatalf("generateKeyToken err: %v", err)
	}
	if prefix1 == "" || secret1 == "" {
		t.Fatalf("expected non-empty prefix/secret")
	}
	if prefix1 == prefix2 || secret1 == secret2 {
		t.Fatalf("expected distinct values across calls")
	}
}

func TestParseKeyToken_RoundTrip(t *testing.T) {
	prefix, secret, err := generateKeyToken()
	if err != nil {
		t.Fatalf("generateKeyToken err: %v", err)
	}
	token := apiKeyTokenPrefix + prefix + "_" + secret

	gotPrefix, gotSecret, ok := parseKeyToken(token)
	if !ok {
		t.Fatalf("expected token to parse")
	}
	if gotPrefix != prefix || gotSecret != secret {
		t.Fatalf("got (%q, %q), want (%q, %q)", gotPrefix, gotSecret, prefix, secret)
	}
}

func TestParseKeyToken_RejectsMalformed(t *testing.T) {
	cases := []string{
		"",
		"not-a-key-token",
		"lw_missingsecret",
		"lw__",
		"lw_prefixonly_",
		"lw__secretonly",
	}
	for _, tc := range cases {
		if _, _, ok := parseKeyToken(tc); ok {
			t.Fatalf("expected %q to be rejected as malformed", tc)
		}
	}
}

// ─── intersectRole ───────────────────────────────────────────────────────────

func TestIntersectRole_NeverWidensPermission(t *testing.T) {
	tests := []struct {
		userRole, keyRole, want string
	}{
		{RoleAdmin, RoleAdmin, RoleAdmin},
		{RoleAdmin, RoleEditor, RoleEditor},
		{RoleAdmin, RoleViewer, RoleViewer},
		{RoleEditor, RoleAdmin, RoleEditor},
		{RoleEditor, RoleEditor, RoleEditor},
		{RoleEditor, RoleViewer, RoleViewer},
		{RoleViewer, RoleAdmin, RoleViewer},
		{RoleViewer, RoleEditor, RoleViewer},
		{RoleViewer, RoleViewer, RoleViewer},
	}
	for _, tc := range tests {
		got := intersectRole(tc.userRole, tc.keyRole)
		if got != tc.want {
			t.Errorf("intersectRole(%q, %q) = %q, want %q", tc.userRole, tc.keyRole, got, tc.want)
		}
	}
}

func TestIntersectRole_UnknownRoleFailsSafeToViewer(t *testing.T) {
	if got := intersectRole("bogus", RoleAdmin); got != RoleViewer {
		t.Errorf("intersectRole(bogus, admin) = %q, want %q (fail safe)", got, RoleViewer)
	}
	if got := intersectRole(RoleAdmin, "bogus"); got != RoleViewer {
		t.Errorf("intersectRole(admin, bogus) = %q, want %q (fail safe)", got, RoleViewer)
	}
}

// ─── CreateAPIKey ────────────────────────────────────────────────────────────

func TestAPIKeyService_CreateAPIKey_DefaultsToViewerRole(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "alice", RoleEditor)

	key, token, err := svc.CreateAPIKey(CreateAPIKeyParams{
		Name: "agent key", UserID: owner.ID, CreatedBy: "admin1",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if key.Role != RoleViewer {
		t.Fatalf("expected default role %q, got %q", RoleViewer, key.Role)
	}
	if !strings.HasPrefix(token, apiKeyTokenPrefix) {
		t.Fatalf("expected token to start with %q, got %q", apiKeyTokenPrefix, token)
	}
	if strings.Contains(key.KeyHash, token) {
		t.Fatalf("stored hash must not contain the plaintext token")
	}
}

func TestAPIKeyService_CreateAPIKey_RejectsInvalidRole(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "bob", RoleAdmin)

	_, _, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, Role: "superuser", CreatedBy: "admin1"})
	if err != ErrUserInvalidRole {
		t.Fatalf("expected ErrUserInvalidRole, got %v", err)
	}
}

func TestAPIKeyService_CreateAPIKey_RejectsUnknownUser(t *testing.T) {
	svc, _ := setupTestAPIKeyService(t)

	_, _, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: "no-such-user", CreatedBy: "admin1"})
	if err != ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

// ─── Resolve ─────────────────────────────────────────────────────────────────

func TestAPIKeyService_Resolve_HappyPath(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "carol", RoleViewer)

	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	resolved, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if resolved.ID != owner.ID {
		t.Fatalf("resolved user ID = %q, want %q", resolved.ID, owner.ID)
	}
	if resolved.Password != "" {
		t.Fatalf("expected resolved user's password to be cleared")
	}
}

func TestAPIKeyService_Resolve_NarrowsRoleToKeyRole(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "dave", RoleAdmin)

	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, Role: RoleEditor, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	resolved, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if resolved.Role != RoleEditor {
		t.Fatalf("resolved role = %q, want %q (narrowed by key role)", resolved.Role, RoleEditor)
	}
}

func TestAPIKeyService_Resolve_NarrowsRoleToUserRole(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	// Owner is only a viewer; an admin-role key must not widen that.
	owner := mustCreateUser(t, users, "erin", RoleViewer)

	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, Role: RoleAdmin, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	resolved, err := svc.Resolve(token)
	if err != nil {
		t.Fatalf("Resolve err: %v", err)
	}
	if resolved.Role != RoleViewer {
		t.Fatalf("resolved role = %q, want %q (capped by owner's role)", resolved.Role, RoleViewer)
	}
}

func TestAPIKeyService_Resolve_RejectsMalformedToken(t *testing.T) {
	svc, _ := setupTestAPIKeyService(t)

	if _, err := svc.Resolve("not-a-token"); err != ErrAPIKeyInvalid {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestAPIKeyService_Resolve_RejectsUnknownPrefix(t *testing.T) {
	svc, _ := setupTestAPIKeyService(t)

	if _, err := svc.Resolve(apiKeyTokenPrefix + "deadbeef_" + strings.Repeat("a", 64)); err != ErrAPIKeyInvalid {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestAPIKeyService_Resolve_RejectsWrongSecret(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "frank", RoleViewer)

	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	prefix, _, _ := parseKeyToken(token)
	tampered := apiKeyTokenPrefix + prefix + "_" + strings.Repeat("f", 64)

	if _, err := svc.Resolve(tampered); err != ErrAPIKeyInvalid {
		t.Fatalf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestAPIKeyService_Resolve_RejectsRevokedKey(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "grace", RoleViewer)

	key, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if err := svc.RevokeAPIKey(key.ID); err != nil {
		t.Fatalf("RevokeAPIKey err: %v", err)
	}

	if _, err := svc.Resolve(token); err != ErrAPIKeyRevoked {
		t.Fatalf("expected ErrAPIKeyRevoked, got %v", err)
	}
}

func TestAPIKeyService_Resolve_RejectsExpiredKey(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "heidi", RoleViewer)

	past := time.Now().Add(-time.Hour)
	_, token, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k", UserID: owner.ID, ExpiresAt: &past, CreatedBy: "admin1"})
	if err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	if _, err := svc.Resolve(token); err != ErrAPIKeyExpired {
		t.Fatalf("expected ErrAPIKeyExpired, got %v", err)
	}
}

func TestAPIKeyService_ListAPIKeys(t *testing.T) {
	svc, users := setupTestAPIKeyService(t)
	owner := mustCreateUser(t, users, "ivan", RoleViewer)

	if _, _, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k1", UserID: owner.ID, CreatedBy: "admin1"}); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if _, _, err := svc.CreateAPIKey(CreateAPIKeyParams{Name: "k2", UserID: owner.ID, CreatedBy: "admin1"}); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	keys, err := svc.ListAPIKeys()
	if err != nil {
		t.Fatalf("ListAPIKeys err: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
}
