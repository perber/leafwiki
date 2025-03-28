package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/tree"
)

func respondWithError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tree.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Page not found"})
	case errors.Is(err, tree.ErrParentNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Parent page not found"})
	case errors.Is(err, tree.ErrPageHasChildren):
		c.JSON(http.StatusBadRequest, gin.H{"error": "Page has children, use recursive delete"})
	case errors.Is(err, tree.ErrTreeNotLoaded):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tree not loaded"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
