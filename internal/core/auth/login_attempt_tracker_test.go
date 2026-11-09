package auth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a settable time source, so the backoff is tested without
// sleeping (timer granularity differs per platform).
type fakeClock struct {
	mu  sync.Mutex
	cur time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{cur: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cur = c.cur.Add(d)
}

func newTestLoginTracker() (*loginAttemptTracker, *fakeClock) {
	clock := newFakeClock()
	tracker := newLoginAttemptTracker()
	tracker.now = clock.Now
	return tracker, clock
}

// useFreeAttempts spends the attempts that carry no wait.
func useFreeAttempts(t *testing.T, tracker *loginAttemptTracker, userID string, client Client) {
	t.Helper()
	for i := 0; i < loginFreeAttempts; i++ {
		if !tracker.attempt(userID, client) {
			t.Fatalf("free attempt %d was refused", i+1)
		}
	}
}

func TestLoginAttemptTracker_FreeAttemptsThenWait(t *testing.T) {
	tracker, clock := newTestLoginTracker()

	useFreeAttempts(t, tracker, "u1", "a")

	if tracker.attempt("u1", "a") {
		t.Fatal("the attempt right after the free ones must be refused")
	}
	clock.Advance(loginBackoffBase - time.Nanosecond)
	if tracker.attempt("u1", "a") {
		t.Fatal("the attempt just before the wait is over must be refused")
	}
	clock.Advance(time.Nanosecond)
	if !tracker.attempt("u1", "a") {
		t.Fatal("the attempt once the wait is over must be allowed")
	}
}

func TestLoginAttemptTracker_WaitsDoubleUpToTheCap(t *testing.T) {
	tracker, clock := newTestLoginTracker()
	useFreeAttempts(t, tracker, "u1", "a")

	want := loginBackoffBase
	for step := 0; step < 12; step++ {
		clock.Advance(want - time.Nanosecond)
		if tracker.attempt("u1", "a") {
			t.Fatalf("step %d: allowed %v early", step, time.Nanosecond)
		}
		clock.Advance(time.Nanosecond)
		if !tracker.attempt("u1", "a") {
			t.Fatalf("step %d: refused after the wait of %v", step, want)
		}
		// The attempt just allowed has started the next, doubled wait.
		want = min(want*2, loginBackoffCap)
	}
	if want != loginBackoffCap {
		t.Fatalf("the wait never reached the cap, got %v", want)
	}

	// At the cap the wait stays there.
	clock.Advance(loginBackoffCap)
	if !tracker.attempt("u1", "a") {
		t.Fatal("attempt after a full cap must be allowed")
	}
	clock.Advance(loginBackoffCap - time.Nanosecond)
	if tracker.attempt("u1", "a") {
		t.Fatal("the wait must not grow beyond the cap")
	}
}

func TestLoginAttemptTracker_RefusedCallNeitherCountsNorExtendsTheWait(t *testing.T) {
	tracker, clock := newTestLoginTracker()
	useFreeAttempts(t, tracker, "u1", "a")

	// Hammer during the wait: none of it may push the allowed time out.
	for i := 0; i < 100; i++ {
		clock.Advance(loginBackoffBase / 200)
		if tracker.attempt("u1", "a") {
			t.Fatalf("call %d inside the wait was allowed", i)
		}
	}
	// The loop ended halfway through the wait. A wait extended from the last
	// refused call would still be running here, at exactly the original end.
	clock.Advance(loginBackoffBase / 2)
	if !tracker.attempt("u1", "a") {
		t.Fatal("refused calls extended the wait")
	}
	// Had the refused calls counted, this wait would be far beyond 2*base.
	clock.Advance(2 * loginBackoffBase)
	if !tracker.attempt("u1", "a") {
		t.Fatal("refused calls were counted as failures")
	}
}

func TestLoginAttemptTracker_EntryIsForgottenOnlyAfterTheDecay(t *testing.T) {
	tracker, clock := newTestLoginTracker()
	useFreeAttempts(t, tracker, "u1", "a")

	clock.Advance(loginBackoffDecay) // exactly the decay: still remembered
	if !tracker.attempt("u1", "a") {
		t.Fatal("the wait (shorter than the decay) is over, the attempt must be allowed")
	}
	// That was the sixth counted attempt; its wait is 10 s.
	if tracker.attempt("u1", "a") {
		t.Fatal("the entry was forgotten at exactly the decay")
	}

	clock.Advance(loginBackoffDecay + time.Nanosecond)
	for i := 0; i < loginFreeAttempts; i++ {
		if !tracker.attempt("u1", "a") {
			t.Fatalf("attempt %d after the decay was refused: the entry was not forgotten", i+1)
		}
	}
	if tracker.attempt("u1", "a") {
		t.Fatal("a forgotten entry starts a fresh count: its sixth attempt must wait again")
	}
}

func TestLoginAttemptTracker_DecayCoversTheCap(t *testing.T) {
	// An entry that is forgotten while it is still waiting would hand out free
	// attempts, so the decay must never be shorter than the longest wait.
	if loginBackoffDecay < loginBackoffCap {
		t.Fatalf("loginBackoffDecay (%v) must be at least loginBackoffCap (%v)", loginBackoffDecay, loginBackoffCap)
	}
}

func TestLoginAttemptTracker_ResetClearsOnlyItsOwnPair(t *testing.T) {
	pairs := []loginKey{{"u1", "a"}, {"u1", "b"}, {"u2", "a"}}
	for _, reset := range pairs {
		t.Run(fmt.Sprintf("reset %s from %s", reset.userID, reset.client), func(t *testing.T) {
			tracker, _ := newTestLoginTracker()
			for _, p := range pairs {
				useFreeAttempts(t, tracker, p.userID, p.client)
			}

			tracker.reset(reset.userID, reset.client)

			for _, p := range pairs {
				allowed := tracker.attempt(p.userID, p.client)
				if p == reset && !allowed {
					t.Errorf("%v: the reset pair is still waiting", p)
				}
				if p != reset && allowed {
					t.Errorf("%v: another pair's reset cleared this entry", p)
				}
			}
		})
	}
}

func TestLoginAttemptTracker_ClientsDoNotShareABudget(t *testing.T) {
	tracker, _ := newTestLoginTracker()

	useFreeAttempts(t, tracker, "u1", "a")
	if tracker.attempt("u1", "a") {
		t.Fatal("client a must be waiting")
	}
	for _, client := range []Client{"b", "c", ""} {
		if !tracker.attempt("u1", client) {
			t.Fatalf("client %q was held back by another client's failures", client)
		}
	}
}

func TestLoginAttemptTracker_KeyIsNotAConcatenation(t *testing.T) {
	tracker, _ := newTestLoginTracker()

	useFreeAttempts(t, tracker, "ab", "c")
	// "a"+"bc" and "ab"+"c" concatenate to the same string.
	if !tracker.attempt("a", "bc") {
		t.Fatal("distinct (user, client) pairs with the same concatenation share an entry")
	}
}

func TestLoginAttemptTracker_FullTableEvictsAndNeverRefusesToTrack(t *testing.T) {
	const maxEntries = 3

	tests := []struct {
		name string
		// setup fills the table with maxEntries entries.
		setup func(t *testing.T, tracker *loginAttemptTracker, clock *fakeClock)
		// evicted is the entry the next insert must drop.
		evicted loginKey
		// kept are entries that must survive it.
		kept []loginKey
	}{
		{
			name: "expired entries go first",
			setup: func(t *testing.T, tracker *loginAttemptTracker, clock *fakeClock) {
				// "old" has an allowed time, "fresh" has none: by allowed time
				// alone "fresh" would go, so only the expiry can pick "old".
				useFreeAttempts(t, tracker, "old", "a")
				clock.Advance(loginBackoffDecay + time.Second)
				useFreeAttempts(t, tracker, "waiting-late", "a")
				tracker.attempt("fresh", "a")
			},
			evicted: loginKey{"old", "a"},
			kept:    []loginKey{{"waiting-late", "a"}, {"fresh", "a"}},
		},
		{
			name: "earliest allowed time next",
			setup: func(t *testing.T, tracker *loginAttemptTracker, clock *fakeClock) {
				// Three entries in their wait; the one that frees up first goes.
				useFreeAttempts(t, tracker, "soon", "a")
				clock.Advance(time.Second)
				useFreeAttempts(t, tracker, "later", "a")
				clock.Advance(time.Second)
				useFreeAttempts(t, tracker, "latest", "a")
				// "soon" has the newest last attempt, so only its allowed time
				// can make it the one to go.
				tracker.entries[loginKey{"soon", "a"}].lastAttempt = clock.Now().Add(time.Second)
			},
			evicted: loginKey{"soon", "a"},
			kept:    []loginKey{{"later", "a"}, {"latest", "a"}},
		},
		{
			name: "tie on allowed time goes to the earliest last attempt",
			setup: func(t *testing.T, tracker *loginAttemptTracker, clock *fakeClock) {
				// Entries that never reached the free limit have no allowed time
				// (all equal); their last attempt differs.
				tracker.attempt("second", "a")
				clock.Advance(time.Second)
				tracker.attempt("first", "a")
				clock.Advance(time.Second)
				tracker.attempt("third", "a")
				// Make "first" the entry with the earliest last attempt.
				tracker.entries[loginKey{"first", "a"}].lastAttempt = clock.Now().Add(-time.Minute)
			},
			evicted: loginKey{"first", "a"},
			kept:    []loginKey{{"second", "a"}, {"third", "a"}},
		},
		{
			name: "full tie goes to the smallest key",
			setup: func(t *testing.T, tracker *loginAttemptTracker, clock *fakeClock) {
				// Same instant, no allowed time: only the key tells them apart.
				tracker.attempt("c", "x")
				tracker.attempt("a", "y")
				tracker.attempt("b", "x")
			},
			evicted: loginKey{"a", "y"},
			kept:    []loginKey{{"b", "x"}, {"c", "x"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker, clock := newTestLoginTracker()
			tracker.maxEntries = maxEntries
			tt.setup(t, tracker, clock)
			if len(tracker.entries) != maxEntries {
				t.Fatalf("setup left %d entries, want %d", len(tracker.entries), maxEntries)
			}

			// The new entry is tracked and counted, never refused.
			if !tracker.attempt("new", "a") {
				t.Fatal("a full table refused a first attempt")
			}

			if _, ok := tracker.entries[tt.evicted]; ok {
				t.Errorf("%v was not evicted", tt.evicted)
			}
			for _, k := range tt.kept {
				if _, ok := tracker.entries[k]; !ok {
					t.Errorf("%v was evicted", k)
				}
			}
			if _, ok := tracker.entries[loginKey{"new", "a"}]; !ok {
				t.Error("the new entry was not stored")
			}
			if len(tracker.entries) > maxEntries {
				t.Errorf("table holds %d entries, cap is %d", len(tracker.entries), maxEntries)
			}
		})
	}
}

// A full table must not switch the throttle off: pairs that merely have a few
// attempts on record are what makes room, a pair that is waiting stays.
func TestLoginAttemptTracker_ManyNewPairsDoNotEvictAWaitingPair(t *testing.T) {
	tracker, _ := newTestLoginTracker()
	tracker.maxEntries = 3

	useFreeAttempts(t, tracker, "u1", "a")
	for i := 0; i < 20; i++ {
		tracker.attempt(fmt.Sprintf("other-%d", i), "a")
	}

	if len(tracker.entries) > 3 {
		t.Fatalf("table holds %d entries, cap is 3", len(tracker.entries))
	}
	if tracker.attempt("u1", "a") {
		t.Fatal("a pair in its wait was forgotten because the table was full")
	}
}

func TestLoginAttemptTracker_ParallelAttemptsOnOnePairLetExactlyTheFreeOnesThrough(t *testing.T) {
	tracker, _ := newTestLoginTracker()

	const callers = 50
	var allowed atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if tracker.attempt("u1", "a") {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := allowed.Load(); got != loginFreeAttempts {
		t.Fatalf("%d of %d parallel attempts got through, want exactly %d", got, callers, loginFreeAttempts)
	}
}

func TestTOTPAttemptTracker_LocksAfterTheFifthAttemptForFifteenMinutes(t *testing.T) {
	clock := newFakeClock()
	tracker := newTOTPAttemptTracker()
	tracker.now = clock.Now

	for i := 0; i < totpMaxAttempts; i++ {
		if !tracker.recordAttempt("u1") {
			t.Fatalf("attempt %d was refused", i+1)
		}
	}
	if tracker.recordAttempt("u1") {
		t.Fatal("the attempt after the limit must be refused")
	}
	if !tracker.recordAttempt("u2") {
		t.Fatal("another user must not be locked")
	}

	clock.Advance(totpLockDuration - time.Nanosecond)
	if tracker.recordAttempt("u1") {
		t.Fatal("the lock ended early")
	}
	clock.Advance(time.Nanosecond)
	if !tracker.recordAttempt("u1") {
		t.Fatal("the lock did not end after its duration")
	}
}

func TestTOTPAttemptTracker_ResetClearsOnlyThatUser(t *testing.T) {
	tracker := newTOTPAttemptTracker()
	for _, id := range []string{"u1", "u2"} {
		for i := 0; i <= totpMaxAttempts; i++ {
			tracker.recordAttempt(id)
		}
	}

	tracker.reset("u1")

	if !tracker.recordAttempt("u1") {
		t.Error("reset user is still locked")
	}
	if tracker.recordAttempt("u2") {
		t.Error("reset of one user cleared another user's lock")
	}
}
