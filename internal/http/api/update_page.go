package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func UpdatePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req struct {
			Title   string  `json:"title" binding:"required"`
			Slug    string  `json:"slug" binding:"required"`
			Content *string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		kind := tree.NodeKindPage
		page, err := w.UpdatePage(user.ID, id, req.Title, req.Slug, req.Content, &kind)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}
