package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func RefreshTokenUserHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Token string `json:"token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token payload"})
			return
		}

		newToken, err := wikiInstance.RefreshToken(req.Token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		c.JSON(http.StatusOK, newToken)
	}
}
