package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

type EnsurePageRequest struct {
	Path        string `json:"path" binding:"required"`
	TargetTitle string `json:"targetTitle" binding:"required"`
}

func EnsurePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req EnsurePageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		kind := tree.NodeKindPage
		result, err := w.EnsurePath(user.ID, req.Path, req.TargetTitle, &kind)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(result, w.GetUserResolver()))
	}
}
