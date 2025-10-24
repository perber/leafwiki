package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

type EnsurePageRequest struct {
	Path        string `json:"path" binding:"required"`
	TargetTitle string `json:"targetTitle" binding:"required"`
}

func EnsurePageHandler(wikiInstance *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req EnsurePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		result, err := wikiInstance.EnsurePath(req.Path, req.TargetTitle)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(result))
	}
}
