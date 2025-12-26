package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/http/middleware"
	"github.com/perber/wiki/internal/wiki"
)

type RefreshUserResponse struct {
	Message string           `json:"message"`
	User    *auth.PublicUser `json:"user"`
}

func RefreshTokenUserHandler(wikiInstance *wiki.Wiki, authCookies *middleware.AuthCookies) gin.HandlerFunc {
	return func(c *gin.Context) {
		rt, err := authCookies.ReadRefresh(c)
		if err != nil || rt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh token"})
			return
		}

		data, err := wikiInstance.RefreshToken(rt)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		if err := authCookies.Set(c, data.Token, data.RefreshToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set authentication cookies"})
			return
		}

		c.JSON(http.StatusOK, RefreshUserResponse{Message: "Token refreshed", User: data.User})
	}
}
