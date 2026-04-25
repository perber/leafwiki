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
	ErrCodePageNotFound          = "page_not_found"
	ErrCodePageParentNotFound    = "page_parent_not_found"
	ErrCodePageSlugConflict      = "page_slug_conflict"
	ErrCodePageHasChildren       = "page_has_children"
	ErrCodePageCircularMove      = "page_circular_move"
	ErrCodePageCannotMoveToSelf  = "page_cannot_move_to_self"
	ErrCodePageRootOperation     = "page_root_operation"
	ErrCodePageConvertNotAllowed = "page_convert_not_allowed"
	ErrCodePageInternalError     = "page_internal_error"
	ErrCodePageMissingPath       = "page_missing_path"
	ErrCodePageMissingID         = "page_missing_id"
	ErrCodePageMissingTitle      = "page_missing_title"
	ErrCodePageVersionRequired   = "page_version_required"
	ErrCodePageVersionConflict   = "page_version_conflict"
	ErrCodePageInvalidRequest    = "page_invalid_request"
	ErrCodePageInvalidPayload    = "page_invalid_payload"
	ErrCodePageInvalidTargetKind = "page_invalid_target_kind"
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

func respondWithPageStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, PageErrorResponse{
		Error: PageErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithPageError is the central error handler for all page endpoints.
// It checks for LocalizedError first (rich, template-ready), then falls back to
// sentinel error mapping so that lower-level service errors produce correct HTTP statuses.
func respondWithPageError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithPageStatusError(c, pageErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
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

	switch {
	case errors.Is(err, tree.ErrPageNotFound):
		respondWithPageStatusError(c, http.StatusNotFound, ErrCodePageNotFound, "Page not found", "page not found")
	case errors.Is(err, tree.ErrParentNotFound):
		respondWithPageStatusError(c, http.StatusNotFound, ErrCodePageParentNotFound, "Parent page not found", "parent page not found")
	case errors.Is(err, tree.ErrPageHasChildren):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageHasChildren, "Page has children, use recursive delete", "page has children")
	case errors.Is(err, tree.ErrPageAlreadyExists):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageSlugConflict, "Page already exists", "page already exists")
	case errors.Is(err, tree.ErrMovePageCircularReference):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageCircularMove, "Move would create a circular reference", "circular reference detected")
	case errors.Is(err, tree.ErrPageCannotBeMovedToItself):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageCannotMoveToSelf, "Page cannot be moved to itself", "page cannot be moved to itself")
	case errors.Is(err, tree.ErrConvertNotAllowed):
		respondWithPageStatusError(c, http.StatusBadRequest, ErrCodePageConvertNotAllowed, "Convert operation not allowed", "convert not allowed")
	case errors.Is(err, tree.ErrTreeNotLoaded):
		respondWithPageStatusError(c, http.StatusInternalServerError, ErrCodePageInternalError, "Tree not loaded", "tree not loaded")
	default:
		respondWithPageStatusError(c, http.StatusInternalServerError, ErrCodePageInternalError, err.Error(), "internal error")
	}
}

func pageErrorStatus(code string) int {
	switch code {
	case ErrCodePageNotFound, ErrCodePageParentNotFound:
		return http.StatusNotFound
	case ErrCodePageHasChildren, ErrCodePageCircularMove, ErrCodePageCannotMoveToSelf, ErrCodePageSlugConflict,
		ErrCodePageConvertNotAllowed, ErrCodePageRootOperation, ErrCodePageVersionRequired,
		ErrCodePageMissingPath, ErrCodePageMissingID, ErrCodePageMissingTitle, ErrCodePageInvalidRequest,
		ErrCodePageInvalidPayload, ErrCodePageInvalidTargetKind:
		return http.StatusBadRequest
	case ErrCodePageVersionConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
