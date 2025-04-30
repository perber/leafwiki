package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func DeletePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		recursive := c.DefaultQuery("recursive", "false") == "true"

		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
			return
		}

		if err := w.DeletePage(id, recursive); err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
	}
}
