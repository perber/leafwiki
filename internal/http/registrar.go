package http

import (
	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	authmiddleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// RouterContext holds the shared HTTP infrastructure passed to each RouteRegistrar.
// It gives every domain module everything it needs to register routes and apply
// its own middleware without coupling to the central router.
type RouterContext struct {
	// Engine is the root gin engine — use Base for route registration.
	Engine *gin.Engine
	// Base is the group with the configured BasePath already applied.
	Base gin.IRouter
	// AuthCookies manages reading/writing JWT access and refresh cookies.
	AuthCookies *authmiddleware.AuthCookies
	// CSRFCookie manages issuing and validating CSRF tokens.
	CSRFCookie *security.CSRFCookie
	// Opts contains the global router configuration.
	Opts RouterOptions
}

// APIAuthGroup returns an "/api" group that always requires an authenticated
// user: InjectPublicEditor (for --disable-auth), RequireAuth, then CSRF. This
// is the one place that middleware chain is defined; domain registrars call it
// instead of repeating the three .Use lines.
func (ctx RouterContext) APIAuthGroup(authService *coreauth.AuthService) *gin.RouterGroup {
	g := ctx.Base.Group("/api")
	g.Use(
		authmiddleware.InjectPublicEditor(ctx.Opts.AuthDisabled),
		authmiddleware.RequireAuth(authService, ctx.AuthCookies, ctx.Opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	return g
}

// APIReadGroup returns an "/api" group for read-only endpoints that may be
// exposed anonymously when "public mode" is on: InjectPublicEditor,
// RequireAuthOrPublicRead (attaches the user if present; requires one only
// when public mode is off), then CSRF (a no-op for the GET verbs these
// endpoints use). This is the single definition of what guards public-capable
// reads.
func (ctx RouterContext) APIReadGroup(authService *coreauth.AuthService) *gin.RouterGroup {
	g := ctx.Base.Group("/api")
	g.Use(
		authmiddleware.InjectPublicEditor(ctx.Opts.AuthDisabled),
		authmiddleware.RequireAuthOrPublicRead(authService, ctx.AuthCookies, ctx.Opts.AuthDisabled, ctx.Opts.PublicAccess),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	return g
}

// RouteRegistrar is the interface each domain module implements to register its
// own routes, groups, and middleware on the engine.
type RouteRegistrar interface {
	RegisterRoutes(ctx RouterContext)
}
