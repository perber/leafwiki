package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

func setupTestAPIKeyStore(t *testing.T) *APIKeyStore {
	t.Helper()
	store, err := NewAPIKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to create api key store: %v", err)
	}
	return store
}

func TestAPIKeyStore_CreatesDatabaseInStorageDir(t *testing.T) {
	storageDir := t.TempDir()
	store, err := NewAPIKeyStore(storageDir)
	if err != nil {
		t.Fatalf("NewAPIKeyStore err: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := os.Stat(filepath.Join(storageDir, "api_keys.db")); err != nil {
		t.Fatalf("expected api_keys.db in storage dir, got err: %v", err)
	}
}

func TestAPIKeyStore_CreateAndGetByPrefix(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	key := &APIKey{
		ID:        "k1",
		Name:      "agent key",
		UserID:    "u1",
		Prefix:    "ab12cd",
		KeyHash:   "hashed",
		Role:      RoleViewer,
		CreatedBy: "admin1",
		CreatedAt: time.Now(),
	}
	if err := store.CreateAPIKey(key); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	got, err := store.GetByPrefix("ab12cd")
	if err != nil {
		t.Fatalf("GetByPrefix err: %v", err)
	}
	if got.ID != key.ID || got.Name != key.Name || got.UserID != key.UserID || got.Role != key.Role {
		t.Fatalf("got %+v, want fields matching %+v", got, key)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expected nil ExpiresAt, got %v", got.ExpiresAt)
	}
	if got.RevokedAt != nil {
		t.Fatalf("expected nil RevokedAt for new key, got %v", got.RevokedAt)
	}
}

func TestAPIKeyStore_GetByPrefix_NotFound(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := store.GetByPrefix("missing"); err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKeyStore_GetByID_NotFound(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if _, err := store.GetByID("missing"); err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKeyStore_CreateAPIKey_DuplicatePrefixCollides(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	base := &APIKey{ID: "k1", Name: "n1", UserID: "u1", Prefix: "dup123", KeyHash: "h1", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(base); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	dup := &APIKey{ID: "k2", Name: "n2", UserID: "u1", Prefix: "dup123", KeyHash: "h2", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(dup); err != ErrAPIKeyPrefixCollision {
		t.Fatalf("expected ErrAPIKeyPrefixCollision, got %v", err)
	}
}

func TestAPIKeyStore_ListAll_OrderedNewestFirst(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	older := &APIKey{ID: "k1", Name: "older", UserID: "u1", Prefix: "p1", KeyHash: "h1", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now().Add(-time.Hour)}
	newer := &APIKey{ID: "k2", Name: "newer", UserID: "u1", Prefix: "p2", KeyHash: "h2", Role: RoleEditor, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(older); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if err := store.CreateAPIKey(newer); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	keys, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll err: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].ID != "k2" || keys[1].ID != "k1" {
		t.Fatalf("expected newest first, got order %s, %s", keys[0].ID, keys[1].ID)
	}
}

func TestAPIKeyStore_Revoke(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	key := &APIKey{ID: "k1", Name: "n1", UserID: "u1", Prefix: "p1", KeyHash: "h1", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(key); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	if err := store.Revoke("k1"); err != nil {
		t.Fatalf("Revoke err: %v", err)
	}

	got, err := store.GetByID("k1")
	if err != nil {
		t.Fatalf("GetByID err: %v", err)
	}
	if got.RevokedAt == nil {
		t.Fatalf("expected RevokedAt to be set after revoke")
	}
	if got.IsActive(time.Now()) {
		t.Fatalf("expected revoked key to be inactive")
	}
}

func TestAPIKeyStore_Revoke_NotFound(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	if err := store.Revoke("missing"); err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound, got %v", err)
	}
}

func TestAPIKeyStore_Revoke_Idempotent(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	key := &APIKey{ID: "k1", Name: "n1", UserID: "u1", Prefix: "p1", KeyHash: "h1", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(key); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}
	if err := store.Revoke("k1"); err != nil {
		t.Fatalf("first Revoke err: %v", err)
	}
	first, _ := store.GetByID("k1")

	time.Sleep(1100 * time.Millisecond) // ensure a distinguishable unix-second boundary
	if err := store.Revoke("k1"); err != nil {
		t.Fatalf("second Revoke err: %v", err)
	}
	second, _ := store.GetByID("k1")

	if !first.RevokedAt.Equal(*second.RevokedAt) {
		t.Fatalf("expected revoked_at to stay at first revocation time, got %v then %v", first.RevokedAt, second.RevokedAt)
	}
}

func TestAPIKeyStore_TouchLastUsed(t *testing.T) {
	store := setupTestAPIKeyStore(t)
	defer test_utils.WrapCloseWithErrorCheck(store.Close, t)

	key := &APIKey{ID: "k1", Name: "n1", UserID: "u1", Prefix: "p1", KeyHash: "h1", Role: RoleViewer, CreatedBy: "admin1", CreatedAt: time.Now()}
	if err := store.CreateAPIKey(key); err != nil {
		t.Fatalf("CreateAPIKey err: %v", err)
	}

	now := time.Now()
	if err := store.TouchLastUsed("k1", now); err != nil {
		t.Fatalf("TouchLastUsed err: %v", err)
	}

	got, err := store.GetByID("k1")
	if err != nil {
		t.Fatalf("GetByID err: %v", err)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(time.Unix(now.Unix(), 0)) {
		t.Fatalf("expected LastUsedAt ~= %v, got %v", now, got.LastUsedAt)
	}
}

func TestAPIKey_IsActive_ExpiredIsInactive(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	key := &APIKey{ExpiresAt: &past}
	if key.IsActive(time.Now()) {
		t.Fatalf("expected expired key to be inactive")
	}
}

func TestAPIKey_IsActive_NoExpiryIsActive(t *testing.T) {
	key := &APIKey{}
	if !key.IsActive(time.Now()) {
		t.Fatalf("expected key with no expiry and no revocation to be active")
	}
}
