package links

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	ErrCodeLinkPageNotFound  = "link_page_not_found"
	ErrCodeLinkUnavailable   = "link_service_unavailable"
	ErrCodeLinkInternalError = "link_internal_error"
)

// respondWithLinkError maps errors to JSON responses for link endpoints.
func respondWithLinkError(c *gin.Context, err error) {
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
