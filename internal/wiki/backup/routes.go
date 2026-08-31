package wikibackup

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	backupSvc "github.com/perber/wiki/internal/backup"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for the backup admin endpoints. It talks to a
// backup.Manager rather than a Repository/Scheduler pair so that a runtime
// reconfigure (settings mode) swaps the underlying git machinery without
// re-registering any routes.
type Routes struct {
	mgr         *backupSvc.Manager
	authService *coreauth.AuthService
}

// NewRoutes constructs the backup RouteRegistrar. mgr may be nil (git backup
// entirely unavailable), in which case every endpoint reports "not enabled".
func NewRoutes(mgr *backupSvc.Manager, authService *coreauth.AuthService) *Routes {
	return &Routes{mgr: mgr, authService: authService}
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
	// Exposes only booleans; no sensitive config or credentials.
	authGroup.GET("/backup/alert", r.handleGetBackupAlert)

	adminGroup := authGroup.Group("/admin")
	adminGroup.Use(authmw.RequireAdmin(opts.AuthDisabled))

	adminGroup.GET("/backup/status", r.handleGetBackupStatus)
	adminGroup.POST("/backup/push", r.handleTriggerBackup)
	adminGroup.POST("/backup/force-push", r.handleForcePush)
	adminGroup.POST("/backup/pull", r.handleTriggerPull)

	// Settings-mode configuration. These 409 when the backup is env-managed.
	adminGroup.GET("/backup/config", r.handleGetBackupConfig)
	adminGroup.POST("/backup/config", r.handleSaveBackupConfig)
	adminGroup.POST("/backup/config/test", r.handleTestBackupConfig)
	adminGroup.POST("/backup/disable", r.handleDisableBackup)
}

func (r *Routes) handleGetBackupStatus(c *gin.Context) {
	if r.mgr == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "envManaged": false})
		return
	}
	body := gin.H{
		"enabled":    r.mgr.Enabled(),
		"envManaged": r.mgr.EnvManaged(),
	}
	if snap, running := r.mgr.Status(); running {
		body["status"] = snap
	}
	if err := r.mgr.BootError(); err != nil {
		body["bootError"] = err.Error()
	}
	c.JSON(http.StatusOK, body)
}

// handleGetBackupAlert returns a lightweight status for the header indicator.
// A configured-but-not-starting backup (BootError) also counts as an error so
// the indicator surfaces it.
func (r *Routes) handleGetBackupAlert(c *gin.Context) {
	if r.mgr == nil {
		c.JSON(http.StatusOK, gin.H{"needsIntervention": false, "hasError": false})
		return
	}
	snap, running := r.mgr.Status()
	hasError := r.mgr.BootError() != nil
	needsIntervention := false
	if running {
		needsIntervention = snap.NeedsIntervention
		hasError = hasError || snap.LastError != ""
	}
	c.JSON(http.StatusOK, gin.H{"needsIntervention": needsIntervention, "hasError": hasError})
}

func (r *Routes) handleTriggerBackup(c *gin.Context) {
	if r.mgr == nil {
		r.respondNotEnabled(c)
		return
	}
	if err := r.mgr.TriggerNow(); err != nil {
		r.respondNotEnabled(c)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"triggered": true})
}

func (r *Routes) handleForcePush(c *gin.Context) {
	if r.mgr == nil {
		r.respondNotEnabled(c)
		return
	}
	if err := r.mgr.ForcePush(); err != nil {
		if errors.Is(err, backupSvc.ErrNotRunning) {
			r.respondNotEnabled(c)
			return
		}
		respondWithBackupStatusError(c, http.StatusInternalServerError, ErrCodeBackupInternalError, err.Error(), "backup internal error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (r *Routes) handleTriggerPull(c *gin.Context) {
	if r.mgr == nil {
		r.respondNotEnabled(c)
		return
	}
	if err := r.mgr.Pull(); err != nil {
		if errors.Is(err, backupSvc.ErrNotRunning) {
			r.respondNotEnabled(c)
			return
		}
		// Empty template so mapApiError surfaces the real (often actionable,
		// e.g. conflict) message rather than a literal unregistered key.
		respondWithBackupStatusError(c, http.StatusInternalServerError, ErrCodeBackupInternalError, err.Error(), "")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (r *Routes) respondNotEnabled(c *gin.Context) {
	respondWithBackupStatusError(c, http.StatusServiceUnavailable, ErrCodeBackupNotEnabled, "Backup is not enabled", "backup not enabled")
}

// ─── Settings-mode configuration ────────────────────────────────────────────

// backupConfigRequest is the body of POST /backup/config and /backup/config/test.
// Empty sshKey / httpPassword mean "keep the stored value".
type backupConfigRequest struct {
	RemoteURL         string `json:"remoteUrl"`
	Branch            string `json:"branch"`
	AuthorName        string `json:"authorName"`
	AuthorEmail       string `json:"authorEmail"`
	SSHKey            string `json:"sshKey"`
	SSHKeyPath        string `json:"sshKeyPath"`
	SSHKnownHostsPath string `json:"sshKnownHostsPath"`
	HTTPUsername      string `json:"httpUsername"`
	HTTPPassword      string `json:"httpPassword"`
	IntervalMinutes   int    `json:"intervalMinutes"`
}

func (r *Routes) handleGetBackupConfig(c *gin.Context) {
	if r.mgr == nil {
		c.JSON(http.StatusOK, gin.H{"available": false})
		return
	}
	cfg, err := r.mgr.CurrentConfig()
	if err != nil {
		// Corrupt file: still answer, so the form can render with defaults and
		// the admin can overwrite it.
		slog.Warn("backup: could not read current config for settings", "error", err)
	}
	c.JSON(http.StatusOK, gin.H{
		"available":              true,
		"envManaged":             r.mgr.EnvManaged(),
		"enabled":                r.mgr.Enabled(),
		"encryptionKeyAvailable": r.mgr.CanStoreSecrets(),
		"minIntervalMinutes":     int(backupSvc.MinSettingsInterval / time.Minute),
		"maxIntervalMinutes":     int(backupSvc.MaxSettingsInterval / time.Minute),
		"config": gin.H{
			"remoteUrl":         backupSvc.RedactRemoteURL(cfg.RemoteURL),
			"branch":            cfg.Branch,
			"authorName":        cfg.AuthorName,
			"authorEmail":       cfg.AuthorEmail,
			"authMode":          authModeFor(cfg.RemoteURL),
			"sshKeyPath":        cfg.SSHKeyPath,
			"sshKnownHostsPath": cfg.SSHKnownHostsPath,
			"httpUsername":      cfg.HTTPUsername,
			"hasSshKey":         cfg.SSHKey != "",
			"hasHttpPassword":   cfg.HTTPPassword != "",
			"intervalMinutes":   intervalMinutes(cfg),
		},
		"bootError": bootErrString(r.mgr),
	})
}

func (r *Routes) handleSaveBackupConfig(c *gin.Context) {
	cfg, ok := r.bindAndValidateConfig(c)
	if !ok {
		return
	}
	if err := backupSvc.TestRemote(c.Request.Context(), cfg); err != nil {
		respondWithBackupStatusError(c, http.StatusBadRequest, ErrCodeBackupRemoteUnreachable, err.Error(), "")
		return
	}
	if err := r.mgr.Reconfigure(cfg); err != nil {
		switch {
		case errors.Is(err, backupSvc.ErrEnvManaged):
			r.respondEnvManaged(c)
		case errors.Is(err, backupSvc.ErrNoEncryptionKey):
			respondWithBackupStatusError(c, http.StatusBadRequest, ErrCodeBackupNoEncryptionKey,
				"Storing backup credentials requires an encryption key; it is unavailable when authentication is disabled", "")
		default:
			respondWithBackupStatusError(c, http.StatusInternalServerError, ErrCodeBackupInternalError, err.Error(), "backup internal error")
		}
		return
	}
	slog.Info("backup: configuration saved via settings", "remote", backupSvc.RedactRemoteURL(cfg.RemoteURL), "interval", cfg.Interval)
	r.handleGetBackupConfig(c)
}

func (r *Routes) handleTestBackupConfig(c *gin.Context) {
	cfg, ok := r.bindAndValidateConfig(c)
	if !ok {
		return
	}
	if err := backupSvc.TestRemote(c.Request.Context(), cfg); err != nil {
		respondWithBackupStatusError(c, http.StatusBadRequest, ErrCodeBackupRemoteUnreachable, err.Error(), "")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (r *Routes) handleDisableBackup(c *gin.Context) {
	if r.mgr == nil {
		r.respondNotEnabled(c)
		return
	}
	if err := r.mgr.Disable(); err != nil {
		if errors.Is(err, backupSvc.ErrEnvManaged) {
			r.respondEnvManaged(c)
			return
		}
		respondWithBackupStatusError(c, http.StatusInternalServerError, ErrCodeBackupInternalError, err.Error(), "backup internal error")
		return
	}
	slog.Info("backup: disabled via settings")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// bindAndValidateConfig parses the request, rejects env-managed mode, merges
// "keep existing" secrets from the stored config, applies defaults, and runs
// Config.ValidateForSettings. It writes the error response itself on failure.
func (r *Routes) bindAndValidateConfig(c *gin.Context) (backupSvc.Config, bool) {
	if r.mgr == nil {
		r.respondNotEnabled(c)
		return backupSvc.Config{}, false
	}
	if r.mgr.EnvManaged() {
		r.respondEnvManaged(c)
		return backupSvc.Config{}, false
	}

	var req backupConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithBackupStatusError(c, http.StatusBadRequest, ErrCodeBackupInvalidConfig, "Invalid request body", "")
		return backupSvc.Config{}, false
	}

	current, _ := r.mgr.CurrentConfig()
	cfg := backupSvc.Config{
		RemoteURL:         strings.TrimSpace(req.RemoteURL),
		Branch:            strings.TrimSpace(req.Branch),
		AuthorName:        strings.TrimSpace(req.AuthorName),
		AuthorEmail:       strings.TrimSpace(req.AuthorEmail),
		SSHKeyPath:        strings.TrimSpace(req.SSHKeyPath),
		SSHKnownHostsPath: strings.TrimSpace(req.SSHKnownHostsPath),
		HTTPUsername:      strings.TrimSpace(req.HTTPUsername),
		Interval:          time.Duration(req.IntervalMinutes) * time.Minute,
	}
	// "keep existing" secrets.
	if req.SSHKey != "" {
		cfg.SSHKey = req.SSHKey
	} else {
		cfg.SSHKey = current.SSHKey
	}
	if req.HTTPPassword != "" {
		cfg.HTTPPassword = req.HTTPPassword
	} else {
		cfg.HTTPPassword = current.HTTPPassword
	}
	// Drop credentials that don't match the chosen transport so stale ones from
	// the other mode never linger in git-backup.json.
	switch authModeFor(cfg.RemoteURL) {
	case "https":
		cfg.SSHKey, cfg.SSHKeyPath, cfg.SSHKnownHostsPath = "", "", ""
	case "ssh":
		cfg.HTTPUsername, cfg.HTTPPassword = "", ""
	}

	cfg = cfg.WithSettingsDefaults()
	if err := cfg.ValidateForSettings(); err != nil {
		respondWithBackupStatusError(c, http.StatusBadRequest, ErrCodeBackupInvalidConfig, err.Error(), "")
		return backupSvc.Config{}, false
	}
	return cfg, true
}

func (r *Routes) respondEnvManaged(c *gin.Context) {
	respondWithBackupStatusError(c, http.StatusConflict, ErrCodeBackupEnvManaged,
		"Git backup is configured via environment variables and cannot be changed here", "")
}

func authModeFor(remoteURL string) string {
	l := strings.ToLower(strings.TrimSpace(remoteURL))
	switch {
	case l == "":
		return ""
	case strings.HasPrefix(l, "http://"), strings.HasPrefix(l, "https://"):
		return "https"
	default:
		return "ssh"
	}
}

func intervalMinutes(cfg backupSvc.Config) int {
	if cfg.Interval <= 0 {
		return int(backupSvc.MinSettingsInterval / time.Minute)
	}
	return int(cfg.Interval / time.Minute)
}

func bootErrString(m *backupSvc.Manager) string {
	if err := m.BootError(); err != nil {
		return err.Error()
	}
	return ""
}
