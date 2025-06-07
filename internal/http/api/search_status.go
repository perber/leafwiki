package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func SearchStatusHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := wikiInstance.GetIndexingStatus()
		c.JSON(http.StatusOK, status)
	}
}
