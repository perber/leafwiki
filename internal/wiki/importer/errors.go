package importer

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	coreimporter "github.com/perber/wiki/internal/importer"
)

// respondWithImporterError maps errors to JSON responses for importer endpoints.
func respondWithImporterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, coreimporter.ErrNoPlan):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, coreimporter.ErrImportExecutionRunning):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, coreimporter.ErrImportStateUnavailable):
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
