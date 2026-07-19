package wikirestore

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/restore"
)

// Routes is the RouteRegistrar for the live restore admin endpoints.
type Routes struct {
	manager     *restore.Manager
	authService *coreauth.AuthService
}

// NewRoutes constructs the restore RouteRegistrar.
func NewRoutes(manager *restore.Manager, authService *coreauth.AuthService) *Routes {
	return &Routes{
		manager:     manager,
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

	adminGroup.POST("/restore/:id", r.handleTrigger)
	adminGroup.GET("/restore/status", r.handleStatus)
	adminGroup.POST("/restore/self-restart", r.handleSelfRestart)
}

// respondNotEnabled writes the standard 503 response for handlers that need
// a non-nil manager, which is only nil if snapshots (and therefore restore)
// are disabled.
func (r *Routes) respondNotEnabled(c *gin.Context) {
	respondWithRestoreStatusError(c, http.StatusServiceUnavailable, ErrCodeRestoreNotEnabled, "Restore is not enabled", "restore not enabled")
}

// handleTrigger starts a restore from snapshot :id and returns 202 Accepted.
func (r *Routes) handleTrigger(c *gin.Context) {
	if r.manager == nil {
		r.respondNotEnabled(c)
		return
	}
	id := c.Param("id")
	if err := r.manager.TriggerRestore(id); err != nil {
		respondWithRestoreError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

// handleStatus returns the current restore job state for client polling.
func (r *Routes) handleStatus(c *gin.Context) {
	if r.manager == nil {
		r.respondNotEnabled(c)
		return
	}
	c.JSON(http.StatusOK, r.manager.Status())
}

// handleSelfRestart re-execs the server process. Only permitted once the
// restore job has reported NeedsIntervention — this is the documented
// recovery path out of a stuck restore, not a general-purpose restart button.
// The HTTP response may never actually reach the client (the process image
// is replaced/exits before the handler can finish writing) — that's expected,
// the frontend treats a dropped connection here as success.
func (r *Routes) handleSelfRestart(c *gin.Context) {
	if r.manager == nil {
		r.respondNotEnabled(c)
		return
	}
	if !r.manager.Status().NeedsIntervention {
		respondWithRestoreStatusError(c, http.StatusConflict, ErrCodeRestoreNotIntervenable,
			"Self-restart is only available after a restore reports it needs intervention",
			"self-restart is only available after a restore reports it needs intervention")
		return
	}
	if err := r.manager.SelfRestart(); err != nil {
		respondWithRestoreStatusError(c, http.StatusInternalServerError, ErrCodeRestoreInternalError, "Failed to restart server", "failed to restart server")
		return
	}
}
