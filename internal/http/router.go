package http

import (
	"embed"
	"html"
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
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
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
	Enabled        bool
	HeaderName     string
	TrustedProxies *auth_middleware.TrustedProxies
	UserService    *coreauth.UserService
	LogoutURL      string // optional URL the frontend redirects to after logout
}

// RouterOptions holds global HTTP server configuration shared across all domains.
type RouterOptions struct {
	PublicAccess            bool                 // Whether the wiki allows public read access
	InjectCodeInHeader      string               // Raw HTML/JS code to inject into the <head> tag
	CustomStylesheet        string               // Path to a custom CSS file (resolved by wiki before passing)
	AllowInsecure           bool                 // Whether to allow insecure HTTP connections
	AccessTokenTimeout      time.Duration        // Duration for access token validity
	RefreshTokenTimeout     time.Duration        // Duration for refresh token validity
	HideLinkMetadataSection bool                 // Whether to hide the link metadata section in the frontend UI
	AuthDisabled            bool                 // Whether authentication is disabled
	BasePath                string               // URL prefix when served behind a reverse proxy (e.g. "/wiki")
	MaxAssetUploadSizeBytes int64                // Maximum allowed size in bytes for asset uploads
	EnableRevision          bool                 // Whether the revision / page history feature is enabled
	EnableLinkRefactor      bool                 // Whether the link refactoring feature is enabled in the frontend
	GitBackupEnabled        bool                 // Whether git backup is enabled (surfaced to admin UI via /api/config)
	HTTPRemoteUser          HTTPRemoteUserConfig // Reverse-proxy authentication via HTTP header
	DisableRequestLog       bool                 // Whether to suppress per-request access log lines
	UserManagementURL       string               // Optional URL; when set, the frontend replaces in-app user management with a link to this URL
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
}

// NewRouter creates the HTTP engine, builds the shared RouterContext, delegates all
// API and static routes to the provided registrars, and wires up the embedded SPA.
func NewRouter(registrars []RouteRegistrar, frontendCfg FrontendConfig, opts RouterOptions) *gin.Engine {
	if opts.MaxAssetUploadSizeBytes <= 0 {
		opts.MaxAssetUploadSizeBytes = assets.DefaultMaxUploadSizeBytes
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
	engine.Use(gin.RecoveryWithWriter(gin.DefaultErrorWriter))
	base := engine.Group(opts.BasePath)

	if opts.HTTPRemoteUser.Enabled {
		base.Use(auth_middleware.InjectRemoteUser(auth_middleware.RemoteUserConfig{
			Enabled:        opts.HTTPRemoteUser.Enabled,
			HeaderName:     opts.HTTPRemoteUser.HeaderName,
			TrustedProxies: opts.HTTPRemoteUser.TrustedProxies,
			UserService:    opts.HTTPRemoteUser.UserService,
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

	// Serve the embedded frontend SPA on all unknown routes.
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

		engine.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if opts.BasePath != "" {
				if path != opts.BasePath && !strings.HasPrefix(path, opts.BasePath+"/") {
					c.String(http.StatusNotFound, "Page not found")
					return
				}
				path = strings.TrimPrefix(path, opts.BasePath)
				if path == "" {
					path = "/"
				}
			}

			if c.Request.Method == http.MethodGet &&
				!strings.HasPrefix(path, "/api") &&
				!strings.HasPrefix(path, "/assets") &&
				!strings.HasPrefix(path, "/static") &&
				!strings.HasPrefix(path, "/branding") {

				c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
				data, err := fs.ReadFile(fsys, "index.html")
				if err != nil {
					c.Status(http.StatusNotFound)
					return
				}

				siteName := "LeafWiki"
				if frontendCfg.GetSiteName != nil {
					if name := frontendCfg.GetSiteName(); name != "" {
						siteName = name
					}
				}
				faviconFile := ""
				if frontendCfg.GetFaviconFile != nil {
					faviconFile = frontendCfg.GetFaviconFile()
				}

				doc := string(data)
				escapedBasePath := html.EscapeString(opts.BasePath)
				doc = strings.ReplaceAll(doc, "{{__SITE_NAME__}}", html.EscapeString(siteName))
				doc = strings.ReplaceAll(doc, "{{__BASE_PATH__}}", escapedBasePath)
				doc = strings.ReplaceAll(doc, "{{__FAVICON_HREF__}}", html.EscapeString(BuildFrontendFaviconHref(opts.BasePath, faviconFile)))
				// Rewrite Vite's relative "./static/" asset references to absolute paths
				// so they resolve correctly when index.html is served for deep SPA routes.
				// Lazy chunks still use import.meta.url for their own path resolution.
				doc = strings.ReplaceAll(doc, `"./static/`, `"`+escapedBasePath+`/static/`)

				doc = injectIntoHead(doc, buildCustomStylesheetTag(opts.BasePath, customStylesheetPath))

				if opts.InjectCodeInHeader != "" {
					doc = injectIntoHead(doc, opts.InjectCodeInHeader)
				}
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(doc))
			} else {
				c.String(http.StatusNotFound, "Page not found")
			}
		})
	}

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
