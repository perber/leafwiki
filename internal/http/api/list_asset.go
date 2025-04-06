package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func ListAssetsHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("id")

		assets, err := w.ListAssets(pageID)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"files": assets})
	}
}
