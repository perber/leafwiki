package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func GetTreeHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		tree := w.GetTree()
		c.JSON(http.StatusOK, ToAPINode(tree, ""))
	}
}
