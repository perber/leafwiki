package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func LookupPagePathHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Query("path")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
			return
		}

		// Lookup the page by path
		lookup, err := w.LookupPagePath(path)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "error on looking up path"})
			return
		}

		c.JSON(http.StatusOK, lookup)
	}
}
