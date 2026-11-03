package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
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

// actorFromContext builds the Actor the write tools hand to PageOps. Only
// call it after requireRole has passed: the role gate fails closed on a nil
// caller, so a user is always present here.
func actorFromContext(ctx context.Context) Actor {
	return Actor{ID: userFromContext(ctx).ID}
}

// pageIDArg trims the write tools' id argument and validates it (required,
// bounded by maxPageIDArgLength), returning the id and a client-facing
// message ("" when the argument is usable).
func pageIDArg(req mcpgo.CallToolRequest) (string, string) {
	id := strings.TrimSpace(req.GetString("id", ""))
	switch {
	case id == "":
		return "", "id is required"
	case len(id) > maxPageIDArgLength:
		return "", fmt.Sprintf("id is too long (maximum %d characters)", maxPageIDArgLength)
	}
	return id, ""
}

// requireRole enforces a tool's minimum role. It fails closed: a nil caller
// (no authenticated user lifted into the request context) is denied, not
// treated as the minimum role. It returns nil to proceed, or a ToolError
// (mcp_forbidden) when the caller must not run the tool.
func requireRole(ctx context.Context, minRole string) *ToolError {
	if coreauth.HasAtLeastRole(userFromContext(ctx), minRole) {
		return nil
	}
	return &ToolError{
		Code:    ErrCodeMCPForbidden,
		Message: "caller lacks the required role for this tool",
	}
}

// newStreamableHandler builds the MCP server and serves it on the
// streamable-HTTP transport as a plain http.Handler (ADR-0018). The page tools
// register here, each capturing the PageOps the composition root hands in.
// WithToolCapabilities keeps the tools capability advertised from the first
// PR — without it mcp-go answers tools/list with "tools not supported" until
// the first tool lands, which would make the endpoint's shape change mid-way
// through the PR series.
func newStreamableHandler(name, version string, log *slog.Logger, ops PageOps) http.Handler {
	server := mcpserver.NewMCPServer(name, version, mcpserver.WithToolCapabilities(false))
	server.AddTools(
		newGetPageTool(log, ops),
		newGetPageByPathTool(log, ops),
		newSearchTool(log, ops),
		newListTreeTool(log, ops),
		newCreatePageTool(log, ops),
		newUpdatePageTool(log, ops),
		newPatchPageTool(log, ops),
		newDeletePageTool(log, ops),
		newMovePageTool(log, ops),
	)
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
