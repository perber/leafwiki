package auth

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestEmailTokenStore(t *testing.T) *EmailTokenStore {
	t.Helper()
	store, err := NewEmailTokenStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewEmailTokenStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	})
	return store
}

func TestEmailTokenStore_IssueThenResolve_ValidToken_Succeeds(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	tok, err := store.Resolve(raw, PurposePasswordReset)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if tok.UserID != "user-1" {
		t.Errorf("expected UserID user-1, got %s", tok.UserID)
	}
	if tok.Purpose != PurposePasswordReset {
		t.Errorf("expected purpose %s, got %s", PurposePasswordReset, tok.Purpose)
	}
	if tok.ConsumedAt != nil {
		t.Errorf("expected ConsumedAt nil for a fresh token")
	}
}

func TestEmailTokenStore_Resolve_WrongPurpose_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposeInvite, InviteTokenTTL)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, err := store.Resolve(raw, PurposePasswordReset); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for a purpose mismatch, got %v", err)
	}
}

func TestEmailTokenStore_Resolve_UnknownID_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	if _, err := store.Resolve("does-not-exist.somesecret", PurposePasswordReset); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for an unknown id, got %v", err)
	}
}

func TestEmailTokenStore_Resolve_MalformedToken_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	for _, raw := range []string{"", "no-dot-here", ".missing-id", "missing-secret."} {
		if _, err := store.Resolve(raw, PurposePasswordReset); err != ErrEmailTokenInvalid {
			t.Errorf("token %q: expected ErrEmailTokenInvalid, got %v", raw, err)
		}
	}
}

func TestEmailTokenStore_Resolve_WrongSecret_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	id := strings.SplitN(raw, ".", 2)[0]

	if _, err := store.Resolve(id+".wrong-secret", PurposePasswordReset); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for a wrong secret, got %v", err)
	}
}

func TestEmailTokenStore_Resolve_ExpiredToken_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, -time.Second)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	if _, err := store.Resolve(raw, PurposePasswordReset); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for an already-expired token, got %v", err)
	}
}

func TestEmailTokenStore_Consume_SingleUse_SecondConsumeFails(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	id := strings.SplitN(raw, ".", 2)[0]

	if err := store.Consume(id); err != nil {
		t.Fatalf("first Consume returned error: %v", err)
	}
	if err := store.Consume(id); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid on second Consume, got %v", err)
	}
}

func TestEmailTokenStore_Resolve_ConsumedToken_ReturnsInvalid(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	id := strings.SplitN(raw, ".", 2)[0]

	if err := store.Consume(id); err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}

	if _, err := store.Resolve(raw, PurposePasswordReset); err != ErrEmailTokenInvalid {
		t.Fatalf("expected ErrEmailTokenInvalid for an already-consumed token, got %v", err)
	}
}

// TestEmailTokenStore_Consume_ConcurrentRace_OnlyOneWinner guards the
// compare-and-swap semantics under real concurrency, not just sequential
// calls: two requests racing to consume the same token (e.g. a double-submit
// of the same reset link) must not both succeed.
func TestEmailTokenStore_Consume_ConcurrentRace_OnlyOneWinner(t *testing.T) {
	store := newTestEmailTokenStore(t)

	raw, err := store.Issue("user-1", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}
	id := strings.SplitN(raw, ".", 2)[0]

	const attempts = 10
	var wg sync.WaitGroup
	successes := make([]bool, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			successes[i] = store.Consume(id) == nil
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, ok := range successes {
		if ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly 1 successful Consume out of %d concurrent attempts, got %d", attempts, wins)
	}
}

func TestEmailTokenStore_CleanupExpired_RemovesOnlyExpiredRows(t *testing.T) {
	store := newTestEmailTokenStore(t)

	expiredRaw, err := store.Issue("user-1", PurposePasswordReset, -time.Second)
	if err != nil {
		t.Fatalf("Issue (expired) returned error: %v", err)
	}
	activeRaw, err := store.Issue("user-2", PurposePasswordReset, time.Hour)
	if err != nil {
		t.Fatalf("Issue (active) returned error: %v", err)
	}

	if err := store.CleanupExpired(); err != nil {
		t.Fatalf("CleanupExpired returned error: %v", err)
	}

	expiredID := strings.SplitN(expiredRaw, ".", 2)[0]
	if _, err := store.getByID(expiredID); err == nil {
		t.Fatalf("expected expired token row to be deleted")
	}

	if _, err := store.Resolve(activeRaw, PurposePasswordReset); err != nil {
		t.Fatalf("expected active token to survive cleanup, Resolve returned: %v", err)
	}
}
