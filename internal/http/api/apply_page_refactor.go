package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	auth_middleware "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/wiki"
)

func ApplyPageRefactorHandler(w *wiki.Wiki) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var req wiki.ApplyPageRefactorRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		user := auth_middleware.MustGetUser(c)
		if user == nil {
			return
		}

		page, err := w.ApplyPageRefactor(user.ID, id, req)
		if err != nil {
			respondWithError(c, err)
			return
		}

		c.JSON(http.StatusOK, ToAPIPage(page, w.GetUserResolver()))
	}
}
