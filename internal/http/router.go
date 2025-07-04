package http

import (
	"embed"
	"io/fs"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/ssr"
	"github.com/perber/wiki/internal/http/api"
	"github.com/perber/wiki/internal/http/middleware"
	"github.com/perber/wiki/internal/wiki"
)

//go:embed dist/**
var frontend embed.FS

// EnableCors is a flag to enable or disable CORS
// This is useful for testing purposes, where we might not want to enable CORS
// During build time, we can set this to false to disable CORS
var EnableCors = "true"

// EmbedFrontend is a flag to enable or disable embedding the frontend
// This is useful for testing purposes, where we might not want to embed the frontend
// During build time, we can set this to false to disable embedding the frontend
var EmbedFrontend = "false"

// Environment is a flag to set the environment
var Environment = "development"

func NewRouter(wikiInstance *wiki.Wiki, publicAccess bool) *gin.Engine {
	if Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.Default()
	if EnableCors == "true" {
		router.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	nonAuthApiGroup := router.Group("/api")
	{
		// Auth
		nonAuthApiGroup.POST("/auth/login", api.LoginUserHandler(wikiInstance))
		nonAuthApiGroup.POST("/auth/refresh-token", api.RefreshTokenUserHandler(wikiInstance))
		nonAuthApiGroup.GET("/config", func(c *gin.Context) {
			c.JSON(200, gin.H{"publicAccess": publicAccess})
		})

		// PUBLIC READ ACCESS (if enabled via flag or env):
		// These routes are accessible without authentication when publicAccess == true.
		// Only safe, read-only operations are allowed here (GET tree/pages).
		if publicAccess {
			nonAuthApiGroup.GET("/tree", api.GetTreeHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/by-path", api.GetPageByPathHandler(wikiInstance))
			nonAuthApiGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))

			// Search
			nonAuthApiGroup.GET("/search/status", api.SearchStatusHandler(wikiInstance))
			nonAuthApiGroup.GET("/search", api.SearchHandler(wikiInstance))
		}
	}

	requiresAuthGroup := router.Group("/api")
	requiresAuthGroup.Use(middleware.RequireAuth(wikiInstance))
	{
		// If public access is disabled, we need to ensure that the tree and pages routes are protected
		// and require authentication. If public access is enabled, these routes are already handled
		if !publicAccess {
			requiresAuthGroup.GET("/tree", api.GetTreeHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))
			requiresAuthGroup.GET("/pages/by-path", api.GetPageByPathHandler(wikiInstance))

			// Search
			requiresAuthGroup.GET("/search/status", api.SearchStatusHandler(wikiInstance))
			requiresAuthGroup.GET("/search", api.SearchHandler(wikiInstance))
		}

		// Pages
		requiresAuthGroup.POST("/pages", api.CreatePageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id", api.UpdatePageHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id", api.DeletePageHandler(wikiInstance))

		requiresAuthGroup.PUT("/pages/:id/move", api.MovePageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/sort", api.SortPagesHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/slug-suggestion", api.SuggestSlugHandler(wikiInstance))

		// User
		requiresAuthGroup.POST("/users", middleware.RequireAdmin(wikiInstance), api.CreateUserHandler(wikiInstance))
		requiresAuthGroup.GET("/users", middleware.RequireAdmin(wikiInstance), api.GetUsersHandler(wikiInstance))
		requiresAuthGroup.PUT("/users/:id", middleware.RequireSelfOrAdmin(wikiInstance), api.UpdateUserHandler(wikiInstance))
		requiresAuthGroup.DELETE("/users/:id", middleware.RequireAdmin(wikiInstance), api.DeleteUserHandler(wikiInstance))

		// Change Own Password
		requiresAuthGroup.PUT("/users/me/password", api.ChangeOwnPasswordUserHandler(wikiInstance))

		// Assets
		requiresAuthGroup.POST("/pages/:id/assets", api.UploadAssetHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/:id/assets", api.ListAssetsHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/assets/rename", api.RenameAssetHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id/assets/:name", api.DeleteAssetHandler(wikiInstance))
	}

	var embeddedFS fs.FS
	var err error
	embeddedFS, err = serveFrontend(router, wikiInstance)
	if err != nil {
		panic("Failed to serve embedded frontend: " + err.Error())
	}

	router.NoRoute(func(c *gin.Context) {
		url := c.Request.URL.Path
		userIsLoggedIn := false

		if ssr.IsApiPath(url) {
			// If the path is an API we will render a json error response
			c.JSON(404, gin.H{"error": "API endpoint not found"})
			return
		}

		if ssr.IsAuthPath(url) {
			// If the path is an authentication-related path, we render the SPA page
			ssr.RenderSPAPage(c, embeddedFS, Environment)
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			userIsLoggedIn = false
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		user, err := wikiInstance.GetAuthService().ValidateToken(token)
		if err == nil && user != nil {
			userIsLoggedIn = true
		}

		if userIsLoggedIn {
			// the user is logged in, so we render the SPA page
			ssr.RenderSPAPage(c, embeddedFS, Environment)
			return
		}

		pageExists := wikiInstance.DoesPageExist(url)
		if publicAccess {
			// If the user is not logged in but public access is enabled, we render the public page
			if pageExists {
				ssr.RenderPublicPage(c, embeddedFS, wikiInstance, Environment)
				return
			}

			ssr.RenderNotFoundPublicPage(c, embeddedFS, Environment)
			return
		}

		// public access it disabled. We will render the SPA page.
		ssr.RenderSPAPage(c, embeddedFS, Environment)

	})

	return router
}
