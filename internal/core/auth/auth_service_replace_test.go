package auth

import (
	"sync"
	"testing"
	"time"
)

func TestAuthService_ReplaceUserStore_SwapsToNewUsers(t *testing.T) {
	oldDir := t.TempDir()
	oldStore, err := NewUserStore(oldDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserService(oldStore).CreateUser("old-user", "old@example.com", "old-password", "admin"); err != nil {
		t.Fatal(err)
	}
	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	authService := NewAuthService(NewUserService(oldStore), sessionStore, nil, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	t.Cleanup(func() { _ = authService.Close() })

	if _, err := authService.Login("old-user", "old-password"); err != nil {
		t.Fatalf("expected login against the original store to succeed: %v", err)
	}

	newDir := t.TempDir()
	newStore, err := NewUserStore(newDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserService(newStore).CreateUser("new-user", "new@example.com", "new-password", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := newStore.Close(); err != nil {
		t.Fatal(err)
	}

	if err := authService.ReplaceUserStore(newDir); err != nil {
		t.Fatalf("ReplaceUserStore failed: %v", err)
	}

	if _, err := authService.Login("old-user", "old-password"); err == nil {
		t.Error("expected the old user to no longer be reachable after ReplaceUserStore")
	}
	if _, err := authService.Login("new-user", "new-password"); err != nil {
		t.Errorf("expected the new user to be reachable after ReplaceUserStore: %v", err)
	}
}

func TestAuthService_ReplaceUserStore_ClosesPreviousStore(t *testing.T) {
	f := setupTestAuthService(t)
	t.Cleanup(func() { _ = f.Close() })

	newDir := t.TempDir()
	newStore, err := NewUserStore(newDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserService(newStore).CreateUser("new-user", "new@example.com", "new-password", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := newStore.Close(); err != nil {
		t.Fatal(err)
	}

	if err := f.ReplaceUserStore(newDir); err != nil {
		t.Fatalf("ReplaceUserStore failed: %v", err)
	}

	// The old store's *sql.DB was closed; any further query against it (via
	// the private field, reachable only within-package) should now fail.
	if _, err := f.userService.store.GetUserByUsername("testuser"); err == nil {
		t.Error("expected a query against the closed previous store to fail")
	}
}

func TestAuthService_ReplaceUserStore_ConcurrentLoginsDuringSwap(t *testing.T) {
	oldDir := t.TempDir()
	oldStore, err := NewUserStore(oldDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserService(oldStore).CreateUser("shared-user", "shared@example.com", "shared-password", "admin"); err != nil {
		t.Fatal(err)
	}
	sessionStore, err := NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	authService := NewAuthService(NewUserService(oldStore), sessionStore, nil, "test-secret-key-for-unit-tests-1", time.Hour, 24*time.Hour)
	t.Cleanup(func() { _ = authService.Close() })

	newDir := t.TempDir()
	newStore, err := NewUserStore(newDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserService(newStore).CreateUser("shared-user", "shared@example.com", "shared-password", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := newStore.Close(); err != nil {
		t.Fatal(err)
	}

	// Hammer Login/RefreshToken concurrently with a single ReplaceUserStore
	// swap, under -race. Requests are never expected to corrupt shared state
	// or crash — a transient failure exactly during the swap instant is
	// acceptable (see ReplaceUserStore's doc comment), a data race is not.
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = authService.Login("shared-user", "shared-password")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		if err := authService.ReplaceUserStore(newDir); err != nil {
			t.Errorf("ReplaceUserStore failed under concurrent load: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()

	// The swap must have taken effect by now regardless of request timing.
	if _, err := authService.Login("shared-user", "shared-password"); err != nil {
		t.Errorf("expected login to succeed against the swapped-in store after the race: %v", err)
	}
}

func TestAuthService_InvalidateAllSessions_RevokesActiveSessions(t *testing.T) {
	f := setupTestAuthService(t)
	t.Cleanup(func() { _ = f.Close() })

	if err := f.sessionStore.CreateSession("jti-a", "user-a", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := f.sessionStore.CreateSession("jti-b", "user-b", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	if err := f.InvalidateAllSessions(); err != nil {
		t.Fatalf("InvalidateAllSessions failed: %v", err)
	}

	for _, s := range []struct{ jti, userID string }{{"jti-a", "user-a"}, {"jti-b", "user-b"}} {
		active, err := f.sessionStore.IsActive(s.jti, s.userID, "refresh", time.Now())
		if err != nil {
			t.Fatalf("IsActive failed: %v", err)
		}
		if active {
			t.Errorf("expected session %s to be invalidated", s.jti)
		}
	}
}
