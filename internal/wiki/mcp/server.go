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

// revisionIDArg trims and validates a revision-argument string (a revision ID
// or the list cursor), bounding it the same way the page ID is
// (maxPageIDArgLength) and returning the value and a client-facing message (""
// when the argument is usable). An optional argument (the cursor) may be
// empty.
func revisionIDArg(name, raw string, required bool) (string, string) {
	id := strings.TrimSpace(raw)
	switch {
	case id == "" && required:
		return "", name + " is required"
	case len(id) > maxPageIDArgLength:
		return "", fmt.Sprintf("%s is too long (maximum %d characters)", name, maxPageIDArgLength)
	}
	return id, ""
}

// pageWindowArgs binds the read-paging arguments the page-read tools share:
// offset is the zero-based index of the first line to read (default 0) and
// lines the number of lines to return (default 0, like a negative value, all
// remaining). A negative offset is answered with a client-facing message; an
// offset past the end of the page is checked against the fetched page by the
// caller (renderPageWindow), because only there is the page's line count
// known.
func pageWindowArgs(req mcpgo.CallToolRequest) (offset, lines int, message string) {
	offset = req.GetInt("offset", 0)
	if offset < 0 {
		return 0, 0, "offset must not be negative"
	}
	return offset, req.GetInt("lines", 0), ""
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
// streamable-HTTP transport as a plain http.Handler (ADR-0019). The page tools
// register here, each capturing the PageOps the composition root hands in.
// WithToolCapabilities keeps the tools capability advertised from the first
// PR — without it mcp-go answers tools/list with "tools not supported" until
// the first tool lands, which would make the endpoint's shape change mid-way
// through the PR series.
func newStreamableHandler(name, version string, log *slog.Logger, ops PageOps, sessions *userSessionRegistry) http.Handler {
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
		newEnsurePathTool(log, ops),
		newConvertNodeTool(log, ops),
		newCopyPageTool(log, ops),
		newGetPageRevisionsTool(log, ops),
		newGetPageRevisionTool(log, ops),
		newComparePageRevisionsTool(log, ops),
		newRestorePageRevisionTool(log, ops),
	)
	return mcpserver.NewStreamableHTTPServer(server,
		mcpserver.WithStreamableHTTPLogger(log),
		mcpserver.WithHTTPContextFunc(httpContextFromRequest),
		// Session IDs are bound to their owner (ADR-0019): one stateful
		// session manager per user, resolved from the caller the gin
		// wrapper lifted into the request context, so a session ID from
		// another user gets the same 404 as an unknown ID instead of
		// letting that user read, call or DELETE the live session. mcp-go's
		// default manager likewise issues IDs that never validate; the
		// per-user stateful managers keep the transport's POST and DELETE
		// verbs validating the session ID (its GET/SSE verb skips the
		// check — the spec does not require it for GET — so the gin
		// wrapper enforces the binding there). The resolver's
		// nil-request arm keeps the idle-TTL sweeper reaping sessions of
		// every user.
		mcpserver.WithSessionIdManagerResolver(sessions),
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
