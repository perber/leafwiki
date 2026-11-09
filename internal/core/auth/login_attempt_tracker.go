package auth

import (
	"sync"
	"time"
)

// Password step: a decaying backoff per (account, client).
//
// Locking an account for everyone after a few wrong passwords lets anyone who
// knows an account name keep its owner out, so the password step slows each
// client down on its own instead. The wait grows with every counted attempt,
// and the whole record is forgotten after a quiet hour.
const (
	// loginFreeAttempts is the number of attempts per (account, client) that
	// carry no wait.
	loginFreeAttempts = 5

	// loginBackoffBase is the wait after the last free attempt. It doubles with
	// every further counted attempt.
	loginBackoffBase = 5 * time.Second

	// loginBackoffCap is the longest wait.
	loginBackoffCap = 15 * time.Minute

	// loginBackoffDecay is the quiet time after the last counted attempt that
	// makes the tracker forget an entry. It must not be shorter than
	// loginBackoffCap, or an entry could be forgotten while it is still waiting.
	loginBackoffDecay = time.Hour

	// loginTrackerMaxEntries bounds the table. The client part of the key comes
	// from the network, so the table cannot be left to grow with it.
	loginTrackerMaxEntries = 10000
)

// loginKey identifies one throttled pair. A struct, not a joined string, so
// two different pairs can never collide.
type loginKey struct {
	userID string
	client Client
}

func (k loginKey) less(o loginKey) bool {
	if k.userID != o.userID {
		return k.userID < o.userID
	}
	return k.client < o.client
}

type loginAttemptEntry struct {
	failures    int
	lastAttempt time.Time
	nextAllowed time.Time
}

// expired reports whether the entry has been quiet long enough to be treated
// as absent.
func (e *loginAttemptEntry) expired(now time.Time) bool {
	return e.lastAttempt.Add(loginBackoffDecay).Before(now)
}

// loginAttemptTracker throttles the password step. It lives in memory: a
// restart clears it, and nobody can cause a restart by failing to log in.
type loginAttemptTracker struct {
	mu         sync.Mutex
	entries    map[loginKey]*loginAttemptEntry
	now        func() time.Time // replaced in tests
	maxEntries int
}

func newLoginAttemptTracker() *loginAttemptTracker {
	return &loginAttemptTracker{
		entries:    make(map[loginKey]*loginAttemptEntry),
		now:        time.Now,
		maxEntries: loginTrackerMaxEntries,
	}
}

// attempt reports whether a password attempt for (userID, client) may be
// verified now, and counts it if so. The check and the count are one critical
// section, so parallel guesses cannot all slip through before the first one is
// recorded. A refused call is not counted and does not extend the wait: a wait
// that grew with every refusal would let anyone who can reach the login form
// keep the real owner of the same client waiting for ever.
func (t *loginAttemptTracker) attempt(userID string, client Client) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	key := loginKey{userID: userID, client: client}
	e, ok := t.entries[key]
	if ok && e.expired(now) {
		ok = false
	}
	if !ok {
		delete(t.entries, key)
		t.makeRoom(now)
		e = &loginAttemptEntry{}
		t.entries[key] = e
	}

	if now.Before(e.nextAllowed) {
		return false
	}

	e.failures++
	e.lastAttempt = now
	if e.failures >= loginFreeAttempts {
		e.nextAllowed = now.Add(loginBackoffFor(e.failures))
	}
	return true
}

// loginBackoffFor is the wait after the failures-th counted attempt.
func loginBackoffFor(failures int) time.Duration {
	wait := loginBackoffBase
	for i := loginFreeAttempts; i < failures && wait < loginBackoffCap; i++ {
		wait *= 2
	}
	return min(wait, loginBackoffCap)
}

// makeRoom frees one slot when the table is full: expired entries first, and if
// there are none the entry that will be allowed again soonest, which is the
// cheapest to lose. Ties are broken by the earliest last attempt, then by key,
// so the choice does not depend on map order. It never refuses to track and
// never empties the table, so a full table cannot switch the throttle off.
func (t *loginAttemptTracker) makeRoom(now time.Time) {
	if len(t.entries) < t.maxEntries {
		return
	}

	var victim loginKey
	var victimEntry *loginAttemptEntry
	for k, e := range t.entries {
		if e.expired(now) {
			delete(t.entries, k)
			continue
		}
		if victimEntry == nil || evictsBefore(k, e, victim, victimEntry) {
			victim, victimEntry = k, e
		}
	}
	if len(t.entries) >= t.maxEntries && victimEntry != nil {
		delete(t.entries, victim)
	}
}

// evictsBefore orders entries for makeRoom: true if a (key ka, entry a) is a
// better one to drop than b.
func evictsBefore(ka loginKey, a *loginAttemptEntry, kb loginKey, b *loginAttemptEntry) bool {
	if !a.nextAllowed.Equal(b.nextAllowed) {
		return a.nextAllowed.Before(b.nextAllowed)
	}
	if !a.lastAttempt.Equal(b.lastAttempt) {
		return a.lastAttempt.Before(b.lastAttempt)
	}
	return ka.less(kb)
}

// reset forgets one (account, client) pair, after that client has logged in
// with the right password. Other clients keep their own entries.
func (t *loginAttemptTracker) reset(userID string, client Client) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, loginKey{userID: userID, client: client})
}

// TOTP step: a short lock per account.
//
// Reaching the code step takes the account's password, so locking the account
// there does not let a stranger keep the owner out. With a 6-digit code that
// accepts a few values per attempt, a per-client budget would multiply the
// guesses by the number of addresses a guesser holds, so the lock covers
// every client together.
const (
	// totpMaxAttempts is the number of counted code attempts per account before
	// the lock starts.
	totpMaxAttempts = 5

	// totpLockDuration is how long the account is refused after the last
	// allowed attempt.
	totpLockDuration = 15 * time.Minute
)

type totpAttemptEntry struct {
	failures    int
	lockedUntil time.Time
}

// totpAttemptTracker counts code attempts per account, independently of the
// password step: wrong passwords never use up code attempts and wrong codes
// never use up password attempts.
type totpAttemptTracker struct {
	mu      sync.Mutex
	entries map[string]*totpAttemptEntry
	now     func() time.Time // replaced in tests
}

func newTOTPAttemptTracker() *totpAttemptTracker {
	return &totpAttemptTracker{
		entries: make(map[string]*totpAttemptEntry),
		now:     time.Now,
	}
}

// recordAttempt atomically checks whether the account is locked and, if not,
// increments the attempt counter. Returns false if the account is currently
// locked (caller must reject the attempt), true if the attempt may proceed.
// On the Nth attempt the lock is set inside the same critical section, so there
// is no window between the check and the increment.
func (t *totpAttemptTracker) recordAttempt(userID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	e, ok := t.entries[userID]
	if !ok {
		e = &totpAttemptEntry{}
		t.entries[userID] = e
	}

	if now.Before(e.lockedUntil) {
		return false
	}

	// Previous lock expired — start a fresh window.
	if !e.lockedUntil.IsZero() {
		e.failures = 0
		e.lockedUntil = time.Time{}
	}

	e.failures++
	if e.failures >= totpMaxAttempts {
		e.lockedUntil = now.Add(totpLockDuration)
		e.failures = 0
	}

	return true
}

func (t *totpAttemptTracker) reset(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, userID)
}
