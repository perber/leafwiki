package auth

import "testing"

func setupTestUserResolver(t *testing.T) (*UserResolver, *UserService) {
	t.Helper()
	service := setupTestUserService(t)
	resolver, err := NewUserResolver(service)
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
