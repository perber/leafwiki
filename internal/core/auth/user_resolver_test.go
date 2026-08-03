package auth

import (
	"testing"
	"time"
)

func setupTestUserResolver(t *testing.T) (*UserResolver, *UserService) {
	t.Helper()
	service := setupTestUserService(t)
	resolver, err := NewUserResolver(func() *UserService { return service })
	if err != nil {
		t.Fatalf("NewUserResolver failed: %v", err)
	}
	return resolver, service
}

func TestUserResolver_ResolveUserLabel_ResolvesKnownUser(t *testing.T) {
	resolver, service := setupTestUserResolver(t)

	user, err := service.CreateUser("alice", "alice@example.com", "secure", "editor")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	label, err := resolver.ResolveUserLabel(user.ID)
	if err != nil {
		t.Fatalf("ResolveUserLabel failed: %v", err)
	}
	if label == nil || label.Username != "alice" {
		t.Fatalf("expected label for alice, got %+v", label)
	}
}

func TestUserResolver_ResolveUserLabel_CachesLookupMissesSoRepeatedCallsDontHitTheStore(t *testing.T) {
	resolver, _ := setupTestUserResolver(t)

	// First call for an ID that was never a real user (e.g. a deleted
	// author still referenced by old page frontmatter) genuinely misses
	// and returns the store's error.
	label, err := resolver.ResolveUserLabel("does-not-exist")
	if err == nil {
		t.Fatalf("expected an error for an unresolvable user id, got nil")
	}
	if label != nil {
		t.Fatalf("expected nil label for an unresolvable user id, got %+v", label)
	}

	// The miss must be cached: resolved should now hold an entry for this
	// ID (even though its value is nil), so a second call takes the fast
	// path instead of hitting the store again.
	resolver.mu.RLock()
	_, ok := resolver.resolved["does-not-exist"]
	resolver.mu.RUnlock()
	if !ok {
		t.Fatalf("expected the miss to be cached in resolved, but no entry was found")
	}

	label, err = resolver.ResolveUserLabel("does-not-exist")
	if err != nil {
		t.Fatalf("expected the cached miss to resolve without an error, got %v", err)
	}
	if label != nil {
		t.Fatalf("expected nil label for a cached miss, got %+v", label)
	}
}

func TestUserResolver_Reload_ClearsCachedMisses(t *testing.T) {
	resolver, _ := setupTestUserResolver(t)

	if _, err := resolver.ResolveUserLabel("does-not-exist"); err == nil {
		t.Fatalf("expected an error for an unresolvable user id, got nil")
	}
	resolver.mu.RLock()
	_, ok := resolver.resolved["does-not-exist"]
	resolver.mu.RUnlock()
	if !ok {
		t.Fatalf("expected the miss to be cached before Reload")
	}

	if err := resolver.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	resolver.mu.RLock()
	_, ok = resolver.resolved["does-not-exist"]
	resolver.mu.RUnlock()
	if ok {
		t.Fatalf("expected Reload to clear cached misses, but the entry is still present")
	}
}

// TestUserResolver_ResolveUserLabel_DoesNotCacheStoreUnavailableAsMiss is the
// regression test for "UserService Swallows the Suspended-Store Error Into
// ErrUserNotFound": before GetUserByID distinguished the two, ResolveUserLabel
// permanently cached ANY error — including a transient "store suspended for
// an in-progress live restore" error — as a cached miss, so a request landing
// in that sub-second window would forever resolve nil for that user even
// after the restore completed and the store came back.
func TestUserResolver_ResolveUserLabel_DoesNotCacheStoreUnavailableAsMiss(t *testing.T) {
	resolver, service := setupTestUserResolver(t)

	user, err := service.CreateUser("bob", "bob@example.com", "secure", "editor")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := service.suspendStore(); err != nil {
		t.Fatalf("suspendStore failed: %v", err)
	}

	label, err := resolver.ResolveUserLabel(user.ID)
	if err == nil {
		t.Fatalf("expected an error while the store is suspended, got nil label=%+v", label)
	}
	if label != nil {
		t.Fatalf("expected nil label for a suspended-store error, got %+v", label)
	}

	resolver.mu.RLock()
	_, cached := resolver.resolved[user.ID]
	resolver.mu.RUnlock()
	if cached {
		t.Fatalf("expected the transient store-unavailable error NOT to be cached as a miss, but resolved has an entry for %q", user.ID)
	}
}

// TestUserResolver_ResolveUserLabel_ReflectsLiveRestore is a regression test
// for the UserResolver half of "User-Management Routes Go Stale After Live
// Restore": a UserResolver built from a plain *UserService (as wiki.go's
// initAuth does with w.user) never sees a live restore's
// AuthService.ReplaceUserStore swap, so a cache-miss lookup for a user that
// only exists in the restored store fails forever.
func TestUserResolver_ResolveUserLabel_ReflectsLiveRestore(t *testing.T) {
	preStore, err := NewUserStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewUserStore(pre): %v", err)
	}
	preSvc := NewUserService(preStore)

	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		if err := sessionStore.Close(); err != nil {
			t.Errorf("Close session store: %v", err)
		}
	})
	sessions := NewSessionManager(sessionStore, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	authSvc := NewAuthService(preSvc, sessions, nil)
	t.Cleanup(func() { _ = authSvc.Close() })

	resolver, err := NewUserResolver(authSvc.UserService)
	if err != nil {
		t.Fatalf("NewUserResolver: %v", err)
	}

	postDir := t.TempDir()
	postStore, err := NewUserStore(postDir)
	if err != nil {
		t.Fatalf("NewUserStore(post): %v", err)
	}
	postSvc := NewUserService(postStore)
	postUser, err := postSvc.CreateUser("post-restore-admin", "post-restore-admin@example.com", "password123", RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser(post): %v", err)
	}
	if err := postStore.Close(); err != nil {
		t.Fatalf("Close(postStore): %v", err)
	}

	// Simulates what a live restore does to AuthService.
	if err := authSvc.ReplaceUserStore(postDir); err != nil {
		t.Fatalf("ReplaceUserStore: %v", err)
	}

	label, err := resolver.ResolveUserLabel(postUser.ID)
	if err != nil {
		t.Fatalf("ResolveUserLabel failed after a live restore swapped the user store: %v", err)
	}
	if label == nil || label.Username != "post-restore-admin" {
		t.Fatalf("expected the resolver to see the post-restore user, got %+v", label)
	}
}
