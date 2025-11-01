package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func SuggestSlugHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		parentID := c.Query("parentID")
		currentID := c.Query("currentID")
		title := c.Query("title")

		if title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "title query param is required"})
			return
		}

		slug, err := w.SuggestSlug(parentID, currentID, title)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"slug": slug})
	}
}
