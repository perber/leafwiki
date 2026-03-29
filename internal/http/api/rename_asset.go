package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func RenameAssetHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("id")

		var req struct {
			OldFilename string `json:"old_filename" binding:"required"`
			NewFilename string `json:"new_filename" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		url, err := w.RenameAsset(user.ID, pageID, req.OldFilename, req.NewFilename)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"url": url})
	}
}
