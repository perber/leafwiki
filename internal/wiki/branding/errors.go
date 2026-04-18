package branding

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// respondWithBrandingError maps errors to JSON responses for branding endpoints.
func respondWithBrandingError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
