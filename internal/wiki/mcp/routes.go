package mcp

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
)

// Routes is the RouteRegistrar for the experimental MCP (Model Context
// Protocol) server (ADR-0018). It serves the wiki at /api/mcp on the
// streamable-HTTP transport, behind the same API auth chain as the REST API:
// a session cookie or, when API-key management is enabled, a Bearer API key.
//
// The registered page tools (reads and writes; revisions/admin in follow-up
// PRs) are thin: they bind arguments, gate the caller's role, delegate to the
// PageOps the composition root supplies (WikiMCPAdapter), format the result,
// and map errors onto a ToolError. The adapter delegates to the existing
// pages/revisions/search use cases; the slice never talks to them directly.
type Routes struct {
	handler     http.Handler
	authService *coreauth.AuthService
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	Name        string // serverInfo.name reported in the initialize response
	Version     string // serverInfo.version
	Log         *slog.Logger
	AuthService *coreauth.AuthService
	// Ops is the PageOps the tool handlers delegate to. The composition root
	// provides the WikiMCPAdapter; it is always non-nil when the server is on.
	Ops PageOps
}

// NewRoutes constructs the mcp RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		handler:     newStreamableHandler(cfg.Name, cfg.Version, cfg.Log, cfg.Ops),
		authService: cfg.AuthService,
	}
}

// RegisterRoutes implements RouteRegistrar. One endpoint, the three verbs of
// the streamable-HTTP transport: POST carries the JSON-RPC messages, GET
// opens the SSE notification stream, DELETE terminates the session. Other
// methods are not routed at all. APIAuthGroup already carries the /api
// prefix, so the route lands at /api/mcp (plus the base path).
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	authGroup := ctx.APIAuthGroup(r.authService)
	authGroup.POST("/mcp", r.handleMCP)
	authGroup.GET("/mcp", r.handleMCP)
	authGroup.DELETE("/mcp", r.handleMCP)
}

// maxRequestBodyBytes caps the MCP POST body. mcp-go reads it with an
// unbounded io.ReadAll, and the write tools carry full page content, so the
// cap has to accept the largest page payload the platform accepts anywhere:
// the importer's ZIP upload cap (internal/wiki/importer's importMaxUploadSize)
// and the backup-restore upload default (--restore-upload-max-size, 500 MiB)
// — both 500 MiB, the bulk-content ceiling the platform already serves. The
// adjacent REST page-update route sets no cap at all, so this is strictly
// tighter; looser would only widen an authenticated-DoS lever (the body is
// fully buffered and JSON-decoded per request). See ADR-0018.
const maxRequestBodyBytes = 500 << 20

// handleMCP lifts the caller resolved by the auth chain into the request
// context and hands the request to the mcp-go streamable handler. The lift
// (gin context → request context) happens here; WithHTTPContextFunc
// (httpContextFromRequest) moves it from the request context into the MCP
// request context.
func (r *Routes) handleMCP(c *gin.Context) {
	// With the cap an oversized read fails and mcp-go answers a JSON-RPC
	// PARSE_ERROR, never a raw HTTP failure.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
	ctx := c.Request.Context()
	if user := authmw.TryGetUser(c); user != nil {
		ctx = context.WithValue(ctx, mcpUserKey, user)
	}
	if authmw.IsAPIKeyAuth(c) {
		ctx = context.WithValue(ctx, mcpKeyAuthKey, true)
	}
	r.handler.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
}
