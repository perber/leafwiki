package mcp

import (
	"errors"
	"net/http"
	"sync"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// A session ID identifies a client's transport state (client info, request
// IDs, event channels), not a credential — but with one global session
// manager (what mcp-go's WithStateful(true) uses) any authenticated user
// could reference, or DELETE, another user's live session. The
// userSessionRegistry fixes that (ADR-0019): one mcp-go stateful session
// manager per user, so a session ID is valid only for the authenticated user
// who created it; anyone else presenting it gets the same 404 the transport
// answers for an unknown ID — for all three transport verbs. mcp-go
// validates the ID against the resolved manager in its POST and DELETE
// handlers, but its GET (SSE stream) handler skips the check (the spec does
// not require session validation for GET), so the gin wrapper (routes.go)
// enforces the binding there through sessionIDOwnedBy.
//
// The registry is shared across concurrent requests: the per-user map is a
// sync.Map, one (tiny) manager per authenticated user, created lazily on
// first use.
type userSessionRegistry struct {
	managers sync.Map // user ID -> *mcpserver.InsecureStatefulSessionIdManager
}

// ResolveSessionIdManager implements mcp-go's SessionIdManagerResolver: it
// returns the session manager for the user the gin wrapper (routes.go)
// lifted into the request context, created on first use.
//
// The interface contract also covers a nil request: mcp-go's idle-TTL
// sweeper resolves the manager outside any request (sweepExpiredSessions)
// and calls Terminate on it. The sweeper must still be able to reap idle
// sessions of every user, so a nil request gets a sweepSessionManager whose
// Terminate fans out to all per-user managers (a manager is a no-op for an
// ID it does not own).
func (r *userSessionRegistry) ResolveSessionIdManager(req *http.Request) mcpserver.SessionIdManager {
	if req == nil {
		return &sweepSessionManager{registry: r}
	}
	if user := userFromContext(req.Context()); user != nil {
		return r.managerFor(user.ID)
	}
	// A request without a user never reaches the transport through the real
	// endpoint (APIAuthGroup rejects it first). Fail closed: a throwaway
	// manager whose sessions are valid nowhere, so a stray session can never
	// land in a real user's space.
	return &mcpserver.InsecureStatefulSessionIdManager{}
}

// managerFor returns the stateful session manager for the user ID, creating
// it on first use.
func (r *userSessionRegistry) managerFor(userID string) *mcpserver.InsecureStatefulSessionIdManager {
	actual, _ := r.managers.LoadOrStore(userID, &mcpserver.InsecureStatefulSessionIdManager{})
	return actual.(*mcpserver.InsecureStatefulSessionIdManager)
}

// sessionIDOwnedBy reports whether the session ID the request presents (the
// Mcp-Session-Id header) is live for the caller the gin wrapper lifted into
// the request context — the owner binding for the GET (SSE) verb, whose
// mcp-go handler does not check the ID. A request without a session ID is
// owned (mcp-go mints a fresh session for it); a request without a caller is
// not (fail closed — APIAuthGroup never lets one through the endpoint
// anyway). terminated distinguishes the transport's two 404 bodies: an ID
// the owner terminated (DELETE or the sweeper) answers "Session terminated",
// an ID the caller does not own answers "Invalid session ID".
func (r *userSessionRegistry) sessionIDOwnedBy(req *http.Request) (owned, terminated bool) {
	sessionID := req.Header.Get(mcpserver.HeaderKeySessionID)
	if sessionID == "" {
		return true, false
	}
	user := userFromContext(req.Context())
	if user == nil {
		return false, false
	}
	terminated, err := r.managerFor(user.ID).Validate(sessionID)
	if err != nil {
		return false, false
	}
	return true, terminated
}

// sweepSessionManager is the manager the resolver hands to mcp-go's idle-TTL
// sweeper (the nil request): its Terminate marks the given ID terminated in
// whichever per-user manager owns it, so idle sessions are reaped across all
// users — the sweep mcp-go's sweeper would run on a single global manager.
// The sweeper never calls Generate or Validate.
type sweepSessionManager struct {
	registry *userSessionRegistry
}

var errSweepManagerCannotValidate = errors.New("sweep session manager cannot validate session IDs")

func (s *sweepSessionManager) Generate() string { return "" }

func (s *sweepSessionManager) Validate(string) (bool, error) {
	return false, errSweepManagerCannotValidate
}

func (s *sweepSessionManager) Terminate(sessionID string) (bool, error) {
	s.registry.managers.Range(func(_, value any) bool {
		// The stateful manager's Terminate cannot fail (it is a no-op for an
		// ID it does not own), so the fan-out discards its result.
		_, _ = value.(*mcpserver.InsecureStatefulSessionIdManager).Terminate(sessionID)
		return true
	})
	return false, nil
}
