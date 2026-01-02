package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func GetPageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
			return
		}

		page, err := w.GetPage(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}
