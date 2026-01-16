package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

type createPageRequest struct {
	ParentID *string `json:"parentId"` // optional
	Title    string  `json:"title" binding:"required"`
	Slug     string  `json:"slug" binding:"required"`
	Kind     *string `json:"kind"` // optional
}

func CreatePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		kind := tree.NodeKindPage
		if req.Kind != nil {
			kind = tree.NodeKind(*req.Kind)
		}
		page, err := w.CreatePage(user.ID, req.ParentID, req.Title, req.Slug, &kind)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusCreated, ToAPIPage(page, w.GetUserResolver()))
	}
}
