package search

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeSearchUnavailable = "search_unavailable"
	ErrCodeSearchInternal    = "search_internal_error"
)

// SearchErrorResponse is the structured JSON error body returned by search endpoints.
type SearchErrorResponse struct {
	Error SearchErrorDetail `json:"error"`
}

// SearchErrorDetail carries the localization-ready error data.
type SearchErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithSearchStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, SearchErrorResponse{
		Error: SearchErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithSearchError maps errors to JSON responses for search endpoints.
func respondWithSearchError(c *gin.Context, err error) {
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
	default:
		return http.StatusInternalServerError
	}
}
