package security

import (
	"crypto/subtle"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CSRFMiddleware is a Gin middleware that protects against CSRF attacks.
func CSRFMiddleware(csrf *CSRFCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// Only protect mutating methods (POST, PUT, PATCH, DELETE)
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		cookieToken, err := csrf.Read(c)
		if err != nil || cookieToken == "" {
			log.Printf("CSRF token missing or error reading token: %v", err)
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
			log.Printf("CSRF token invalid or does not match cookie")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Invalid CSRF token",
			})
			return
		}

		c.Next()
	}
}
