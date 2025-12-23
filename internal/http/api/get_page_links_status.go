package api

import (
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func GetPageLinkStatusHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("id")

		status, err := w.GetLinkStatusForPage(pageID)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(200, status)
	}
}
