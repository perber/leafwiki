package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func GetPageByPathHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
			return
		}

		page, err := w.FindByPath(path)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}
