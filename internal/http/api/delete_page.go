package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
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

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		if err := w.DeletePage(user.ID, id, recursive); err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Page deleted"})
	}
}
