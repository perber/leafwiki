package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

// PublicReadGate reports whether unauthenticated read access ("public mode")
// is currently allowed. It is read on every request so the flag can be
// toggled at runtime; internal/http.PublicAccessProvider satisfies it.
type PublicReadGate interface {
	Enabled() bool
}

// RequireAuthOrPublicRead gates the read-only route groups that used to be
// registered on either a bare public group or an authenticated group
// depending on a boot-time flag. Registering them once behind this middleware
// is what lets public mode flip without a restart.
//
// It behaves exactly like RequireAuth, except that on each path where
// RequireAuth would abort with 401 it instead calls c.Next() *anonymously*
// when public mode is currently on. Consequences:
//
//   - user already in context (proxy header / API key / a prior mw) → passes,
//     identical to RequireAuth.
//   - valid session cookie → user is resolved and attached, so a logged-in
//     editor keeps write affordances even while public mode is on. This is the
//     one behaviour that differs from the pre-refactor public group, which ran
//     no auth middleware at all and therefore saw every caller as anonymous.
//   - no / invalid cookie, public mode ON  → passes anonymously.
//   - no / invalid cookie, public mode OFF → 401, byte-for-byte as RequireAuth.
//   - authService misconfigured (nil) with a token present → 500, as
//     RequireAuth, regardless of public mode.
func RequireAuthOrPublicRead(authService *auth.AuthService, authCookies *AuthCookies, authDisabled bool, public PublicReadGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userValue, exists := c.Get("user"); exists {
			user, ok := userValue.(*auth.User)
			if !ok || user == nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
				return
			}
			c.Next()
			return
		}

		publicOK := public != nil && public.Enabled()

		if authDisabled {
			// InjectPublicEditor normally sets a user before this runs when
			// auth is disabled; this branch only trips if it didn't.
			if publicOK {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated and auth is disabled"})
			return
		}

		token, err := authCookies.ReadAccess(c)
		if err != nil || token == "" {
			if publicOK {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid access token"})
			return
		}

		if authService == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Authentication service unavailable"})
			return
		}

		user, err := authService.ValidateToken(token)
		if err != nil {
			if publicOK {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
