package pages

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/favorites"
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

// newPageMissingIDError and newPageMissingPathError keep the message and
// template the handlers returned for a missing ID or path.
func newPageMissingIDError() *sharederrors.LocalizedError {
	return sharederrors.NewLocalizedError(ErrCodePageMissingID, "Page ID is required", "page id is required", nil)
}

func newPageMissingPathError() *sharederrors.LocalizedError {
	return sharederrors.NewLocalizedError(ErrCodePageMissingPath, "Missing path", "missing path", nil)
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

// MapPageError maps an error a page use case passes through onto the
// LocalizedError the page routes would respond with for it: each tree sentinel
// the routes handle gets its domain code, message and template, and anything
// else falls into the internal-error arm (the same fallback the REST endpoints
// use, with the raw detail kept as the client-facing message). A drift error
// is the exception: its Error() text embeds the absolute storage path, so it
// gets a fixed message and keeps the DriftError as the cause — the path stays
// in the log and the cause, never in the client response. It is the non-HTTP
// core of respondWithPageError: consumers outside the HTTP layer map errors
// onto the same table instead of keeping a copy (ADR-0019).
func MapPageError(err error) *sharederrors.LocalizedError {
	switch {
	case errors.Is(err, tree.ErrPageNotFound):
		return sharederrors.NewLocalizedError(ErrCodePageNotFound, "Page not found", "page not found", err)
	case errors.Is(err, tree.ErrParentNotFound):
		return sharederrors.NewLocalizedError(ErrCodePageParentNotFound, "Parent page not found", "parent page not found", err)
	case errors.Is(err, tree.ErrPageHasChildren):
		return sharederrors.NewLocalizedError(ErrCodePageHasChildren, "Page has children, use recursive delete", "page has children", err)
	case errors.Is(err, tree.ErrPageAlreadyExists):
		return sharederrors.NewLocalizedError(ErrCodePageSlugConflict, "Page already exists", "page already exists", err)
	case errors.Is(err, tree.ErrMovePageCircularReference):
		return sharederrors.NewLocalizedError(ErrCodePageCircularMove, "Move would create a circular reference", "circular reference detected", err)
	case errors.Is(err, tree.ErrPageCannotBeMovedToItself):
		return sharederrors.NewLocalizedError(ErrCodePageCannotMoveToSelf, "Page cannot be moved to itself", "page cannot be moved to itself", err)
	case errors.Is(err, tree.ErrConvertNotAllowed):
		return sharederrors.NewLocalizedError(ErrCodePageConvertNotAllowed, "Convert operation not allowed", "convert not allowed", err)
	case errors.Is(err, tree.ErrInvalidOperation):
		return sharederrors.NewLocalizedError(ErrCodePageInvalidRequest, err.Error(), "invalid operation", err)
	case errors.Is(err, tree.ErrVersionConflict):
		return sharederrors.NewLocalizedError(ErrCodePageVersionConflict, "Page was changed by another request", "page was changed by another request", err)
	case errors.Is(err, tree.ErrVersionRequired):
		return sharederrors.NewLocalizedError(ErrCodePageVersionRequired, "Page version is required", "page version is required", err)
	case errors.Is(err, tree.ErrTreeNotLoaded):
		return sharederrors.NewLocalizedError(ErrCodePageInternalError, "Tree not loaded", "tree not loaded", err)
	case errors.Is(err, tree.ErrDrift):
		// Fixed message: DriftError.Error() embeds the absolute storage
		// path, which must stay server-side (logs and cause only).
		return sharederrors.NewLocalizedError(ErrCodePageInternalError, "Storage drift detected", "storage drift detected", err)
	default:
		return sharederrors.NewLocalizedError(ErrCodePageInternalError, err.Error(), "internal error", err)
	}
}

// respondWithPageError is the central error handler for all page endpoints.
// It checks for LocalizedError first (rich, template-ready), then
// ValidationErrors, and falls back to MapPageError's sentinel table so that
// lower-level service errors produce correct HTTP statuses.
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

	loc := MapPageError(err)
	respondWithPageStatusError(c, pageErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
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
	case favorites.ErrCodeFavoritesStoreUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
