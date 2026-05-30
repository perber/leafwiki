package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

var (
	ErrInvalidUserContext      = errors.New("invalid user context")
	ErrAuthDisabledMissingUser = errors.New("user not authenticated and auth is disabled")
	ErrMissingAccessToken      = errors.New("missing or invalid access token")
	ErrAuthServiceUnavailable  = errors.New("authentication service unavailable")
	ErrInvalidOrExpiredToken   = errors.New("invalid or expired token")
)

func ResolveRequestUser(c *gin.Context, authService *coreauth.AuthService, authCookies *AuthCookies, authDisabled bool) (*coreauth.User, error) {
	if userValue, exists := c.Get("user"); exists {
		user, ok := userValue.(*coreauth.User)
		if !ok || user == nil {
			return nil, ErrInvalidUserContext
		}
		return user, nil
	}

	if authDisabled {
		return nil, ErrAuthDisabledMissingUser
	}

	token, err := authCookies.ReadAccess(c)
	if err != nil || token == "" {
		return nil, ErrMissingAccessToken
	}

	if authService == nil {
		return nil, ErrAuthServiceUnavailable
	}

	user, err := authService.ValidateToken(token)
	if err != nil {
		return nil, ErrInvalidOrExpiredToken
	}

	c.Set("user", user)
	return user, nil
}

func RequireAuth(authService *coreauth.AuthService, authCookies *AuthCookies, authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := ResolveRequestUser(c, authService, authCookies, authDisabled)
		if err == nil {
			c.Next()
			return
		}

		switch {
		case errors.Is(err, ErrInvalidUserContext):
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
		case errors.Is(err, ErrAuthDisabledMissingUser):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated and auth is disabled"})
		case errors.Is(err, ErrMissingAccessToken):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid access token"})
		case errors.Is(err, ErrAuthServiceUnavailable):
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authentication service unavailable"})
		case errors.Is(err, ErrInvalidOrExpiredToken):
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
		}
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

		user, ok := userValue.(*coreauth.User)
		if !ok || !user.HasRole(coreauth.RoleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}

func RequireSelfOrAdmin(authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Block all user management operations when authentication is disabled
		if authDisabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User management is not available when authentication is disabled"})
			return
		}

		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*coreauth.User)
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

		// Check if user has admin privileges for accessing other users
		if !user.HasRole(coreauth.RoleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}

// OptionalAuth validates the session cookie if present and stores the user in context,
// but unlike RequireAuth it does not abort the request for unauthenticated callers.
// Exception: a token IS present but authService is nil — that is a misconfiguration
// and aborts with 500, matching RequireAuth's behaviour for the same case.
func OptionalAuth(authService *coreauth.AuthService, authCookies *AuthCookies) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get("user"); exists {
			c.Next()
			return
		}
		token, err := authCookies.ReadAccess(c)
		if err != nil || token == "" {
			c.Next()
			return
		}
		if authService == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authentication service unavailable"})
			return
		}
		if user, err := authService.ValidateToken(token); err == nil {
			c.Set("user", user)
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

		user, ok := userValue.(*coreauth.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		if user.HasRole(coreauth.RoleAdmin) || user.HasRole(coreauth.RoleEditor) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Editor or Admin role required"})
	}
}

func RequireSelf() gin.HandlerFunc {
	return func(c *gin.Context) {
		userValue, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not authenticated"})
			return
		}

		user, ok := userValue.(*coreauth.User)
		if !ok || user.ID != c.Param("id") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You can only access your own account"})
			return
		}

		c.Next()
	}
}
