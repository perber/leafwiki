package http

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/api"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
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

type RouterOptions struct {
	PublicAccess            bool          // Whether the wiki allows public read access
	InjectCodeInHeader      string        // Raw HTML/JS code to inject into the <head> tag
	AllowInsecure           bool          // Whether to allow insecure HTTP connections
	AccessTokenTimeout      time.Duration // Duration for access token validity
	RefreshTokenTimeout     time.Duration // Duration for refresh token validity
	HideLinkMetadataSection bool          // Whether to hide the link metadata section in the frontend UI
	AuthDisabled            bool          // Whether authentication is disabled
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

	router := gin.Default()
	router.StaticFS("/assets", gin.Dir(wikiInstance.GetAssetService().GetAssetsDir(), true))

	authCookies := auth_middleware.NewAuthCookies(options.AllowInsecure, options.AccessTokenTimeout, options.RefreshTokenTimeout)
	csrfCookie := security.NewCSRFCookie(options.AllowInsecure, 3*24*time.Hour)

	loginRateLimiter := security.NewRateLimiter(10, 5*time.Minute, true)  // limit to 10 login attempts per 5 minutes per IP - reset on success
	refreshRateLimiter := security.NewRateLimiter(30, time.Minute, false) // limit to 30 refresh attempts per minute per IP - do not reset on success

	nonAuthApiGroup := router.Group("/api")
	{
		// Auth
		nonAuthApiGroup.POST("/auth/login", loginRateLimiter, api.LoginUserHandler(wikiInstance, authCookies, csrfCookie))
		nonAuthApiGroup.POST("/auth/refresh-token", refreshRateLimiter, api.RefreshTokenUserHandler(wikiInstance, authCookies, csrfCookie))
		nonAuthApiGroup.GET("/config", func(c *gin.Context) {
			if _, err := csrfCookie.Issue(c); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to issue CSRF cookie"})
				return
			}
			c.JSON(200, gin.H{"publicAccess": options.PublicAccess, "hideLinkMetadataSection": options.HideLinkMetadataSection, "authDisabled": options.AuthDisabled})
		})

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

	requiresAuthGroup := router.Group("/api")
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
		}

		// Assets
		requiresAuthGroup.POST("/pages/:id/assets", auth_middleware.RequireEditorOrAdmin(), api.UploadAssetHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/:id/assets", auth_middleware.RequireEditorOrAdmin(), api.ListAssetsHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/assets/rename", auth_middleware.RequireEditorOrAdmin(), api.RenameAssetHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id/assets/:name", auth_middleware.RequireEditorOrAdmin(), api.DeleteAssetHandler(wikiInstance))

		// Branding (admin only)
		requiresAuthGroup.PUT("/branding", auth_middleware.RequireAdmin(options.AuthDisabled), api.UpdateBrandingHandler(wikiInstance))
		requiresAuthGroup.POST("/branding/logo", auth_middleware.RequireAdmin(options.AuthDisabled), api.UploadBrandingLogoHandler(wikiInstance))
		requiresAuthGroup.POST("/branding/favicon", auth_middleware.RequireAdmin(options.AuthDisabled), api.UploadBrandingFaviconHandler(wikiInstance))
	}

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
		router.StaticFS("/static", http.FS(staticFS))

		router.GET("/favicon.svg", func(c *gin.Context) {
			file, err := fsys.Open("favicon.svg")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			stat, err := file.Stat()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}

			c.DataFromReader(http.StatusOK, stat.Size(), "image/svg+xml", file, nil)
		})

		router.NoRoute(func(c *gin.Context) {
			if c.Request.Method == http.MethodGet &&
				!strings.HasPrefix(c.Request.URL.Path, "/api") &&
				!strings.HasPrefix(c.Request.URL.Path, "/assets") &&
				!strings.HasPrefix(c.Request.URL.Path, "/static") {

				c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
				data, err := fs.ReadFile(fsys, "index.html")
				if err != nil {
					c.Status(http.StatusNotFound)
					return
				}

				if options.InjectCodeInHeader != "" {
					html := string(data)
					// replaces the closing </head> tag with the injected code
					newHtml := strings.Replace(html, "</head>", "  "+options.InjectCodeInHeader+"\n  </head>", 1)
					if newHtml == html {
						log.Printf("Warning: could not inject code into header, </head> tag not found")
					}
					data = []byte(newHtml)
				}

				c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			} else {
				c.String(http.StatusNotFound, "Page not found")
			}
		})

	}

	return router
}
