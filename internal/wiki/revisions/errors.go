package revisions

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeRevisionNotFound                    = "revision_not_found"
	ErrCodeRevisionInvalidPageID               = "revision_invalid_page_id"
	ErrCodeRevisionInvalidRevisionID           = "revision_invalid_revision_id"
	ErrCodeRevisionInvalidLimit                = "revision_invalid_limit"
	ErrCodeRevisionCompareInvalidRequest       = "revision_compare_invalid_request"
	ErrCodeRevisionRestoreInvalidPageID        = "revision_restore_invalid_page_id"
	ErrCodeRevisionRestoreInvalidRevision      = "revision_restore_invalid_revision"
	ErrCodeRevisionRestoreRevisionNotFound     = "revision_restore_revision_not_found"
	ErrCodeRevisionRestorePageNotFound         = "revision_restore_page_not_found"
	ErrCodeRevisionRestoreFailed               = "revision_restore_failed"
	ErrCodeRevisionRestoreContentMissing       = "revision_restore_content_missing"
	ErrCodeRevisionRestoreAssetsMissing        = "revision_restore_assets_missing"
	ErrCodeRevisionServiceUnavailable          = "revision_service_unavailable"
	ErrCodeRevisionPreviewContentUnavailable   = "revision_preview_content_unavailable"
	ErrCodeRevisionPreviewAssetsUnavailable    = "revision_preview_assets_unavailable"
	ErrCodeRevisionPreviewAssetNotFound        = "revision_preview_asset_not_found"
	ErrCodeRevisionPreviewAssetInvalidName     = "revision_preview_asset_invalid_name"
	ErrCodeRevisionPreviewAssetBlobUnavailable = "revision_preview_asset_blob_unavailable"
	ErrCodeRevisionInternalError               = "revision_internal_error"
)

// RevisionErrorResponse is the structured JSON error body returned by revision endpoints.
type RevisionErrorResponse struct {
	Error RevisionErrorDetail `json:"error"`
}

// RevisionErrorDetail carries the localization-ready error data.
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

// MapRevisionError maps an error a revisions use case passes through onto
// the LocalizedError the revision routes would respond with: a store
// os.ErrNotExist gets revision_not_found, and anything else falls into the
// internal-error arm with the route's fixed client message (the raw detail
// stays in the cause, off the wire). It is the non-HTTP core of
// respondWithRevisionError: consumers outside the HTTP layer map errors onto
// the same table instead of keeping a copy (ADR-0018).
func MapRevisionError(err error) *sharederrors.LocalizedError {
	if errors.Is(err, os.ErrNotExist) {
		return sharederrors.NewLocalizedError(ErrCodeRevisionNotFound, "Revision resource not found", "revision resource not found", err)
	}
	return sharederrors.NewLocalizedError(ErrCodeRevisionInternalError, "Revision request failed", "revision request failed", err)
}

// respondWithRevisionError is the central error handler for revision
// endpoints. It checks for LocalizedError first (rich, template-ready), then
// falls back to MapRevisionError's table so that lower-level service errors
// produce correct HTTP statuses.
func respondWithRevisionError(c *gin.Context, err error) {
	if localized, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithRevisionStatusError(c, revisionErrorStatus(localized.Code), localized.Code, localized.Message, localized.Template, localized.Args...)
		return
	}

	loc := MapRevisionError(err)
	respondWithRevisionStatusError(c, revisionErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
}

func revisionErrorStatus(code string) int {
	switch code {
	case ErrCodeRevisionNotFound, ErrCodeRevisionRestoreRevisionNotFound, ErrCodeRevisionRestorePageNotFound, ErrCodeRevisionPreviewAssetNotFound:
		return http.StatusNotFound
	case ErrCodeRevisionRestoreInvalidPageID, ErrCodeRevisionRestoreInvalidRevision, ErrCodeRevisionPreviewAssetInvalidName:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
