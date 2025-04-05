package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func DeleteUserHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := wikiInstance.DeleteUser(id); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}
