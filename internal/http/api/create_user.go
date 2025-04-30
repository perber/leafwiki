package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func CreateUserHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required"`
			Password string `json:"password" binding:"required"`
			Role     string `json:"role" binding:"required,oneof=admin editor"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user payload"})
			return
		}

		user, err := wikiInstance.CreateUser(req.Username, req.Email, req.Password, req.Role)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusCreated, user)
	}
}
