package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func DeleteAssetHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		pageID := c.Param("id")
		filename := c.Param("name")

		if filename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing filename"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		if err := w.DeleteAsset(user.ID, pageID, filename); err != nil {
			respondWithError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "asset deleted"})
	}
}
