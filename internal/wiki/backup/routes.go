package wikibackup

import (
	"net/http"

	"github.com/gin-gonic/gin"
	backupSvc "github.com/perber/wiki/internal/backup"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the backup admin endpoints.
type Routes struct {
	repo        *backupSvc.Repository
	scheduler   *backupSvc.Scheduler
	authService *coreauth.AuthService
}

// NewRoutes constructs the backup RouteRegistrar.
func NewRoutes(repo *backupSvc.Repository, scheduler *backupSvc.Scheduler, authService *coreauth.AuthService) *Routes {
	return &Routes{
		repo:        repo,
		scheduler:   scheduler,
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

	// Lightweight alert endpoint — any authenticated user (editors + admins).
	// Exposes only needsIntervention bool; no sensitive config or credentials.
	// When authDisabled=true, InjectPublicEditor synthesises a user, so this
	// endpoint is effectively public — intentional, as it reveals no secrets.
	authGroup.GET("/backup/alert", r.handleGetBackupAlert)

	// Admin-only backup endpoints
	adminGroup := authGroup.Group("/admin")
	adminGroup.Use(authmw.RequireAdmin(opts.AuthDisabled))

	adminGroup.GET("/backup/status", r.handleGetBackupStatus)
	adminGroup.POST("/backup/push", r.handleTriggerBackup)
	adminGroup.POST("/backup/force-push", r.handleForcePush)
}

// handleGetBackupStatus returns the current backup status.
func (r *Routes) handleGetBackupStatus(c *gin.Context) {
	if r.scheduler == nil || r.repo == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled": true,
		"status":  r.repo.Status(),
	})
}

// handleGetBackupAlert returns a lightweight status for the header indicator —
// accessible to any authenticated user (editors + admins). Exposes no sensitive
// config or credentials. hasError covers both transient failures and
// NeedsIntervention so the indicator appears for any backup problem.
func (r *Routes) handleGetBackupAlert(c *gin.Context) {
	if r.scheduler == nil || r.repo == nil {
		c.JSON(http.StatusOK, gin.H{"needsIntervention": false, "hasError": false})
		return
	}
	s := r.repo.Status()
	c.JSON(http.StatusOK, gin.H{
		"needsIntervention": s.NeedsIntervention,
		"hasError":          s.LastError != "",
	})
}

// handleTriggerBackup triggers an immediate backup and returns 202 Accepted.
func (r *Routes) handleTriggerBackup(c *gin.Context) {
	if r.scheduler == nil {
		respondWithBackupStatusError(c, http.StatusServiceUnavailable, ErrCodeBackupNotEnabled, "Backup is not enabled", "backup not enabled")
		return
	}
	r.scheduler.TriggerNow()
	c.JSON(http.StatusAccepted, gin.H{"triggered": true})
}

// handleForcePush force-pushes the local backup history to the remote,
// overwriting any diverged remote state. Used to resolve NeedsIntervention.
func (r *Routes) handleForcePush(c *gin.Context) {
	if r.scheduler == nil || r.repo == nil {
		respondWithBackupStatusError(c, http.StatusServiceUnavailable, ErrCodeBackupNotEnabled, "Backup is not enabled", "backup not enabled")
		return
	}
	if err := r.repo.ForcePush(); err != nil {
		respondWithBackupStatusError(c, http.StatusInternalServerError, ErrCodeBackupInternalError, err.Error(), "backup internal error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
