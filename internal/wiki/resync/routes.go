package wikiresync

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the filesystem resync admin endpoints.
type Routes struct {
	triggerUC   *TriggerResyncUseCase
	statusUC    *GetResyncStatusUseCase
	authService *coreauth.AuthService
}

// NewRoutes constructs the resync RouteRegistrar.
func NewRoutes(triggerUC *TriggerResyncUseCase, statusUC *GetResyncStatusUseCase, authService *coreauth.AuthService) *Routes {
	return &Routes{
		triggerUC:   triggerUC,
		statusUC:    statusUC,
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
	adminGroup.GET("/resync/status", r.handleResyncStatus)
}

// handleTriggerResync starts a background resync and returns 202 immediately.
// Returns 409 if a resync is already running.
func (r *Routes) handleTriggerResync(c *gin.Context) {
	if err := r.triggerUC.Execute(c.Request.Context()); err != nil {
		respondWithResyncError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

// handleResyncStatus returns the current resync job state for client polling.
func (r *Routes) handleResyncStatus(c *gin.Context) {
	out := r.statusUC.Execute(c.Request.Context())
	c.JSON(http.StatusOK, out.Status)
}
