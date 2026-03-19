package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func PreviewPageRefactorHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req wiki.PageRefactorPreviewRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		preview, err := w.PreviewPageRefactor(id, req)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, preview)
	}
}
