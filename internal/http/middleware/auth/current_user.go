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

// TryGetUser returns the authenticated user from context, or nil if not set.
func TryGetUser(c *gin.Context) *auth.User {
	v, exists := c.Get("user")
	if !exists {
		return nil
	}
	user, ok := v.(*auth.User)
	if !ok {
		return nil
	}
	return user
}

// IsAPIKeyAuth reports whether the current request's user was resolved from
// a Bearer API key (set by InjectAPIKeyUser) rather than a normal cookie/JWT
// session or reverse-proxy header.
func IsAPIKeyAuth(c *gin.Context) bool {
	v, _ := c.Get("apiKeyAuth")
	b, _ := v.(bool)
	return b
}
