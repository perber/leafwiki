package http

import (
	"embed"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/api"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/importer"
	"github.com/perber/wiki/internal/wiki"
)

//go:embed dist/**
var frontend embed.FS

// EmbedFrontend is a flag to enable or disable embedding the frontend
// This is useful for testing purposes, where we might not want to embed the frontend
// During build time, we can set this to false to disable embedding the frontend
var EmbedFrontend = "false"

// Environment is a flag to set the environment
var Environment = "development"

// Slog Wrapper for Gin (Info level)
type slogWriter struct {
	logger *slog.Logger
}

func (sw *slogWriter) Write(p []byte) (n int, err error) {
	sw.logger.Info(strings.TrimSpace(string(p)))
	return len(p), nil
}

// Slog Wrapper for Gin Errors (Error level)
type slogErrorWriter struct {
	logger *slog.Logger
}

func (sew *slogErrorWriter) Write(p []byte) (n int, err error) {
	sew.logger.Error(strings.TrimSpace(string(p)))
	return len(p), nil
}

type RouterOptions struct {
	PublicAccess            bool          // Whether the wiki allows public read access
	InjectCodeInHeader      string        // Raw HTML/JS code to inject into the <head> tag
	AllowInsecure           bool          // Whether to allow insecure HTTP connections
	AccessTokenTimeout      time.Duration // Duration for access token validity
	RefreshTokenTimeout     time.Duration // Duration for refresh token validity
	HideLinkMetadataSection bool          // Whether to hide the link metadata section in the frontend UI
	AuthDisabled            bool          // Whether authentication is disabled
	BasePath                string        // URL prefix when served behind a reverse proxy (e.g. "/wiki")
}

// wireImporterService sets up and returns an ImporterService instance
// Parameters:
//   - w: the wiki instance to use for importing
func wireImporterService(w *wiki.Wiki) *importer.ImporterService {
	slugger := w.GetSlugService()
	planner := importer.NewPlanner(w, slugger)
	store := importer.NewPlanStore()
	return importer.NewImporterService(planner, store)
}

// NewRouter creates a new HTTP router for the wiki application.
// Parameters:
//   - wikiInstance: the wiki instance to serve
//   - options: RouterOptions struct containing configuration options
func NewRouter(wikiInstance *wiki.Wiki, options RouterOptions) *gin.Engine {
	if Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Set Gin to use slog for logging
	gin.DefaultWriter = &slogWriter{logger: slog.Default().With("component", "gin")}
	gin.DefaultErrorWriter = &slogErrorWriter{logger: slog.Default().With("component", "gin")}

	importerService := wireImporterService(wikiInstance)

	router := gin.Default()

	cookiePath := "/"
	if options.BasePath != "" {
		cookiePath = options.BasePath
	}

	authCookies := auth_middleware.NewAuthCookies(options.AllowInsecure, options.AccessTokenTimeout, options.RefreshTokenTimeout, cookiePath)
	csrfCookie := security.NewCSRFCookie(options.AllowInsecure, 3*24*time.Hour, cookiePath)

	loginRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)  // limit to 10 login attempts per 5 minutes per IP - reset on success
	refreshRateLimiter := security.NewRateLimiter(30, time.Minute, false) // limit to 30 refresh attempts per minute per IP - do not reset on success

	base := router.Group(options.BasePath)

	assetsFS := gin.Dir(wikiInstance.GetAssetService().GetAssetsDir(), false) // false = no directory listing

	if options.PublicAccess || options.AuthDisabled {
		// public read access or auth disabled -> assets are publicly accessible
		base.StaticFS("/assets", assetsFS)
	} else {
		// private mode -> assets only accessible with authentication
		assetsGroup := base.Group("/assets")
		assetsGroup.Use(
			auth_middleware.InjectPublicEditor(options.AuthDisabled),
			auth_middleware.RequireAuth(wikiInstance, authCookies, options.AuthDisabled),
		)
		assetsGroup.StaticFS("/", assetsFS)
	}

	nonAuthApiGroup := base.Group("/api")
	{
		// Auth
		nonAuthApiGroup.POST("/auth/login", loginRateLimiter, api.LoginUserHandler(wikiInstance, authCookies, csrfCookie))
		nonAuthApiGroup.POST("/auth/refresh-token", refreshRateLimiter, api.RefreshTokenUserHandler(wikiInstance, authCookies, csrfCookie))
		nonAuthApiGroup.GET("/config", func(c *gin.Context) {
			if _, err := csrfCookie.Issue(c); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to issue CSRF cookie"})
				return
			}
			c.JSON(200, gin.H{"publicAccess": options.PublicAccess, "hideLinkMetadataSection": options.HideLinkMetadataSection, "authDisabled": options.AuthDisabled, "basePath": options.BasePath})
		})

		// Branding (public, no auth required)
		nonAuthApiGroup.GET("/branding", api.GetBrandingHandler(wikiInstance))

		// PUBLIC READ ACCESS (if enabled via flag or env):
		// These routes are accessible without authentication when options.PublicAccess == true.
		// Only safe, read-only operations are allowed here (GET tree/pages).
		if options.PublicAccess {
			nonAuthApiGroup.GET("/tree", api.GetTreeHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/by-path", api.GetPageByPathHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/lookup", api.LookupPagePathHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/:id/links", api.GetPageLinkStatusHandler(wikiInstance))

			// Search
			nonAuthApiGroup.GET("/search/status", api.SearchStatusHandler(wikiInstance))
			nonAuthApiGroup.GET("/search", api.SearchHandler(wikiInstance))
		}
	}

	requiresAuthGroup := base.Group("/api")
	requiresAuthGroup.Use(auth_middleware.InjectPublicEditor(options.AuthDisabled), auth_middleware.RequireAuth(wikiInstance, authCookies, options.AuthDisabled), security.CSRFMiddleware(csrfCookie))
	{
		// If public access is disabled, we need to ensure that the tree and pages routes are protected
		// and require authentication. If public access is enabled, these routes are already handled
		if !options.PublicAccess {
			requiresAuthGroup.GET("/tree", api.GetTreeHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/lookup", api.LookupPagePathHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/by-path", api.GetPageByPathHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/:id/links", api.GetPageLinkStatusHandler(wikiInstance))

			// Search
			requiresAuthGroup.GET("/search/status", api.SearchStatusHandler(wikiInstance))
			requiresAuthGroup.GET("/search", api.SearchHandler(wikiInstance))
		}

		// Auth
		requiresAuthGroup.POST("/auth/logout", api.LogoutUserHandler(wikiInstance, authCookies, csrfCookie))

		// Pages
		requiresAuthGroup.POST("/pages", auth_middleware.RequireEditorOrAdmin(), api.CreatePageHandler(wikiInstance))
		requiresAuthGroup.POST("/pages/ensure", auth_middleware.RequireEditorOrAdmin(), api.EnsurePageHandler(wikiInstance))
		requiresAuthGroup.POST("/pages/convert/:id", auth_middleware.RequireEditorOrAdmin(), api.ConvertPageHandler(wikiInstance))
		requiresAuthGroup.POST("/pages/copy/:id", auth_middleware.RequireEditorOrAdmin(), api.CopyPageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id", auth_middleware.RequireEditorOrAdmin(), api.UpdatePageHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id", auth_middleware.RequireEditorOrAdmin(), api.DeletePageHandler(wikiInstance))

		requiresAuthGroup.PUT("/pages/:id/move", auth_middleware.RequireEditorOrAdmin(), api.MovePageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/sort", auth_middleware.RequireEditorOrAdmin(), api.SortPagesHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/slug-suggestion", auth_middleware.RequireEditorOrAdmin(), api.SuggestSlugHandler(wikiInstance))

		// User
		requiresAuthGroup.POST("/users", auth_middleware.RequireAdmin(options.AuthDisabled), api.CreateUserHandler(wikiInstance))
		requiresAuthGroup.GET("/users", auth_middleware.RequireAdmin(options.AuthDisabled), api.GetUsersHandler(wikiInstance))
		requiresAuthGroup.PUT("/users/:id", auth_middleware.RequireSelfOrAdmin(options.AuthDisabled), api.UpdateUserHandler(wikiInstance))
		requiresAuthGroup.DELETE("/users/:id", auth_middleware.RequireAdmin(options.AuthDisabled), api.DeleteUserHandler(wikiInstance))

		// Change Own Password (only meaningful when authentication is enabled)
		if !options.AuthDisabled {
			requiresAuthGroup.PUT("/users/me/password", api.ChangeOwnPasswordUserHandler(wikiInstance))
			// Branding (admin only)
			// Only allowed when authentication is enabled
			requiresAuthGroup.PUT("/branding", auth_middleware.RequireAdmin(options.AuthDisabled), api.UpdateBrandingHandler(wikiInstance))
			requiresAuthGroup.POST("/branding/logo", auth_middleware.RequireAdmin(options.AuthDisabled), api.UploadBrandingLogoHandler(wikiInstance))
			requiresAuthGroup.POST("/branding/favicon", auth_middleware.RequireAdmin(options.AuthDisabled), api.UploadBrandingFaviconHandler(wikiInstance))
			requiresAuthGroup.DELETE("/branding/logo", auth_middleware.RequireAdmin(options.AuthDisabled), api.DeleteBrandingLogoHandler(wikiInstance))
			requiresAuthGroup.DELETE("/branding/favicon", auth_middleware.RequireAdmin(options.AuthDisabled), api.DeleteBrandingFaviconHandler(wikiInstance))
		}

		// Assets
		requiresAuthGroup.POST("/pages/:id/assets", auth_middleware.RequireEditorOrAdmin(), api.UploadAssetHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/:id/assets", auth_middleware.RequireEditorOrAdmin(), api.ListAssetsHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/assets/rename", auth_middleware.RequireEditorOrAdmin(), api.RenameAssetHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id/assets/:name", auth_middleware.RequireEditorOrAdmin(), api.DeleteAssetHandler(wikiInstance))

		// Importer
		requiresAuthGroup.POST("/import/plan", auth_middleware.RequireEditorOrAdmin(), api.CreateImportPlanHandler(importerService))
		requiresAuthGroup.GET("/import/plan", auth_middleware.RequireEditorOrAdmin(), api.GetImportPlanHandler(importerService))
		requiresAuthGroup.POST("/import/execute", auth_middleware.RequireEditorOrAdmin(), api.ExecuteImportHandler(importerService, wikiInstance))
		requiresAuthGroup.DELETE("/import/plan", auth_middleware.RequireEditorOrAdmin(), api.ClearImportPlanHandler(importerService))
	}

	// Serve branding assets (logos, favicons) with extension validation
	base.GET("/branding/:filename", func(c *gin.Context) {
		filename := c.Param("filename")

		// Sanitize filename to prevent directory traversal and malicious input
		// Only allow simple filenames (no path separators, no null bytes, no ..)
		if strings.Contains(filename, "..") ||
			strings.Contains(filename, "/") ||
			strings.Contains(filename, "\\") ||
			strings.Contains(filename, "\x00") {
			c.Status(http.StatusForbidden)
			return
		}

		// Get allowed extensions from branding constraints
		constraints, err := wikiInstance.GetBrandingConstraints()
		if err != nil {
			log.Printf("Failed to get branding constraints: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		// Build a combined set of allowed extensions for O(1) lookup
		allowedExts := make(map[string]bool)
		for _, ext := range constraints.LogoExts {
			allowedExts[ext] = true
		}
		for _, ext := range constraints.FaviconExts {
			allowedExts[ext] = true
		}

		// Validate file extension against whitelist
		ext := strings.ToLower(filepath.Ext(filename))
		if !allowedExts[ext] {
			c.Status(http.StatusForbidden)
			return
		}

		// Construct file path
		brandingDir := wikiInstance.GetBrandingService().GetBrandingAssetsDir()
		filePath := filepath.Join(brandingDir, filename)

		// Clean the path and verify it's within the branding directory
		cleanPath := filepath.Clean(filePath)
		cleanBrandingDir := filepath.Clean(brandingDir)

		// Ensure the resolved path is still within the branding directory
		// Use filepath.Rel to check the relative path doesn't escape the directory
		rel, err := filepath.Rel(cleanBrandingDir, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			c.Status(http.StatusForbidden)
			return
		}

		// Check if file exists
		if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
			c.Status(http.StatusNotFound)
			return
		} else if err != nil {
			log.Printf("Error checking file existence: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		// Serve the file
		c.File(cleanPath)
	})

	// If frontend embedding is enabled, serve it on all unknown routes
	if EmbedFrontend == "true" {
		fsys, err := fs.Sub(frontend, "dist")
		if err != nil {
			panic("failed to create sub FS: " + err.Error())
		}

		staticFS, err := fs.Sub(frontend, "dist/static")
		if err != nil {
			panic("failed to create sub FS: " + err.Error())
		}

		// Serve the embedded frontend files js, css, ...
		base.StaticFS("/static", http.FS(staticFS))

		base.GET("/favicon.svg", func(c *gin.Context) {
			// Get branding config to check for custom favicon
			brandingConfig, err := wikiInstance.GetBranding()
			if err == nil && brandingConfig.FaviconFile != "" {
				// Serve custom favicon from branding assets
				faviconPath := filepath.Join(wikiInstance.GetBrandingService().GetBrandingAssetsDir(), brandingConfig.FaviconFile)
				c.File(faviconPath)
				return
			}

			// Serve default leaf favicon as SVG
			svgContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🌿</text></svg>`
			c.Data(http.StatusOK, "image/svg+xml", []byte(svgContent))
		})

		router.NoRoute(func(c *gin.Context) {
			// Strip basePath prefix to check known route prefixes
			path := c.Request.URL.Path
			if options.BasePath != "" {
				path = strings.TrimPrefix(path, options.BasePath)
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

				// get site name from branding config
				var siteName string = "LeafWiki"
				if branding, err := wikiInstance.GetBranding(); err == nil {
					siteName = branding.SiteName
				}

				html := string(data)
				html = strings.ReplaceAll(html, "{{__SITE_NAME__}}", siteName)
				html = strings.ReplaceAll(html, "{{__BASE_PATH__}}", options.BasePath)

				// Rewrite absolute asset paths in the built HTML so the browser
				// fetches them under the base path (e.g. /wiki/static/...).
				if options.BasePath != "" {
					html = strings.ReplaceAll(html, `"/static/`, `"`+options.BasePath+`/static/`)
					html = strings.ReplaceAll(html, `"/favicon.svg"`, `"`+options.BasePath+`/favicon.svg"`)
				}

				if options.InjectCodeInHeader != "" {
					// replaces the closing </head> tag with the injected code
					newHtml := strings.Replace(html, "</head>", "  "+options.InjectCodeInHeader+"\n  </head>", 1)
					if newHtml == html {
						log.Printf("Warning: could not inject code into header, </head> tag not found")
					}
					html = newHtml
				}

				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
			} else {
				c.String(http.StatusNotFound, "Page not found")
			}
		})

	}

	return router
}
