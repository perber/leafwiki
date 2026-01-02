package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/wiki"
)

func RequireAuth(wikiInstance *wiki.Wiki, authCookies *AuthCookies, authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {

		if authDisabled {
			if _, exists := c.Get("user"); exists {
				c.Next()
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated and auth is disabled"})
			}
			return
		}

		token, err := authCookies.ReadAccess(c)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid access token"})
			return
		}

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

func RequireAdmin(authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Explicitly block admin operations when authentication is disabled
		if authDisabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin operations are not available when authentication is disabled"})
			return
		}

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

func RequireSelfOrAdmin(authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Invalid user"})
			return
		}

		// Check if user is trying to access their own resource
		isSelf := user.ID == c.Param("id")

		// Allow users to access their own resources
		if isSelf {
			c.Next()
			return
		}

		// For non-self access (admin operations), block if auth is disabled
		if authDisabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin operations are not available when authentication is disabled"})
			return
		}

		// Check if user has admin privileges for accessing other users
		if !user.HasRole(auth.RoleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}

func RequireEditorOrAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok || !(user.HasRole(auth.RoleAdmin) || user.HasRole(auth.RoleEditor)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Editor or Admin role required"})
			return
		}

		c.Next()
	}
}

func RequireSelf() gin.HandlerFunc {
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
