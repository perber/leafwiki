package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// The context keys the caller info travels on. The gin wrapper (routes.go)
// stores the values on the incoming request's context; httpContextFromRequest
// lifts them into the mcp-go request context the tools receive. Unexported so
// only this package writes and reads them.
type contextKey int

const (
	mcpUserKey contextKey = iota
	mcpKeyAuthKey
)

// userFromContext returns the caller lifted into the context by the gin
// wrapper, or nil when no user is present.
func userFromContext(ctx context.Context) *coreauth.User {
	user, _ := ctx.Value(mcpUserKey).(*coreauth.User)
	return user
}

// keyAuthFromContext reports whether the caller authenticated with a Bearer
// API key rather than a browser session. Tool handlers may use the marker
// to refuse key-authenticated callers on session-only tools, mirroring
// RequireCookieSession on the REST API.
func keyAuthFromContext(ctx context.Context) bool {
	keyAuth, _ := ctx.Value(mcpKeyAuthKey).(bool)
	return keyAuth
}

// newStreamableHandler builds the MCP server (zero tools in PR 1: a client
// gets a valid initialize and an empty tools/list) and serves it on the
// streamable-HTTP transport as a plain http.Handler (ADR-0018).
// WithToolCapabilities keeps the tools capability advertised with zero tools
// registered — without it mcp-go answers tools/list with "tools not
// supported" until the first tool lands, which would make the endpoint's
// shape change mid-way through the PR series.
func newStreamableHandler(name, version string, log *slog.Logger) http.Handler {
	server := mcpserver.NewMCPServer(name, version, mcpserver.WithToolCapabilities(false))
	return mcpserver.NewStreamableHTTPServer(server,
		mcpserver.WithStreamableHTTPLogger(log),
		mcpserver.WithHTTPContextFunc(httpContextFromRequest),
		// mcp-go's default session manager issues session IDs but never
		// validates them: a terminated or unknown session would keep
		// working. Stateful mode tracks sessions, so the transport's
		// DELETE verb really terminates a session and unknown IDs get a
		// 404. Its sticky-session requirement is moot for a single
		// binary like LeafWiki.
		mcpserver.WithStateful(true),
		// mcp-go keeps per-session state (client info, request IDs, event
		// channels) until an explicit DELETE; an idle TTL bounds the slow
		// leak from clients that disconnect without one.
		mcpserver.WithSessionIdleTTL(30*time.Minute),
	)
}

// httpContextFromRequest is the WithHTTPContextFunc: it moves the caller
// from the incoming request's context (where the gin wrapper put it) into
// the context mcp-go hands to the tools.
func httpContextFromRequest(ctx context.Context, r *http.Request) context.Context {
	if user := userFromContext(r.Context()); user != nil {
		ctx = context.WithValue(ctx, mcpUserKey, user)
	}
	if keyAuthFromContext(r.Context()) {
		ctx = context.WithValue(ctx, mcpKeyAuthKey, true)
	}
	return ctx
}
