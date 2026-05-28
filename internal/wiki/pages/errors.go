package pages

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

// Error codes for the pages domain.
const (
	ErrCodePageNotFound            = "page_not_found"
	ErrCodePageParentNotFound      = "page_parent_not_found"
	ErrCodePageSlugConflict        = "page_slug_conflict"
	ErrCodePageHasChildren         = "page_has_children"
	ErrCodePageCircularMove        = "page_circular_move"
	ErrCodePageCannotMoveToSelf    = "page_cannot_move_to_self"
	ErrCodePageRootOperation       = "page_root_operation"
	ErrCodePageConvertNotAllowed   = "page_convert_not_allowed"
	ErrCodePageInternalError       = "page_internal_error"
	ErrCodePageMissingPath         = "page_missing_path"
	ErrCodePageInvalidPath         = "page_invalid_path"
	ErrCodePageMissingID           = "page_missing_id"
	ErrCodePageMissingTitle        = "page_missing_title"
	ErrCodePageInvalidTitle        = "page_invalid_title"
	ErrCodePageVersionRequired     = "page_version_required"
	ErrCodePageVersionConflict     = "page_version_conflict"
	ErrCodePageInvalidRequest      = "page_invalid_request"
	ErrCodePageInvalidPayload      = "page_invalid_payload"
	ErrCodePageInvalidKind         = "page_invalid_kind"
	ErrCodePageInvalidParentID     = "page_invalid_parent_id"
	ErrCodePageInvalidTargetKind   = "page_invalid_target_kind"
	ErrCodePageInvalidRefactorKind = "page_invalid_refactor_kind"
)

func newPageRootOperationError(operation string) *sharederrors.LocalizedError {
	return sharederrors.NewLocalizedError(
		ErrCodePageRootOperation,
		fmt.Sprintf("cannot %s root page", operation),
		"cannot %s root page",
		nil, operation,
	)
}

// PageErrorResponse is the structured JSON error body returned by page endpoints.
type PageErrorResponse struct {
	Error PageErrorDetail `json:"error"`
}

// PageErrorDetail carries the localization-ready error data.
type PageErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func newPageErrorDetail(code, message, template string, args ...string) PageErrorDetail {
	return PageErrorDetail{
		Code:     code,
		Message:  message,
		Template: template,
		Args:     append([]string(nil), args...),
	}
}

// PageErrorDetailForError maps page-domain errors to the same localization-ready
// contract used by HTTP routes. The boolean is false when the error is outside
// the page domain and callers should preserve their existing fallback behavior.
func PageErrorDetailForError(err error) (PageErrorDetail, int, bool) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		return newPageErrorDetail(loc.Code, loc.Message, loc.Template, loc.Args...), pageErrorStatus(loc.Code), true
	}

	switch {
	case errors.Is(err, tree.ErrPageNotFound):
		return newPageErrorDetail(ErrCodePageNotFound, "Page not found", "page not found"), http.StatusNotFound, true
	case errors.Is(err, tree.ErrParentNotFound):
		return newPageErrorDetail(ErrCodePageParentNotFound, "Parent page not found", "parent page not found"), http.StatusNotFound, true
	case errors.Is(err, tree.ErrPageHasChildren):
		return newPageErrorDetail(ErrCodePageHasChildren, "Page has children, use recursive delete", "page has children"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrPageAlreadyExists):
		return newPageErrorDetail(ErrCodePageSlugConflict, "Page already exists", "page already exists"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrMovePageCircularReference):
		return newPageErrorDetail(ErrCodePageCircularMove, "Move would create a circular reference", "circular reference detected"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrPageCannotBeMovedToItself):
		return newPageErrorDetail(ErrCodePageCannotMoveToSelf, "Page cannot be moved to itself", "page cannot be moved to itself"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrConvertNotAllowed):
		return newPageErrorDetail(ErrCodePageConvertNotAllowed, "Convert operation not allowed", "convert not allowed"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrVersionConflict):
		return newPageErrorDetail(ErrCodePageVersionConflict, "Page was changed by another request", "page was changed by another request"), http.StatusConflict, true
	case errors.Is(err, tree.ErrVersionRequired):
		return newPageErrorDetail(ErrCodePageVersionRequired, "Page version is required", "page version is required"), http.StatusBadRequest, true
	case errors.Is(err, tree.ErrTreeNotLoaded):
		return newPageErrorDetail(ErrCodePageInternalError, "Tree not loaded", "tree not loaded"), http.StatusInternalServerError, true
	default:
		return PageErrorDetail{}, 0, false
	}
}

func respondWithPageStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, PageErrorResponse{
		Error: newPageErrorDetail(code, message, template, args...),
	})
}

// respondWithPageError is the central error handler for all page endpoints.
// It checks for LocalizedError first (rich, template-ready), then falls back to
// sentinel error mapping so that lower-level service errors produce correct HTTP statuses.
func respondWithPageError(c *gin.Context, err error) {
	if detail, status, ok := PageErrorDetailForError(err); ok {
		c.JSON(status, PageErrorResponse{Error: detail})
		return
	}

	var vErr *sharederrors.ValidationErrors
	if errors.As(err, &vErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation_error",
			"fields": vErr.Errors,
		})
		return
	}

	respondWithPageStatusError(c, http.StatusInternalServerError, ErrCodePageInternalError, err.Error(), "internal error")
}

func pageErrorStatus(code string) int {
	switch code {
	case ErrCodePageNotFound, ErrCodePageParentNotFound:
		return http.StatusNotFound
	case ErrCodePageHasChildren, ErrCodePageCircularMove, ErrCodePageCannotMoveToSelf, ErrCodePageSlugConflict,
		ErrCodePageConvertNotAllowed, ErrCodePageRootOperation, ErrCodePageVersionRequired,
		ErrCodePageMissingPath, ErrCodePageMissingID, ErrCodePageMissingTitle, ErrCodePageInvalidTitle, ErrCodePageInvalidRequest,
		ErrCodePageInvalidPath, ErrCodePageInvalidPayload, ErrCodePageInvalidKind, ErrCodePageInvalidParentID, ErrCodePageInvalidTargetKind, ErrCodePageInvalidRefactorKind:
		return http.StatusBadRequest
	case ErrCodePageVersionConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
