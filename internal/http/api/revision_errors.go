package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

type RevisionErrorResponse struct {
	Error RevisionErrorDetail `json:"error"`
}

type RevisionErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithRevisionStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, RevisionErrorResponse{
		Error: RevisionErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

func respondWithRevisionError(c *gin.Context, err error) {
	if localized, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithRevisionStatusError(c, revisionErrorStatus(localized.Code), localized.Code, localized.Message, localized.Template, localized.Args...)
		return
	}

	switch {
	case errors.Is(err, os.ErrNotExist):
		respondWithRevisionStatusError(c, http.StatusNotFound, "revision_not_found", "Revision resource not found", "revision resource not found")
	default:
		respondWithRevisionStatusError(c, http.StatusInternalServerError, "revision_internal_error", "Revision request failed", "revision request failed")
	}
}

func revisionErrorStatus(code string) int {
	switch code {
	case "revision_restore_trash_not_found", "revision_restore_revision_not_found", "revision_restore_page_not_found":
		return http.StatusNotFound
	case "revision_preview_content_unavailable", "revision_preview_assets_unavailable":
		return http.StatusInternalServerError
	case "revision_preview_asset_not_found":
		return http.StatusNotFound
	case "revision_preview_asset_invalid_name":
		return http.StatusBadRequest
	case "revision_preview_asset_blob_unavailable":
		return http.StatusInternalServerError
	case "revision_restore_slug_conflict", "revision_restore_structure_conflict":
		return http.StatusConflict
	case "revision_restore_invalid_page_id", "revision_restore_invalid_revision", "revision_restore_invalid_kind", "revision_restore_parent_required":
		return http.StatusBadRequest
	case "revision_restore_parent_not_found":
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
