package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func LoginUserHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Identifier string `json:"identifier" binding:"required"` // can be username or email
			Password   string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid login payload"})
			return
		}

		token, err := wikiInstance.Login(req.Identifier, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		c.JSON(http.StatusOK, token)
	}
}
