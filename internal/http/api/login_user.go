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

type LoginUserResponse struct {
	Message string           `json:"message"`
	User    *auth.PublicUser `json:"user"`
}

func LoginUserHandler(wikiInstance *wiki.Wiki, authCookies *auth_middleware.AuthCookies, csrfCookie *security.CSRFCookie) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Identifier string `json:"identifier" binding:"required"` // can be username or email
			Password   string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login payload"})
			return
		}

		data, err := wikiInstance.Login(req.Identifier, req.Password)

		if errors.Is(err, wiki.ErrAuthDisabled) {
			c.JSON(http.StatusNotFound, gin.H{"error": "authentication is disabled"})
			return
		}

		if err != nil && err == auth.ErrUserInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
			return
		}

		if _, err := csrfCookie.Issue(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to issue CSRF cookie"})
			return
		}

		if err := authCookies.Set(c, data.Token, data.RefreshToken); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to set authentication cookies"})
			return
		}

		c.JSON(http.StatusOK, LoginUserResponse{Message: "Login successful", User: data.User})
	}
}
