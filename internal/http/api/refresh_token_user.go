package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
	"github.com/perber/wiki/internal/wiki"
)

type RefreshUserResponse struct {
	Message string           `json:"message"`
	User    *auth.PublicUser `json:"user"`
}

func RefreshTokenUserHandler(wikiInstance *wiki.Wiki, authCookies *auth_middleware.AuthCookies, csrfCookie *security.CSRFCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		rt, err := authCookies.ReadRefresh(c)
		if err != nil || rt == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid refresh token"})
			return
		}

		data, err := wikiInstance.RefreshToken(rt)
		if err != nil {
			if errors.Is(err, wiki.ErrAuthDisabled) {
				c.JSON(http.StatusNotFound, gin.H{"error": "authentication is disabled"})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		if _, err := csrfCookie.Issue(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to issue CSRF cookie"})
			return
		}

		if err := authCookies.Set(c, data.Token, data.RefreshToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set authentication cookies"})
			return
		}

		c.JSON(http.StatusOK, RefreshUserResponse{Message: "Token refreshed", User: data.User})
	}
}
