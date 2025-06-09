package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func SearchHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter 'q' is required"})
			return
		}

		// offset & limit
		offsetStr := c.Query("offset")
		limitStr := c.Query("limit")

		if offsetStr == "" {
			offsetStr = "0" // Default offset
		}
		if limitStr == "" {
			limitStr = "20" // Default limit
		}

		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset value"})
			return
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit value"})
			return
		}

		results, err := wikiInstance.Search(query, offset, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to perform search"})
			return
		}

		c.JSON(http.StatusOK, results)
	}
}
