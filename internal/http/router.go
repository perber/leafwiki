package http

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/assets"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/maintenance"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/publicaccess"
	"github.com/perber/wiki/internal/restore"
	"github.com/perber/wiki/internal/tocdisplay"
)

//go:embed dist/**
var frontend embed.FS

// EmbedFrontend is a flag to enable or disable embedding the frontend.
// Set to "true" at build time to embed the SPA.
var EmbedFrontend = "false"

// Environment controls gin's run mode ("production" → ReleaseMode).
var Environment = "development"

const DefaultFaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🌿</text></svg>`

// slogWriter forwards gin debug output (e.g. route registration) to slog at Debug level.
type slogWriter struct{ logger *slog.Logger }

func (sw *slogWriter) Write(p []byte) (n int, err error) {
	sw.logger.Debug(strings.TrimSpace(string(p)))
	return len(p), nil
}

// slogErrorWriter forwards gin Error logs to slog.
type slogErrorWriter struct{ logger *slog.Logger }

func (sew *slogErrorWriter) Write(p []byte) (n int, err error) {
	sew.logger.Error(strings.TrimSpace(string(p)))
	return len(p), nil
}

func slogRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path += "?" + raw
		}

		c.Next()

		slog.Default().Info("http request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start),
			"ip", c.ClientIP(),
		)
	}
}

func disableClientCache(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", time.Unix(0, 0).UTC().Format(http.TimeFormat))
}

// HTTPRemoteUserConfig configures reverse-proxy-based authentication.
type HTTPRemoteUserConfig struct {
	Enabled         bool
	HeaderName      string
	AutoCreate      bool   // Whether to auto-provision users asserted by the proxy but unknown to LeafWiki
	EmailHeaderName string // Optional header supplying the email for auto-created users
	DefaultRole     string // Role assigned to auto-created users
	TrustedProxies  *auth_middleware.TrustedProxies
	// UserService is resolved on every request rather than captured once
	// here — see auth_middleware.RemoteUserConfig.UserService.
	UserService func() *coreauth.UserService
}

// RouterOptions holds global HTTP server configuration shared across all domains.
type RouterOptions struct {
	// PublicAccess is the current "public mode" state (anonymous read access to
	// every page), read per request so it can be toggled at runtime with no
	// restart. nil is treated as a fixed-false provider. See internal/publicaccess.
	PublicAccess publicaccess.Provider
	// AlwaysShowToc is the current "always show table of contents" state
	// (show the TOC panel/dropdown regardless of heading count), read per
	// request so it can be toggled at runtime with no restart. nil is
	// treated as a fixed-false provider. See internal/tocdisplay.
	AlwaysShowToc           tocdisplay.Provider
	EditorLimit             int                      // Max admin+editor users allowed; 0 = unlimited
	InjectCodeInHeader      string                   // Raw HTML/JS code to inject into the <head> tag
	CustomStylesheet        string                   // Path to a custom CSS file (resolved by wiki before passing)
	AllowInsecure           bool                     // Whether to allow insecure HTTP connections
	AccessTokenTimeout      time.Duration            // Duration for access token validity
	RefreshTokenTimeout     time.Duration            // Duration for refresh token validity
	HideLinkMetadataSection bool                     // Whether to hide the link metadata section in the frontend UI
	AuthDisabled            bool                     // Whether authentication is disabled
	BasePath                string                   // URL prefix when served behind a reverse proxy (e.g. "/wiki")
	MaxAssetUploadSizeBytes int64                    // Maximum allowed size in bytes for asset uploads
	EnableRevision          bool                     // Whether the revision / page history feature is enabled
	EnableLinkRefactor      bool                     // Whether the link refactoring feature is enabled in the frontend
	EnableAPIKeyManagement  bool                     // Whether the experimental API key management feature is enabled
	Metrics                 *httpmetrics.HTTPMetrics // Optional Prometheus HTTP metrics collector; nil disables request instrumentation
	GitBackupEnabled        bool                     // Whether git backup is currently running (surfaced to admin UI via /api/config)
	GitBackupEnvManaged     bool                     // Whether git backup is configured via flags/env (settings UI is status-only); surfaced via /api/config
	GitBackupConfigured     bool                     // Whether git backup is set up at all (running, env-managed, booting, or boot-failed); drives the header health indicator
	SnapshotEnabled         bool                     // Whether full-backup (snapshot) is enabled (surfaced to admin UI via /api/config)
	SMTPEnabled             bool                     // Whether SMTP (password reset / user invite email) is configured (surfaced to UI via /api/config)
	TOTPAvailable           bool                     // Whether a TOTP encryption key is configured, i.e. TOTP self-service can be offered (surfaced to UI via /api/config)
	HTTPRemoteUser          HTTPRemoteUserConfig     // Reverse-proxy authentication via HTTP header
	APIKeyService           *coreauth.APIKeyService  // Bearer API-key authentication; nil disables the feature
	DisableRequestLog       bool                     // Whether to suppress per-request access log lines
	UserManagementURL       string                   // Optional URL; when set, the frontend replaces in-app user management with a link to this URL
	DefaultLanguage         string                   // Optional default UI language code (e.g. "de"); frontend applies it only if it matches a language it ships
	LoginURL                string                   // Optional URL the frontend redirects to instead of showing the built-in login form
	LogoutURL               string                   // Optional URL the frontend redirects to after logout
	WriteGate               *restore.WriteGate       // Optional; when set, gates mutating requests while a restore is in progress. nil disables the middleware entirely (no snapshot/restore enabled)
}

// FrontendConfig carries the minimal runtime data required to serve the embedded SPA.
type FrontendConfig struct {
	// GetSiteName returns the current site name injected into the HTML.
	GetSiteName func() string
	// GetFaviconFile returns the current branding favicon filename for initial HTML rendering.
	GetFaviconFile func() string
	// CustomStylesheetPath is the fully-resolved, validated path to a custom CSS file.
	// Empty string disables custom stylesheet serving.
	CustomStylesheetPath string
	// StorageDir is used to validate that CustomStylesheet in RouterOptions is within the storage dir.
	StorageDir string
	// ShellResponders may answer SPA requests instead of the shell; the first
	// that handles a request wins. See shell.go for the contract.
	ShellResponders []ShellResponder
	// ShellContributors add title, head and body markup to the SPA shell, in
	// this order, before the built-in custom stylesheet and InjectCodeInHeader.
	ShellContributors []ShellContributor
}

// NewRouter creates the HTTP engine, builds the shared RouterContext, delegates all
// API and static routes to the provided registrars, and wires up the embedded SPA.
func NewRouter(registrars []RouteRegistrar, frontendCfg FrontendConfig, opts RouterOptions) *gin.Engine {
	if opts.MaxAssetUploadSizeBytes <= 0 {
		opts.MaxAssetUploadSizeBytes = assets.DefaultMaxUploadSizeBytes
	}

	if opts.PublicAccess == nil {
		opts.PublicAccess = publicaccess.Fixed(false)
	}

	if opts.AlwaysShowToc == nil {
		opts.AlwaysShowToc = tocdisplay.Fixed(false)
	}

	if Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	gin.DefaultWriter = &slogWriter{logger: slog.Default().With("component", "gin")}
	gin.DefaultErrorWriter = &slogErrorWriter{logger: slog.Default().With("component", "gin")}

	authCookies := auth_middleware.NewAuthCookies(opts.AllowInsecure, opts.AccessTokenTimeout, opts.RefreshTokenTimeout)
	csrfCookie := security.NewCSRFCookie(opts.AllowInsecure, 3*24*time.Hour)

	engine := gin.New()
	if !opts.DisableRequestLog {
		engine.Use(slogRequestLogger())
	}
	if opts.Metrics != nil {
		engine.Use(opts.Metrics.Middleware())
	}
	engine.Use(gin.RecoveryWithWriter(gin.DefaultErrorWriter))
	base := engine.Group(opts.BasePath)

	if opts.WriteGate != nil {
		base.Use(maintenance.WriteGateMiddleware(opts.WriteGate))
	}

	if opts.HTTPRemoteUser.Enabled {
		base.Use(auth_middleware.InjectRemoteUser(auth_middleware.RemoteUserConfig{
			Enabled:         opts.HTTPRemoteUser.Enabled,
			HeaderName:      opts.HTTPRemoteUser.HeaderName,
			AutoCreate:      opts.HTTPRemoteUser.AutoCreate,
			EmailHeaderName: opts.HTTPRemoteUser.EmailHeaderName,
			DefaultRole:     opts.HTTPRemoteUser.DefaultRole,
			TrustedProxies:  opts.HTTPRemoteUser.TrustedProxies,
			UserService:     opts.HTTPRemoteUser.UserService,
		}))
	}

	if opts.APIKeyService != nil {
		// Deliberate design choice: a key is injected globally, so it is
		// exactly as powerful as role(user) ∩ role(key) everywhere the normal
		// RequireAuth/RequireAdmin/RequireEditorOrAdmin guards apply — the
		// same way roles work everywhere else in the app — rather than being
		// restricted to a curated subset of read routes. CSRFMiddleware skips
		// requests with a resolved key, so writes follow the same role guards;
		// routes that manage credentials, sessions or users opt out with
		// RequireCookieSession (see InjectAPIKeyUser's doc comment).
		base.Use(auth_middleware.InjectAPIKeyUser(auth_middleware.APIKeyConfig{
			Service: opts.APIKeyService,
			// Only failed attempts count toward the limit (NotifyResult
			// resets on success), so this never throttles a valid key's
			// normal traffic — it exists to cap the cost of prefix/secret
			// guessing now that Resolve uses a fast constant-time compare.
			RateLimiter: security.NewKeyedLimiter(20, 5*time.Minute, true),
		}))
	}

	ctx := RouterContext{
		Engine:      engine,
		Base:        base,
		AuthCookies: authCookies,
		CSRFCookie:  csrfCookie,
		Opts:        opts,
	}

	for _, r := range registrars {
		r.RegisterRoutes(ctx)
	}

	// Resolve custom stylesheet: prefer pre-validated FrontendConfig path,
	// fall back to normalizing opts.CustomStylesheet against StorageDir.
	customStylesheetPath := frontendCfg.CustomStylesheetPath
	if customStylesheetPath == "" && opts.CustomStylesheet != "" {
		resolved, err := NormalizeCustomStylesheetPath(frontendCfg.StorageDir, opts.CustomStylesheet)
		if err != nil {
			slog.Default().Error("custom stylesheet disabled", "error", err)
		} else {
			customStylesheetPath = resolved
		}
	}

	// Serve custom stylesheet if a valid path was provided.
	if customStylesheetPath != "" {
		cssPath := customStylesheetPath
		base.GET("/custom.css", func(c *gin.Context) {
			if _, err := os.Stat(cssPath); os.IsNotExist(err) {
				c.Status(http.StatusNotFound)
				return
			} else if err != nil {
				slog.Default().Error("error checking custom stylesheet existence", "error", err, "path", cssPath)
				c.Status(http.StatusInternalServerError)
				return
			}
			c.Header("Content-Type", "text/css; charset=utf-8")
			c.File(cssPath)
		})
	}

	// Unknown routes: shell responders, then the embedded SPA shell (see shell.go).
	shell := newShellHandler(frontendCfg, opts, customStylesheetPath)
	if EmbedFrontend == "true" {
		fsys, err := fs.Sub(frontend, "dist")
		if err != nil {
			panic("failed to create sub FS: " + err.Error())
		}
		staticFS, err := fs.Sub(frontend, "dist/static")
		if err != nil {
			panic("failed to create sub FS: " + err.Error())
		}

		base.StaticFS("/static", http.FS(staticFS))

		base.GET("/favicon.svg", func(c *gin.Context) {
			disableClientCache(c)
			// favicon is served by the branding registrar if a custom one exists;
			// fall back to the default leaf SVG.
			c.Data(http.StatusOK, "image/svg+xml", []byte(DefaultFaviconSVG))
		})

		shell.setIndex(fs.ReadFile(fsys, "index.html"))
	}
	engine.NoRoute(shell.serve)

	return engine
}

func BuildFrontendFaviconHref(basePath, faviconFile string) string {
	if faviconFile != "" {
		return basePath + "/branding/" + faviconFile
	}

	return basePath + "/favicon.svg"
}

// NormalizeCustomStylesheetPath resolves and validates a CSS path relative to storageDir.
// Returns empty string (no error) if cssPath is empty.
func NormalizeCustomStylesheetPath(storageDir, customStylesheet string) (string, error) {
	cssPath := strings.TrimSpace(customStylesheet)
	if cssPath == "" {
		return "", nil
	}

	if strings.ToLower(filepath.Ext(cssPath)) != ".css" {
		return "", os.ErrPermission
	}

	if !filepath.IsAbs(cssPath) {
		cssPath = filepath.Join(storageDir, cssPath)
	}

	cleanStorageDir := filepath.Clean(storageDir)
	cleanCSSPath := filepath.Clean(cssPath)

	relPath, err := filepath.Rel(cleanStorageDir, cleanCSSPath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}

	return cleanCSSPath, nil
}

func buildCustomStylesheetTag(basePath, customStylesheet string) string {
	if strings.TrimSpace(customStylesheet) == "" {
		return ""
	}
	return `<link rel="stylesheet" href="` + basePath + `/custom.css">`
}

func injectIntoHead(html, snippet string) string {
	if strings.TrimSpace(snippet) == "" {
		return html
	}
	newHTML := strings.Replace(html, "</head>", "  "+snippet+"\n  </head>", 1)
	if newHTML == html {
		slog.Default().Warn("could not inject code into header", "reason", "</head> tag not found")
	}
	return newHTML
}
