package search

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/http/dto"
)

const (
	ErrCodeSearchUnavailable   = "search_unavailable"
	ErrCodeSearchInternal      = "search_internal_error"
	ErrCodeSearchMissingQuery  = "search_missing_query"
	ErrCodeSearchInvalidOffset = "search_invalid_offset"
	ErrCodeSearchInvalidLimit  = "search_invalid_limit"
	ErrCodeSearchQueryTooLong  = "search_query_too_long"
)

// statusClientClosedRequest answers a search whose client went away before
// it finished (nginx's 499). Nobody reads the response; the status only
// keeps the access log from showing a disconnect as a 200 or a server error.
const statusClientClosedRequest = 499

func respondWithSearchStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithSearchError maps errors to JSON responses for search endpoints.
func respondWithSearchError(c *gin.Context, err error) {
	if errors.Is(err, context.Canceled) {
		c.AbortWithStatus(statusClientClosedRequest)
		return
	}
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithSearchStatusError(c, searchErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}

	respondWithSearchStatusError(c, http.StatusInternalServerError, ErrCodeSearchInternal, "Failed to perform search", "failed to perform search")
}

func searchErrorStatus(code string) int {
	switch code {
	case ErrCodeSearchUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeSearchMissingQuery, ErrCodeSearchInvalidOffset, ErrCodeSearchInvalidLimit, ErrCodeSearchQueryTooLong:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
