package auth

import (
	"sync"
	"time"
)

const (
	loginMaxFailures  = 5
	loginLockDuration = 15 * time.Minute
)

type loginAttemptEntry struct {
	failures    int
	lockedUntil time.Time
}

type loginAttemptTracker struct {
	mu      sync.Mutex
	entries map[string]*loginAttemptEntry
}

func newLoginAttemptTracker() *loginAttemptTracker {
	return &loginAttemptTracker{
		entries: make(map[string]*loginAttemptEntry),
	}
}

// recordAttempt atomically checks whether the account is locked and, if not,
// increments the failure counter. Returns false if the account is currently
// locked (caller must reject the attempt), true if the attempt may proceed.
// On the Nth failure the lock is set inside the same critical section, so there
// is no window between the check and the increment.
func (t *loginAttemptTracker) recordAttempt(userID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	e, ok := t.entries[userID]
	if !ok {
		e = &loginAttemptEntry{}
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
	if e.failures >= loginMaxFailures {
		e.lockedUntil = now.Add(loginLockDuration)
		e.failures = 0
	}

	return true
}

func (t *loginAttemptTracker) reset(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, userID)
}
