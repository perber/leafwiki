package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

type createPageRequest struct {
	ParentID *string `json:"parentId"` // optional
	Title    string  `json:"title" binding:"required"`
}

func CreatePageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		page, err := w.CreatePage(req.ParentID, req.Title)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, page)
	}
}
