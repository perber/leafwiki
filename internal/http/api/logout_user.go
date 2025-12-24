package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware"
)

type LogoutUserResponse struct {
	Message string `json:"message"`
}

func LogoutUserHandler(authCookies *middleware.AuthCookies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cookies löschen (Access + Refresh)
		if err := authCookies.Clear(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, LogoutUserResponse{
			Message: "Logout successful",
		})
	}
}
