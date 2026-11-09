package auth

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/perber/wiki/internal/test_utils"
)

// The username and the email address name one account, so they share one
// budget per client.
func TestAuthService_Login_UsernameAndEmailShareOneBudget(t *testing.T) {
	authService := setupTestAuthService(t)
	freezeLoginClocks(authService)

	identifiers := []string{"testuser", "test@example.com"}
	for i := 0; i < loginFreeAttempts; i++ {
		id := identifiers[i%len(identifiers)]
		if _, err := authService.Login(id, "wrong-guess", testClient); !errors.Is(err, ErrUserInvalidCredentials) {
			t.Fatalf("attempt %d with %q: expected ErrUserInvalidCredentials, got %v", i+1, id, err)
		}
	}

	for _, id := range identifiers {
		if _, err := authService.Login(id, "securepass", testClient); !errors.Is(err, ErrLoginThrottled) {
			t.Errorf("Login(%q) with the right password: expected ErrLoginThrottled, got %v", id, err)
		}
	}
	if n := len(authService.attempts.entries); n != 1 {
		t.Errorf("expected one entry for both identifiers, got %d", n)
	}
}

// A right password after the wait gives the client a fresh set of free attempts.
func TestAuthService_Login_SuccessAfterTheWaitRestoresTheFreeAttempts(t *testing.T) {
	authService := setupTestAuthService(t)
	clock := freezeLoginClocks(authService)

	for i := 0; i < loginFreeAttempts; i++ {
		mustBeInvalidCredentials(t, authService, "wrong-guess", testClient)
	}
	clock.Advance(loginBackoffBase)
	if _, err := authService.Login("testuser", "securepass", testClient); err != nil {
		t.Fatalf("login after the wait: %v", err)
	}

	// Without the reset the very next typo would start a longer wait.
	for i := 0; i < loginFreeAttempts; i++ {
		mustBeInvalidCredentials(t, authService, "wrong-guess", testClient)
	}
	if _, err := authService.Login("testuser", "securepass", testClient); !errors.Is(err, ErrLoginThrottled) {
		t.Fatalf("expected a wait after the new free attempts are used, got %v", err)
	}
	clock.Advance(loginBackoffBase)
	if _, err := authService.Login("testuser", "securepass", testClient); err != nil {
		t.Fatalf("the wait after a reset must be the base wait again, got %v", err)
	}
}

// After a restore swaps the user store the tracker keeps working: entries of
// the old store are harmless, and users of the new store get their own budget.
func TestAuthService_ReplaceUserStore_LeavesTheLoginThrottleUsable(t *testing.T) {
	authService := setupTestAuthService(t)
	freezeLoginClocks(authService)

	for i := 0; i < loginFreeAttempts; i++ {
		mustBeInvalidCredentials(t, authService, "wrong-guess", testClient)
	}
	if _, err := authService.Login("testuser", "securepass", testClient); !errors.Is(err, ErrLoginThrottled) {
		t.Fatalf("expected a wait before the swap, got %v", err)
	}

	newDir := t.TempDir()
	newStore, err := NewUserStore(newDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(newStore.Close, t) })
	if _, err := NewUserService(newStore).CreateUser("restored-user", "restored@example.com", "restored-password", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := newStore.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = authService.Close() })
	if err := authService.ReplaceUserStore(newDir); err != nil {
		t.Fatalf("ReplaceUserStore: %v", err)
	}

	if _, err := authService.Login("restored-user", "restored-password", testClient); err != nil {
		t.Fatalf("login against the restored store: %v", err)
	}
	for i := 0; i < loginFreeAttempts; i++ {
		if _, err := authService.Login("restored-user", "wrong-guess", testClient); !errors.Is(err, ErrUserInvalidCredentials) {
			t.Fatalf("restored user, attempt %d: got %v", i+1, err)
		}
	}
	if _, err := authService.Login("restored-user", "restored-password", testClient); !errors.Is(err, ErrLoginThrottled) {
		t.Fatalf("the restored user is throttled like any other, got %v", err)
	}
}

// Many clients logging in for one account at the same time each get their own
// entry and none of them is lost (run with -race).
func TestAuthService_Login_ParallelClientsForOneAccountKeepTheirOwnEntries(t *testing.T) {
	authService := setupTestAuthService(t)
	freezeLoginClocks(authService)

	const clients = 24
	var wg sync.WaitGroup
	errs := make(chan error, clients)
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(client Client) {
			defer wg.Done()
			if _, err := authService.Login("testuser", "wrong-guess", client); !errors.Is(err, ErrUserInvalidCredentials) {
				errs <- fmt.Errorf("client %s: expected ErrUserInvalidCredentials, got %v", client, err)
			}
		}(Client(fmt.Sprintf("198.51.100.%d", i+1)))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	if n := len(authService.attempts.entries); n != clients {
		t.Errorf("expected %d entries, got %d", clients, n)
	}
}

// The cap holds with real account IDs and client buckets, and the newest pair
// is always tracked.
func TestAuthService_Login_TableCapHoldsWithRealKeys(t *testing.T) {
	authService := setupTestAuthService(t)
	clock := freezeLoginClocks(authService)
	const limit = 8
	authService.attempts.maxEntries = limit

	var last Client
	for i := 0; i < 3*limit; i++ {
		last = Client(fmt.Sprintf("2001:db8:%x::", i+1))
		mustBeInvalidCredentials(t, authService, "wrong-guess", last)
		clock.Advance(time.Second)
		if n := len(authService.attempts.entries); n > limit {
			t.Fatalf("after %d clients the table holds %d entries, cap is %d", i+1, n, limit)
		}
	}

	user, err := authService.users().GetUserByIdentifier("testuser")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := authService.attempts.entries[loginKey{userID: user.ID, client: last}]; !ok {
		t.Error("the newest pair is not tracked")
	}
}

// Many clients at once against a small table never exceed the cap (run with -race).
func TestLoginAttemptTracker_ParallelClientsStayWithinTheCap(t *testing.T) {
	tracker, _ := newTestLoginTracker()
	tracker.maxEntries = 64

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				tracker.attempt(fmt.Sprintf("user-%d", g%3), Client(fmt.Sprintf("client-%d-%d", g, i)))
			}
		}(g)
	}
	wg.Wait()

	if n := len(tracker.entries); n > tracker.maxEntries {
		t.Fatalf("table holds %d entries, cap is %d", n, tracker.maxEntries)
	}
}
