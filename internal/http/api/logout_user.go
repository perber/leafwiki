package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware"
	"github.com/perber/wiki/internal/wiki"
)

type LogoutUserResponse struct {
	Message string `json:"message"`
}

func LogoutUserHandler(w *wiki.Wiki, authCookies *middleware.AuthCookies) gin.HandlerFunc {
	return func(c *gin.Context) {
		refreshToken, _ := authCookies.ReadRefresh(c)
		if refreshToken != "" {
			// revoke the refresh token session
			_ = w.GetAuthService().RevokeRefreshToken(refreshToken)
		}

		// clear cookies!
		if err := authCookies.Clear(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, LogoutUserResponse{
			Message: "Logout successful",
		})
	}
}
