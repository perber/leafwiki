package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

// MustGetUser returns the authenticated user from context or aborts the request.
func MustGetUser(c *gin.Context) *auth.User {
	v, exists := c.Get("user")
	if !exists {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
		return nil
	}

	user, ok := v.(*auth.User)
	if !ok || user == nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
		return nil
	}

	return user
}
