package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	coreauth "github.com/perber/wiki/internal/core/auth"
)

// requestWithCaller builds an httptest request with the given caller lifted
// into the request context the way the gin wrapper (routes.go) does; a nil
// user is an anonymous request.
func requestWithCaller(t *testing.T, user *coreauth.User) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/mcp", nil)
	if user == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), mcpUserKey, user))
}

// TestUserSessionRegistry_ScopesSessionsToTheirOwner is the owner-binding:
// a session ID generated for user A validates for A and is unknown to B's
// manager — the transport answers B's request with the same 404 it answers
// for any unknown ID (ADR-0018).
func TestUserSessionRegistry_ScopesSessionsToTheirOwner(t *testing.T) {
	registry := &userSessionRegistry{}
	userA := &coreauth.User{ID: "user-a"}
	userB := &coreauth.User{ID: "user-b"}

	mgrA := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	mgrB := registry.ResolveSessionIdManager(requestWithCaller(t, userB))

	sessionA := mgrA.Generate()
	if _, err := mgrA.Validate(sessionA); err != nil {
		t.Fatalf("the owner's own session must validate, got %v", err)
	}
	if _, err := mgrB.Validate(sessionA); err == nil {
		t.Fatal("another user's session ID must not validate for them")
	}
}

// TestUserSessionRegistry_StablePerUser pins that repeated requests for the
// same user resolve to the same manager, so a session survives across that
// user's requests.
func TestUserSessionRegistry_StablePerUser(t *testing.T) {
	registry := &userSessionRegistry{}
	userA := &coreauth.User{ID: "user-a"}

	first := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	second := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	if first != second {
		t.Fatal("the same user must resolve to the same session manager")
	}
}

// TestUserSessionRegistry_OtherUserCannotTerminate covers the DELETE verb:
// terminating an ID the user's manager does not own is a no-op (mcp-go's
// stateful manager answers a 200), so B cannot kill A's live session; the
// owner's own terminate works.
func TestUserSessionRegistry_OtherUserCannotTerminate(t *testing.T) {
	registry := &userSessionRegistry{}
	userA := &coreauth.User{ID: "user-a"}
	userB := &coreauth.User{ID: "user-b"}

	mgrA := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	mgrB := registry.ResolveSessionIdManager(requestWithCaller(t, userB))
	sessionA := mgrA.Generate()

	if notAllowed, err := mgrB.Terminate(sessionA); notAllowed || err != nil {
		t.Fatalf("B's terminate of A's session = (notAllowed=%v, err=%v), want (false, nil)", notAllowed, err)
	}
	if terminated, err := mgrA.Validate(sessionA); terminated || err != nil {
		t.Fatalf("A's session must survive B's terminate (terminated=%v, err=%v)", terminated, err)
	}

	if _, err := mgrA.Terminate(sessionA); err != nil {
		t.Fatalf("the owner's terminate must succeed, got %v", err)
	}
	if terminated, err := mgrA.Validate(sessionA); !terminated || err != nil {
		t.Fatalf("the owner's terminate must mark the session terminated (terminated=%v, err=%v)", terminated, err)
	}
}

// TestUserSessionRegistry_SweeperReap pins the nil-request arm of the
// resolver: mcp-go's idle-TTL sweeper resolves the manager with a nil request
// and calls Terminate on it, and the sweeper must still reap idle sessions of
// every user without touching live ones of other users.
func TestUserSessionRegistry_SweeperReap(t *testing.T) {
	registry := &userSessionRegistry{}
	userA := &coreauth.User{ID: "user-a"}
	userB := &coreauth.User{ID: "user-b"}

	mgrA := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	mgrB := registry.ResolveSessionIdManager(requestWithCaller(t, userB))
	sessionA := mgrA.Generate()
	sessionB := mgrB.Generate()

	sweeper := registry.ResolveSessionIdManager(nil)
	if _, err := sweeper.Terminate(sessionA); err != nil {
		t.Fatalf("the sweeper's terminate must succeed, got %v", err)
	}
	if terminated, err := mgrA.Validate(sessionA); !terminated || err != nil {
		t.Fatalf("the sweeper must terminate A's session (terminated=%v, err=%v)", terminated, err)
	}
	if _, err := mgrB.Validate(sessionB); err != nil {
		t.Fatalf("the sweeper must not touch other users' sessions, got %v", err)
	}
	// Reaping an ID nobody owns is a no-op, not an error.
	if _, err := sweeper.Terminate("mcp-session-00000000-0000-0000-0000-000000000000"); err != nil {
		t.Fatalf("reaping an unknown ID must be a no-op, got %v", err)
	}
}

// TestUserSessionRegistry_AnonymousRequestFailsClosed covers the request
// that carries no user: unreachable through the real endpoint (the auth
// chain sits in front), it must not touch any user's manager — its session
// IDs are valid nowhere.
func TestUserSessionRegistry_AnonymousRequestFailsClosed(t *testing.T) {
	registry := &userSessionRegistry{}
	userA := &coreauth.User{ID: "user-a"}

	mgrA := registry.ResolveSessionIdManager(requestWithCaller(t, userA))
	sessionA := mgrA.Generate()

	anonymous := registry.ResolveSessionIdManager(requestWithCaller(t, nil))
	anonymousNext := registry.ResolveSessionIdManager(requestWithCaller(t, nil))
	if _, err := anonymous.Validate(sessionA); err == nil {
		t.Fatal("an anonymous request must not validate a user's session ID")
	}
	// Each anonymous request gets its own throwaway manager, so a session it
	// creates is not carried into the next request either.
	if _, err := anonymousNext.Validate(anonymous.Generate()); err == nil {
		t.Fatal("an anonymous request's sessions must not be shared with the next request")
	}
}

// TestUserSessionRegistry_ConcurrentRequestsStayConsistent exercises the
// shared registry under concurrent requests (different users, and the same
// user racing itself through LoadOrStore): every request's own session must
// still validate for that request's user.
func TestUserSessionRegistry_ConcurrentRequestsStayConsistent(t *testing.T) {
	registry := &userSessionRegistry{}
	const users = 8
	const requestsPerUser = 50

	var wg sync.WaitGroup
	errs := make(chan error, users*requestsPerUser)
	for u := 0; u < users; u++ {
		user := &coreauth.User{ID: fmt.Sprintf("user-%d", u)}
		for r := 0; r < requestsPerUser; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mgr := registry.ResolveSessionIdManager(requestWithCaller(t, user))
				if _, err := mgr.Validate(mgr.Generate()); err != nil {
					errs <- fmt.Errorf("own session failed to validate: %w", err)
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
