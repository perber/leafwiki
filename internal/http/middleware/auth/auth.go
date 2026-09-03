package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

const errUserNotAuthenticated = "User not authenticated"

// sessionOutcome is the result of resolving the caller's identity for a
// request: either a user, or the HTTP status + body a strict gate would abort
// with. resolveSession never writes to the response, so callers can decide
// whether a failure is fatal (RequireAuth) or may fall through to anonymous
// access (RequireAuthOrPublicRead).
type sessionOutcome struct {
	user   *auth.User
	status int   // 0 when user != nil
	body   gin.H // nil when user != nil
}

func abortStatus(status int, message string) sessionOutcome {
	return sessionOutcome{status: status, body: gin.H{"error": message}}
}

// resolveSession applies the standard identity resolution: a user already put
// in context by a trusted upstream (proxy header / API key) wins; otherwise
// the access-token cookie is validated. On success the user is stored in the
// context. It is the single implementation shared by RequireAuth and
// RequireAuthOrPublicRead.
func resolveSession(c *gin.Context, authService *auth.AuthService, authCookies *AuthCookies, authDisabled bool) sessionOutcome {
	if userValue, exists := c.Get("user"); exists {
		user, ok := userValue.(*auth.User)
		if !ok || user == nil {
			return abortStatus(http.StatusInternalServerError, "Invalid user context")
		}
		return sessionOutcome{user: user}
	}

	if authDisabled {
		return abortStatus(http.StatusUnauthorized, "User not authenticated and auth is disabled")
	}

	token, err := authCookies.ReadAccess(c)
	if err != nil || token == "" {
		return abortStatus(http.StatusUnauthorized, "Missing or invalid access token")
	}

	if authService == nil {
		return abortStatus(http.StatusInternalServerError, "Authentication service unavailable")
	}

	user, err := authService.ValidateToken(token)
	if err != nil {
		return abortStatus(http.StatusUnauthorized, "Invalid or expired token")
	}

	c.Set("user", user)
	return sessionOutcome{user: user}
}

func RequireAuth(authService *auth.AuthService, authCookies *AuthCookies, authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r := resolveSession(c, authService, authCookies, authDisabled); r.user == nil {
			c.AbortWithStatusJSON(r.status, r.body)
			return
		}
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errUserNotAuthenticated})
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

// RequireCookieSession blocks a request whose "user" was resolved from a
// Bearer API key rather than a normal cookie/JWT session. Used for endpoints
// that must remain UI/session-only, like API key management itself — an API
// key must not be usable to create, list, or revoke other API keys.
func RequireCookieSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAPIKeyAuth(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this endpoint requires a browser session, not an API key"})
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errUserNotAuthenticated})
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

		// Check if user has admin privileges for accessing other users
		if !user.HasRole(auth.RoleAdmin) {
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
func OptionalAuth(authService *auth.AuthService, authCookies *AuthCookies) gin.HandlerFunc {
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errUserNotAuthenticated})
			return
		}

		user, ok := userValue.(*auth.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errUserNotAuthenticated})
			return
		}

		if user.HasRole(auth.RoleAdmin) || user.HasRole(auth.RoleEditor) {
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": errUserNotAuthenticated})
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
