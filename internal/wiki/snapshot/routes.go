package wikisnapshot

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
)

// Routes is the RouteRegistrar for the full-backup (snapshot) admin endpoints.
type Routes struct {
	manager        *snapshotSvc.Manager
	scheduler      *snapshotSvc.Scheduler
	authService    *coreauth.AuthService
	retentionCount int
}

// NewRoutes constructs the snapshot RouteRegistrar.
func NewRoutes(manager *snapshotSvc.Manager, scheduler *snapshotSvc.Scheduler, authService *coreauth.AuthService, retentionCount int) *Routes {
	return &Routes{
		manager:        manager,
		scheduler:      scheduler,
		authService:    authService,
		retentionCount: retentionCount,
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

	adminGroup.GET("/snapshot/status", r.handleStatus)
	adminGroup.GET("/snapshot", r.handleList)
	adminGroup.POST("/snapshot", r.handleTrigger)
	adminGroup.GET("/snapshot/:id/download", r.handleDownload)
	adminGroup.DELETE("/snapshot/:id", r.handleDelete)
}

// handleStatus returns whether the feature is enabled, the configured
// retention count, and the current run status.
func (r *Routes) handleStatus(c *gin.Context) {
	if r.manager == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":        true,
		"retentionCount": r.retentionCount,
		"status":         r.manager.Status(),
	})
}

// handleList returns all finished snapshots, newest first.
func (r *Routes) handleList(c *gin.Context) {
	entries, err := r.manager.List()
	if err != nil {
		respondWithSnapshotError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"snapshots": entries})
}

// handleTrigger triggers an immediate snapshot and returns 202 Accepted.
func (r *Routes) handleTrigger(c *gin.Context) {
	if r.scheduler == nil {
		respondWithSnapshotStatusError(c, http.StatusServiceUnavailable, ErrCodeSnapshotNotEnabled, "Snapshot backup is not enabled", "snapshot backup not enabled")
		return
	}
	if !r.scheduler.TriggerNow() {
		respondWithSnapshotError(c, snapshotSvc.ErrAlreadyRunning)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"triggered": true})
}

// handleDownload streams the ZIP file for a given snapshot id as an attachment.
func (r *Routes) handleDownload(c *gin.Context) {
	id := c.Param("id")
	zipPath, err := r.manager.SnapshotZipPath(id)
	if err != nil {
		respondWithSnapshotError(c, err)
		return
	}

	f, err := os.Open(zipPath)
	if err != nil {
		respondWithSnapshotStatusError(c, http.StatusInternalServerError, ErrCodeSnapshotInternalError, "Failed to open snapshot", "failed to open snapshot")
		return
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		respondWithSnapshotStatusError(c, http.StatusInternalServerError, ErrCodeSnapshotInternalError, "Failed to read snapshot", "failed to read snapshot")
		return
	}

	filename := filepath.Base(zipPath)
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if disposition == "" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", disposition)
	c.Writer.Header().Set("Content-Type", "application/zip")
	http.ServeContent(c.Writer, c.Request, filename, stat.ModTime(), f)
}

// handleDelete removes a snapshot's ZIP and sidecar metadata.
func (r *Routes) handleDelete(c *gin.Context) {
	id := c.Param("id")
	if err := r.manager.Delete(id); err != nil {
		respondWithSnapshotError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
