package security

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuthContextKey is the Gin context key InjectAPIKeyUser sets to true
// once a Bearer API key has been resolved to a user. It lives here, not in the
// auth middleware package, because auth imports security.
const APIKeyAuthContextKey = "apiKeyAuth"

// CSRFMiddleware is a Gin middleware that protects against CSRF attacks.
func CSRFMiddleware(csrf *CSRFCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// Only protect mutating methods (POST, PUT, PATCH, DELETE)
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		// A request authenticated with a resolved API key carries no browser
		// credentials: browsers never attach an Authorization header on their
		// own, so it cannot be forged cross-site. This keys off the flag set
		// by InjectAPIKeyUser after a successful key lookup, never off the
		// mere presence of the header.
		if resolved, _ := c.Get(APIKeyAuthContextKey); resolved == true {
			c.Next()
			return
		}

		cookieToken, err := csrf.Read(c)
		if err != nil || cookieToken == "" {
			slog.Default().Warn("CSRF token missing or error reading token", "error", err)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing",
			})
			return
		}

		// Expect token in header X-CSRF-Token, alternatively in form field csrf_token
		headerToken := c.GetHeader("X-CSRF-Token")
		if headerToken == "" {
			headerToken = c.PostForm("csrf_token")
		}

		// No token in header/form or no match
		if headerToken == "" || subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) != 1 {
			slog.Default().Warn("CSRF token invalid or does not match cookie")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF token",
			})
			return
		}

		c.Next()
	}
}
