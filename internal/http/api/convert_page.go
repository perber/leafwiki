package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

type convertPageRequest struct {
	TargetKind string `json:"targetKind" binding:"required"`
}

func ConvertPageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req convertPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing page ID"})
			return
		}

		err := w.ConvertPage(user.ID, id, tree.NodeKind(req.TargetKind))
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "page converted"})
	}
}
