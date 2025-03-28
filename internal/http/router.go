package http

import (
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/api"
	"github.com/perber/wiki/internal/wiki"
)

func NewRouter(wikiInstance *wiki.Wiki) *gin.Engine {
	router := gin.Default()

	apiGroup := router.Group("/api")
	{
		// Pages
		apiGroup.POST("/pages", api.CreatePageHandler(wikiInstance))
		apiGroup.GET("/pages/:id", api.GetPageHandler(wikiInstance))
		apiGroup.PUT("/pages/:id", api.UpdatePageHandler(wikiInstance))
		apiGroup.DELETE("/pages/:id", api.DeletePageHandler(wikiInstance))

		apiGroup.POST("/pages/:id/move", api.MovePageHandler(wikiInstance))
		apiGroup.GET("/pages/slug-suggestion", api.SuggestSlugHandler(wikiInstance))

		// Tree
		apiGroup.GET("/tree", api.GetTreeHandler(wikiInstance))
	}

	return router
}
