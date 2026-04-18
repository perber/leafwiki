package http

import (
	authmiddleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/gin-gonic/gin"
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

// RouteRegistrar is the interface each domain module implements to register its
// own routes, groups, and middleware on the engine.
type RouteRegistrar interface {
	RegisterRoutes(ctx RouterContext)
}
