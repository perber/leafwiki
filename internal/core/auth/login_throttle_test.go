package auth

import (
	"errors"
	"fmt"
	"testing"
)

// freezeLoginClocks puts both throttles of the service on one fake clock, so a
// test decides when a wait is over and never races real time.
func freezeLoginClocks(a *AuthService) *fakeClock {
	clock := newFakeClock()
	a.attempts.now = clock.Now
	a.totpAttempts.now = clock.Now
	return clock
}

func mustBeInvalidCredentials(t *testing.T, a *AuthService, password string, client Client) {
	t.Helper()
	if _, err := a.Login("testuser", password, client); !errors.Is(err, ErrUserInvalidCredentials) {
		t.Fatalf("Login(%q) from %q: expected ErrUserInvalidCredentials, got %v", password, client, err)
	}
}

func TestAuthService_Login_ThrottlingOneClientDoesNotLockOutAnother(t *testing.T) {
	authService := setupTestAuthService(t)
	clock := freezeLoginClocks(authService)

	for i := 0; i < loginFreeAttempts; i++ {
		mustBeInvalidCredentials(t, authService, "wrong-guess", "198.51.100.1")
	}

	// The owner, from another client, is not held back by those failures.
	if _, err := authService.Login("testuser", "securepass", "203.0.113.9"); err != nil {
		t.Fatalf("the owner's login from another client failed: %v", err)
	}

	// The throttled client is refused even with the correct password, until
	// the wait is over.
	if _, err := authService.Login("testuser", "securepass", "198.51.100.1"); !errors.Is(err, ErrLoginThrottled) {
		t.Fatalf("expected ErrLoginThrottled during the wait, got %v", err)
	}
	clock.Advance(loginBackoffBase)
	if _, err := authService.Login("testuser", "securepass", "198.51.100.1"); err != nil {
		t.Fatalf("login after the wait failed: %v", err)
	}
}

func TestAuthService_Login_SuccessClearsOnlyItsOwnClient(t *testing.T) {
	authService := setupTestAuthService(t)
	freezeLoginClocks(authService)

	for _, client := range []Client{"198.51.100.1", "198.51.100.2"} {
		for i := 0; i < loginFreeAttempts; i++ {
			mustBeInvalidCredentials(t, authService, "wrong-guess", client)
		}
	}
	if _, err := authService.Login("testuser", "securepass", "203.0.113.9"); err != nil {
		t.Fatalf("login from an unrelated client failed: %v", err)
	}

	for _, client := range []Client{"198.51.100.1", "198.51.100.2"} {
		if _, err := authService.Login("testuser", "securepass", client); !errors.Is(err, ErrLoginThrottled) {
			t.Errorf("client %s: a success elsewhere cleared this client's wait, got %v", client, err)
		}
	}
}

func TestAuthService_Login_UnknownIdentifierIsNotTracked(t *testing.T) {
	authService := setupTestAuthService(t)
	freezeLoginClocks(authService)

	for i := 0; i < 3*loginFreeAttempts; i++ {
		if _, err := authService.Login("no-such-user", "whatever", testClient); !errors.Is(err, ErrUserInvalidCredentials) {
			t.Fatalf("attempt %d: expected ErrUserInvalidCredentials, got %v", i, err)
		}
	}

	if n := len(authService.attempts.entries); n != 0 {
		t.Fatalf("unknown identifiers left %d entries in the throttle table", n)
	}
}

func TestAuthService_Login_CorrectPasswordWithTOTPClearsThePasswordEntry(t *testing.T) {
	f := setupTOTPTestFixture(t)
	freezeLoginClocks(f.authService)

	// Four typos, then the correct password: the fifth counted attempt.
	for i := 0; i < loginFreeAttempts-1; i++ {
		mustBeInvalidCredentials(t, f.authService, "wrong-guess", testClient)
	}
	// Without a reset the next login would already be waiting.
	for i := 0; i < 2*loginFreeAttempts; i++ {
		challenge, err := f.authService.Login("testuser", "securepass", testClient)
		if err != nil {
			t.Fatalf("login %d of a TOTP account was refused: %v", i, err)
		}
		if !challenge.RequiresTOTP {
			t.Fatalf("login %d: expected a TOTP challenge", i)
		}
	}
}

func TestAuthService_TOTPLock(t *testing.T) {
	// wrongCodes sends n wrong codes against one challenge, all counted.
	wrongCodes := func(t *testing.T, f *totpTestFixture, challenge string, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			_, err := f.authService.CompleteTOTPLogin(challenge, "000000")
			if err == nil || errors.Is(err, ErrLoginThrottled) {
				t.Fatalf("wrong code %d: expected an invalid-code error, got %v", i+1, err)
			}
		}
	}
	login := func(t *testing.T, f *totpTestFixture, client Client) string {
		t.Helper()
		challenge, err := f.authService.Login("testuser", "securepass", client)
		if err != nil {
			t.Fatalf("Login from %q: %v", client, err)
		}
		return challenge.LoginChallengeToken
	}

	t.Run("five counted attempts lock the user for every client", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		freezeLoginClocks(f.authService)
		challenge := login(t, f, "198.51.100.1")

		wrongCodes(t, f, challenge, totpMaxAttempts)

		// A challenge obtained from another client is refused as well, even
		// with a valid code.
		other := login(t, f, "203.0.113.9")
		if _, err := f.authService.CompleteTOTPLogin(other, f.currentCode(t)); !errors.Is(err, ErrLoginThrottled) {
			t.Fatalf("expected ErrLoginThrottled for another client, got %v", err)
		}
	})

	t.Run("the lock ends after its duration", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		clock := freezeLoginClocks(f.authService)
		challenge := login(t, f, testClient)
		wrongCodes(t, f, challenge, totpMaxAttempts)

		clock.Advance(totpLockDuration)

		if _, err := f.authService.CompleteTOTPLogin(challenge, f.currentCode(t)); err != nil {
			t.Fatalf("a valid code after the lock failed: %v", err)
		}
	})

	t.Run("wrong passwords do not consume code attempts", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		freezeLoginClocks(f.authService)
		challenge := login(t, f, "203.0.113.9")

		for i := 0; i < 2*loginFreeAttempts; i++ {
			client := Client(fmt.Sprintf("198.51.100.%d", i+1))
			mustBeInvalidCredentials(t, f.authService, "wrong-guess", client)
		}

		if _, err := f.authService.CompleteTOTPLogin(challenge, f.currentCode(t)); err != nil {
			t.Fatalf("wrong passwords used up the code attempts: %v", err)
		}
	})

	t.Run("wrong codes do not consume password attempts", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		freezeLoginClocks(f.authService)
		challenge := login(t, f, testClient)
		wrongCodes(t, f, challenge, totpMaxAttempts)

		// The code step is locked, the password step of the same client is not.
		for i := 0; i < loginFreeAttempts; i++ {
			mustBeInvalidCredentials(t, f.authService, "wrong-guess", testClient)
		}
	})

	t.Run("a correct password never resets it", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		freezeLoginClocks(f.authService)
		challenge := login(t, f, testClient)
		wrongCodes(t, f, challenge, totpMaxAttempts-1)

		// Resubmitting the password must not give the code step a fresh budget.
		again := login(t, f, testClient)
		wrongCodes(t, f, again, 1)

		if _, err := f.authService.CompleteTOTPLogin(again, f.currentCode(t)); !errors.Is(err, ErrLoginThrottled) {
			t.Fatalf("expected ErrLoginThrottled after %d wrong codes, got %v", totpMaxAttempts, err)
		}
	})

	t.Run("only a full success resets it", func(t *testing.T) {
		f := setupTOTPTestFixture(t)
		freezeLoginClocks(f.authService)
		challenge := login(t, f, testClient)
		wrongCodes(t, f, challenge, totpMaxAttempts-1)
		if _, err := f.authService.CompleteTOTPLogin(challenge, f.currentCode(t)); err != nil {
			t.Fatalf("valid code: %v", err)
		}

		// A full budget again, with a new challenge.
		next := login(t, f, testClient)
		wrongCodes(t, f, next, totpMaxAttempts)
	})
}
