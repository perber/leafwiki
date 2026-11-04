package http

import (
	"net/http"
	pathpkg "path"
	"strings"
	"sync"

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

	capabilities *capabilityPrefixes
}

// capabilityRequestKey marks a request served by a capability group, so the
// request logger logs the route template instead of the URL.
const capabilityRequestKey = "leafwiki.capabilityRequest"

// capabilityPrefixes are the path prefixes of all capability groups. The
// request logger needs them for requests that match no route: those never run
// the group's middleware, but their URL may still carry a secret.
type capabilityPrefixes struct {
	mu       sync.RWMutex
	prefixes []string
}

func (p *capabilityPrefixes) add(prefix string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prefixes = append(p.prefixes, prefix)
}

// match returns the capability prefix path falls under, or "". It checks the
// path as sent and cleaned, so neither "/api//shared/x" nor
// "/api/shared/../x" slips past "/api/shared".
func (p *capabilityPrefixes) match(path string) string {
	if p == nil {
		return ""
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, candidate := range []string{path, pathpkg.Clean(path)} {
		for _, prefix := range p.prefixes {
			if candidate == prefix || strings.HasPrefix(candidate, prefix+"/") {
				return prefix
			}
		}
	}
	return ""
}

// CapabilityGroup returns a read-only "/api<path>" group for capability URLs:
// URLs whose secret grants access by itself, like share links (ADR-0018).
// Such a request never carries an identity, so the group is built from the
// engine and skips Base's middlewares: no session, no reverse-proxy identity
// (an unknown asserted user is neither rejected nor auto-provisioned), no API
// key. Responses of its routes get no-referrer, noindex and no-store, and the
// access log shows the route template instead of the URL. The group is meant
// for reads only: a write route registered in it answers 405, since it would
// bypass the restore write gate and CSRF. Methods without a registered route
// (HEAD included) get the router's normal 404.
func (ctx RouterContext) CapabilityGroup(path string) *gin.RouterGroup {
	prefix := ctx.Opts.BasePath + "/api" + path
	ctx.capabilities.add(prefix)
	g := ctx.Engine.Group(prefix)
	g.Use(capabilityGuard)
	return g
}

func capabilityGuard(c *gin.Context) {
	c.Set(capabilityRequestKey, true)
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return
	}
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex, nofollow")
	c.Header("Cache-Control", "no-store")
	c.Next()
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
