package wikibackup

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	backupSvc "github.com/perber/wiki/internal/backup"
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

	// Admin-only backup endpoints
	adminGroup := authGroup.Group("/admin")
	adminGroup.Use(authmw.RequireAdmin(opts.AuthDisabled))

	adminGroup.GET("/backup/status", r.handleGetBackupStatus)
	adminGroup.POST("/backup/push", r.handleTriggerBackup)
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

// handleTriggerBackup triggers an immediate backup and returns 202 Accepted.
func (r *Routes) handleTriggerBackup(c *gin.Context) {
	if r.scheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backup not enabled"})
		return
	}
	r.scheduler.TriggerNow()
	c.JSON(http.StatusAccepted, gin.H{"triggered": true})
}