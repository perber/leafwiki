package assets

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/shared"
	"github.com/perber/wiki/internal/core/tree"
)

const (
	ErrCodeAssetFileTooLarge   = "asset_file_too_large"
	ErrCodeAssetMissingFile    = "asset_missing_file"
	ErrCodeAssetMissingName    = "asset_missing_name"
	ErrCodeAssetPageNotFound   = "asset_page_not_found"
	ErrCodeAssetInvalidPayload = "asset_invalid_payload"
	ErrCodeAssetInternalError  = "asset_internal_error"
)

// respondWithAssetError maps errors to JSON responses for asset endpoints.
func respondWithAssetError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, shared.ErrFileTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
	case errors.Is(err, tree.ErrPageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "page not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
