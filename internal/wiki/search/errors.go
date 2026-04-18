package search

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// respondWithSearchError maps errors to JSON responses for search endpoints.
func respondWithSearchError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
