// Package instancesettings exposes admin-only instance settings that can be
// changed at runtime instead of only via flags/env + a restart. Today that is
// just the public-access ("public mode") toggle; it is the natural home for
// future runtime-reconfigurable RouterOptions.
package instancesettings

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/publicaccess"
)

// Routes is the RouteRegistrar for runtime instance settings.
type Routes struct {
	publicAccess *publicaccess.Service
	authService  *coreauth.AuthService
	log          *slog.Logger
}

// NewRoutes constructs the instance-settings RouteRegistrar.
func NewRoutes(publicAccess *publicaccess.Service, authService *coreauth.AuthService, log *slog.Logger) *Routes {
	if log == nil {
		log = slog.Default()
	}
	return &Routes{publicAccess: publicAccess, authService: authService, log: log}
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts

	adminGroup := ctx.Base.Group("/api/admin/settings")
	adminGroup.Use(
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuth(r.authService, ctx.AuthCookies, opts.AuthDisabled),
		security.CSRFMiddleware(ctx.CSRFCookie),
	)
	adminGroup.PUT("/public-access", authmw.RequireAdmin(opts.AuthDisabled), r.handleSetPublicAccess)
}

// ─── Handlers ───────────────────────────────────────────────────────────────

func (r *Routes) handleSetPublicAccess(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		respondWithStatusError(c, http.StatusBadRequest, ErrCodeInvalidPayload, "Invalid payload", "invalid payload")
		return
	}

	if err := r.publicAccess.SetEnabled(*req.Enabled); err != nil {
		// Env-managed instances land here with publicaccess.ErrCodeEnvManaged
		// → 409, so the frontend knows to render its status-only view.
		respondWithError(c, err)
		return
	}

	r.log.Info("public access mode changed",
		"enabled", r.publicAccess.Enabled(),
		"actor_id", actorID(c),
	)
	c.JSON(http.StatusOK, gin.H{"enabled": r.publicAccess.Enabled()})
}

func actorID(c *gin.Context) string {
	if u := authmw.TryGetUser(c); u != nil {
		return u.ID
	}
	return ""
}
