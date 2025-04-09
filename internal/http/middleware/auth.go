package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/wiki"
)

func RequireAuth(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		user, err := wikiInstance.GetAuthService().ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		// Store the user in context for later use
		c.Set("user", user)
		c.Next()
	}
}

func RequireAdmin(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok || !user.HasRole(auth.RoleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}

func RequireSelfOrAdmin(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok || (!user.HasRole(auth.RoleAdmin) && user.ID != c.Param("id")) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}

func RequireSelf(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok || user.ID != c.Param("id") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You can only access your own account"})
			return
		}

		c.Next()
	}
}
