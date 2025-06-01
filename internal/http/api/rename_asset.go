package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func RenameAssetHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("id")
		oldFilename := c.Query("old_filename")
		newFilename := c.Query("new_filename")

		url, err := w.RenameAsset(pageID, oldFilename, newFilename)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}
