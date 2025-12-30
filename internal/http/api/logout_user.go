package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/wiki"
)

type LogoutUserResponse struct {
	Message string `json:"message"`
}

func LogoutUserHandler(w *wiki.Wiki, authCookies *auth_middleware.AuthCookies, csrfCookie *security.CSRFCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		refreshToken, _ := authCookies.ReadRefresh(c)
		if refreshToken != "" {
			// revoke the refresh token session
			err := w.GetAuthService().RevokeRefreshToken(refreshToken)
			if err != nil {
				log.Printf("[INFO] Unable to revoke the refresh token: %v", err)
			}
		}

		// clear cookies!
		if err := authCookies.Clear(c); err != nil {
			log.Printf("[INFO] Unable to clear auth cookies: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := csrfCookie.Clear(c); err != nil {
			log.Printf("[INFO] Unable to clear CSRF cookie: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to clear CSRF cookie"})
			return
		}

		c.JSON(http.StatusOK, LogoutUserResponse{
			Message: "Logout successful",
		})
	}
}
