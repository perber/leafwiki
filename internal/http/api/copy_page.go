package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

type copyPageRequest struct {
	TargetParentID *string `json:"targetParentId"`
	Title          string  `json:"title" binding:"required"`
	Slug           string  `json:"slug" binding:"required"`
}

func CopyPageHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req copyPageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var normalizedTargetID *string = nil
		if req.TargetParentID != nil && (*req.TargetParentID == "" || *req.TargetParentID == "root") {
			normalizedTargetID = req.TargetParentID
		}

		currentPageID := c.Param("id")
		page, err := w.CopyPage(currentPageID, normalizedTargetID, req.Title, req.Slug)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusCreated, ToAPIPage(page))
	}
}
