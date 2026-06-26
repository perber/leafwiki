package wikiresync

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the filesystem resync admin endpoint.
type Routes struct {
	trigger     func() error
	authService *coreauth.AuthService
}

// NewRoutes constructs the resync RouteRegistrar.
func NewRoutes(trigger func() error, authService *coreauth.AuthService) *Routes {
	return &Routes{
		trigger:     trigger,
		authService: authService,
	}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	authGroup := ctx.Base.Group("/api")
	authGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)

	adminGroup := authGroup.Group("/admin")
	adminGroup.Use(authmw.RequireAdmin(opts.AuthDisabled))

	adminGroup.POST("/resync", r.handleTriggerResync)
}

// handleTriggerResync blocks until the filesystem reload completes, then returns 200.
func (r *Routes) handleTriggerResync(c *gin.Context) {
	if err := r.trigger(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
