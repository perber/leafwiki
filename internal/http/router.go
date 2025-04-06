package http

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/api"
	"github.com/perber/wiki/internal/http/middleware"
	"github.com/perber/wiki/internal/wiki"
)

func NewRouter(wikiInstance *wiki.Wiki) *gin.Engine {
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	router.StaticFS("/assets", gin.Dir(wikiInstance.GetStorageDir(), true))

	nonAuthApiGroup := router.Group("/api")
	{
		// Auth
		nonAuthApiGroup.POST("/auth/login", api.LoginUserHandler(wikiInstance))
		nonAuthApiGroup.POST("/auth/refresh-token", api.RefreshTokenUserHandler(wikiInstance))
	}

	requiresAuthGroup := router.Group("/api")
	requiresAuthGroup.Use(middleware.RequireAuth(wikiInstance))
	{
		// Pages
		requiresAuthGroup.POST("/pages", api.CreatePageHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id", api.UpdatePageHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id", api.DeletePageHandler(wikiInstance))
		requiresAuthGroup.GET("pages/by-path", api.GetPageByPathHandler(wikiInstance))

		requiresAuthGroup.PUT("/pages/:id/move", api.MovePageHandler(wikiInstance))
		requiresAuthGroup.PUT("/pages/:id/sort", api.SortPagesHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/slug-suggestion", api.SuggestSlugHandler(wikiInstance))

		// Tree
		requiresAuthGroup.GET("/tree", api.GetTreeHandler(wikiInstance))

		// User
		requiresAuthGroup.POST("/users", middleware.RequireAdmin(wikiInstance), api.CreateUserHandler(wikiInstance))
		requiresAuthGroup.GET("/users", api.GetUsersHandler(wikiInstance))
		requiresAuthGroup.PUT("/users/:id", middleware.RequireAdmin(wikiInstance), api.UpdateUserHandler(wikiInstance))
		requiresAuthGroup.DELETE("/users/:id", middleware.RequireAdmin(wikiInstance), api.DeleteUserHandler(wikiInstance))

		// Assets
		requiresAuthGroup.POST("/pages/:id/assets", api.UploadAssetHandler(wikiInstance))
		requiresAuthGroup.GET("/pages/:id/assets", api.ListAssetsHandler(wikiInstance))
		requiresAuthGroup.DELETE("/pages/:id/assets/:name", api.DeleteAssetHandler(wikiInstance))

	}

	return router
}
