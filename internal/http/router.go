package http

import (
	"github.com/aws/aws-sdk-go-v2/aws/middleware/private/metrics/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/api"
	"github.com/perber/wiki/internal/wiki"
)

func NewRouter(wikiInstance *wiki.Wiki) *gin.Engine {
	router := gin.Default()

	router.Use(cors.Default())

	nonAuthApiGroup := router.Group("/api")
	{
		// Auth
		nonAuthApiGroup.POST("/auth/login", api.LoginUserHandler(wikiInstance))
		nonAuthApiGroup.POST("/auth/refresh-token", api.RefreshTokenUserHandler(wikiInstance))
	}

	requiresAuthGroup := router.Group("/api")
	requiresAuthGroup.Use(middleware.AuthMiddleware(wikiInstance))
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

	}

	return router
}
